package broadcast

import (
	"fmt"
	"sync"
	"time"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
)

type mutateConnection func(conns []*Connection, add *Connection) []*Connection

// RedisReceiver receives messages from Redis and broadcasts them to all
// registered websocket connections that are Registered.
type RedisReceiver struct {
	mu *sync.Mutex

	pubSubMu          *sync.Mutex
	pubSubConnFactory func() *redis.PubSubConn
	pubSubConn        *redis.PubSubConn

	close               *err.Close
	establishedChannels []string

	messages       chan *Message
	newConnections chan *Connection

	mapp *sync.Map
}

// NewRedisReceiver creates a redisReceiver that will use the provided
// redis.Pool.
func NewRedisReceiver(pool *redis.Pool, close *err.Close) Receiver {
	return &RedisReceiver{
		mu:       &sync.Mutex{},
		pubSubMu: &sync.Mutex{},
		pubSubConnFactory: func() *redis.PubSubConn {
			conn := pool.Get()
			return &redis.PubSubConn{Conn: conn}
		},
		pubSubConn:     nil,
		close:          close,
		messages:       make(chan *Message, 1000), // 1000 is arbitrary
		newConnections: make(chan *Connection),

		mapp: &sync.Map{},
	}
}

// Init init pubsub
func (rr *RedisReceiver) Init() {
	rr.pubSubConn = rr.pubSubConnFactory()
}

func (rr *RedisReceiver) registerChannel(channel string) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	for _, existing := range rr.establishedChannels {
		if existing == channel {
			return false
		}
	}

	rr.establishedChannels = append(rr.establishedChannels, channel)
	rr.pubSubConn.Subscribe(channel)

	return true
}

// Run receives pubsub messages from Redis after establishing a connection.
// When a valid message is received it is broadcast to all connected websockets
func (rr *RedisReceiver) Run(channel string) {
	if ok := rr.registerChannel(channel); !ok {
		return
	}

	rr.close.Wg.Add(1)
	go rr.broadcaster()

	rr.close.Wg.Add(1)
	go rr.listener(channel)

	return
}

// Broadcast the provided message to all connected websocket connections.
// If an error occurs while writting a message to a websocket connection it is
// closed and deregistered.
func (rr *RedisReceiver) Broadcast(msg *Message) {
	rr.messages <- msg
}

func (rr *RedisReceiver) listener(channel string) {
	defer rr.close.Wg.Done()

	exit := make(chan bool)
	rr.close.Mu.Lock()
	*rr.close.Exit = append(*rr.close.Exit, exit)
	rr.close.Mu.Unlock()

	dispose := func() {
		// close connection
		timeout := time.After(time.Millisecond * 5000)
		closeResult := make(chan error)
		go func() {
			rr.pubSubMu.Lock()
			defer rr.pubSubMu.Unlock()
			rr.pubSubConn.Conn.Send("PING")
			rr.pubSubConn.Conn.Flush()
			closeResult <- rr.pubSubConn.Close()
		}()
		select {
		case <-timeout:
			fmt.Println("waiting on close pub sub timeout")
		case <-closeResult:
			fmt.Println("waiting on close pub sub success")
		}

		// remove channel from in memory list
		rr.mu.Lock()
		defer rr.mu.Unlock()

		var i int
		var found bool
		for i = 0; i < len(rr.establishedChannels); i++ {
			if rr.establishedChannels[i] == channel {
				found = true
				break
			}
		}
		if found {
			copy(rr.establishedChannels[i:], rr.establishedChannels[i+1:])                  // shift down
			rr.establishedChannels[len(rr.establishedChannels)-1] = ""                      // nil last element
			rr.establishedChannels = rr.establishedChannels[:len(rr.establishedChannels)-1] // truncate slice
		}
	}

	for {
		receiver := make(chan interface{})
		go func() {
			rr.pubSubMu.Lock()
			defer rr.pubSubMu.Unlock()
			receiver <- rr.pubSubConn.Receive()
		}()
		select {
		case <-exit:
			fmt.Println("disposing listener")
			dispose()
			fmt.Println("disposed listener")
			return
		case v := <-receiver:
			switch v.(type) {
			case redis.Message:
				rr.Broadcast(&Message{Channel: channel, Data: v.(redis.Message).Data})
			case redis.Subscription:
				continue
			case error:
				fmt.Printf("Error while subscribed to Redis channel %s", v.(error))
			default:
			}
		}
	}
}

// Message sync web socket sending
func (rr *RedisReceiver) Message(connection *Connection) {
	exit := make(chan bool)
	rr.close.Mu.Lock()
	*rr.close.Exit = append(*rr.close.Exit, exit)
	rr.close.Mu.Unlock()

	webSocketChan := make(chan *Connection)
	rr.mapp.Store(connection.Id, webSocketChan)
	go func() {
		for {
			select {
			case <-exit:
				fmt.Println("disposed websocket messenger")
				return
			case conn := <-webSocketChan:
				conn.WebSocketSent <- conn.Websocket.WriteMessage(websocket.TextMessage, conn.Data)
			}
		}
	}()
	webSocketChan <- connection
}

// Register the websocket connection with the receiver.
func (rr *RedisReceiver) Register(connection *Connection) {
	rr.newConnections <- connection
}

func (rr *RedisReceiver) broadcaster() {
	defer rr.close.Wg.Done()

	exitBroadcaster := make(chan bool)
	rr.close.Mu.Lock()
	*rr.close.Exit = append(*rr.close.Exit, exitBroadcaster)
	rr.close.Mu.Unlock()

	connMu := &sync.Mutex{}
	conns := make([]*Connection, 0)

	mutateConns := func(conn *Connection, fn mutateConnection) {
		connMu.Lock()
		defer connMu.Unlock()
		conns = fn(conns, conn)
	}

	send := func(msg *Message) {
		var iter func(i int)
		iter = func(i int) {
			connMu.Lock()
			if i > len(conns)-1 {
				connMu.Unlock()
				return
			}
			connMu.Unlock()

			conn := conns[i]
			if conn.Channel == msg.Channel {
				result, ok := rr.mapp.Load(conn.Id)
				if ok {
					chanConnection := result.(chan *Connection)
					chanConnection <- &Connection{ // copy connection w/ new data from redis
						Id:            conn.Id,
						Data:          msg.Data,
						Channel:       conn.Channel,
						Websocket:     conn.Websocket,
						WebSocketSent: conn.WebSocketSent,
					}
					if err := <-conn.WebSocketSent; err != nil {
						mutateConns(conn, removeConn)
						rr.mapp.Delete(conn.Id)
						iter(i)
						return
					}
				}
			}
			i++
			iter(i)
		}
		iter(0)
	}

	checkChannel := func(msg *Message) {
		rr.mu.Lock()
		defer rr.mu.Unlock()

		connMu.Lock()
		defer connMu.Unlock()

		found := false
		for _, conn := range conns {
			if conn.Channel == msg.Channel {
				found = true
			}
		}

		if !found {
			var i int
			var exists = false
			for i = 0; i < len(rr.establishedChannels); i++ {
				if rr.establishedChannels[i] == msg.Channel {
					exists = true
					break
				}
			}
			if exists {
				rr.establishedChannels = append(rr.establishedChannels[:i], rr.establishedChannels[i+1:]...)
				rr.pubSubConn.Unsubscribe(msg.Channel)
			}
		}
	}

	dispose := func() {
		connMu.Lock()
		defer connMu.Unlock()

		if len(conns) > 0 {
			deleteReq := make(chan deleteWs)

			go func() {
				for _, c := range conns {
					deleteReq <- deleteWs{id: c.Id, err: c.Websocket.Close()}
				}
				close(deleteReq)
			}()

			for dr := range deleteReq {
				if dr.err != nil {
					fmt.Printf("\n closing web socket error: %s", dr.err)
				} else {
					rr.mapp.Delete(dr.id)
				}
			}
		}
	}

	for {
		select {
		case <-exitBroadcaster:
			fmt.Println("disposing broadcaster")
			dispose()
			fmt.Println("disposed broadcaster")
			return
		case msg := <-rr.messages:
			send(msg)
			go checkChannel(msg)
		case conn := <-rr.newConnections:
			go mutateConns(conn, appendConn)
		}
	}
}

type deleteWs struct {
	id  string
	err error
}

func appendConn(conns []*Connection, add *Connection) []*Connection {
	conns = append(conns, add)
	return conns
}

func removeConn(conns []*Connection, remove *Connection) []*Connection {
	var i int
	var found bool
	for i = 0; i < len(conns); i++ {
		if conns[i] == remove {
			found = true
			break
		}
	}

	if !found {
		// log
		return conns
	}

	copy(conns[i:], conns[i+1:]) // shift down
	conns[len(conns)-1] = nil    // nil last element
	return conns[:len(conns)-1]  // truncate slice
}
