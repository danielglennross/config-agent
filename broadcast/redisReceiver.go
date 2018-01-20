package broadcast

import (
	"fmt"
	"sync"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type mutateConnection func(conns []*Connection, add *Connection) []*Connection

// RedisReceiver receives messages from Redis and broadcasts them to all
// registered websocket connections that are Registered.
type RedisReceiver struct {
	mu                  *sync.Mutex
	pubSubConnFactory   func() *redis.PubSubConn
	pubSubConn          *redis.PubSubConn
	close               *err.Close
	establishedChannels []string

	messages       chan *Message
	newConnections chan *Connection
}

// NewRedisReceiver creates a redisReceiver that will use the provided
// redis.Pool.
func NewRedisReceiver(pool *redis.Pool, close *err.Close) Receiver {
	return &RedisReceiver{
		mu: &sync.Mutex{},
		pubSubConnFactory: func() *redis.PubSubConn {
			conn := pool.Get()
			return &redis.PubSubConn{Conn: conn}
		},
		pubSubConn:     nil,
		close:          close,
		messages:       make(chan *Message, 1000), // 1000 is arbitrary
		newConnections: make(chan *Connection),
	}
}

// Init init pubsub
func (rr *RedisReceiver) Init() {
	rr.pubSubConn = rr.pubSubConnFactory()
}

func (rr *RedisReceiver) destroyWebsockets(conns []*Connection) {
	err := make(chan error)

	for _, c := range conns {
		go func(c *Connection) {
			err <- c.Websocket.Close()
		}(c)
	}

	for e := range err {
		fmt.Println(e)
	}
}

func (rr *RedisReceiver) destroyRedis(channel string) {
	// close connection
	rr.pubSubConn.Close()

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

// Run receives pubsub messages from Redis after establishing a connection.
// When a valid message is received it is broadcast to all connected websockets
func (rr *RedisReceiver) Run(channel string) error {
	rr.mu.Lock()

	for _, existing := range rr.establishedChannels {
		if existing == channel {
			return nil
		}
	}

	rr.establishedChannels = append(rr.establishedChannels, channel)

	rr.pubSubConn.Subscribe(channel)

	rr.mu.Unlock()

	rr.close.Wg.Add(1)
	go connHandler(rr)

	rr.close.Wg.Add(1)
	defer rr.close.Wg.Done()

	exit := make(chan bool)
	*rr.close.Exit = append(*rr.close.Exit, exit)

	for {
		fmt.Println("waiting for msg...")
		receiver := make(chan interface{})
		go func() {
			receiver <- rr.pubSubConn.Receive()
		}()
		select {
		case <-exit:
			fmt.Println("exiting receiver run")
			rr.destroyRedis(channel)
			return nil
		case v := <-receiver:
			fmt.Printf("\ngot msg type: %s - %v", v, v)
			switch v.(type) {
			case redis.Message:
				rr.Broadcast(&Message{Channel: channel, Data: v.(redis.Message).Data})
			case redis.Subscription:
				continue
			case error:
				return errors.Wrap(v.(error), "Error while subscribed to Redis channel")
			default:
			}
		}
	}
}

// Broadcast the provided message to all connected websocket connections.
// If an error occurs while writting a message to a websocket connection it is
// closed and deregistered.
func (rr *RedisReceiver) Broadcast(msg *Message) {
	rr.messages <- msg
}

// Register the websocket connection with the receiver.
func (rr *RedisReceiver) Register(connection *Connection) {
	rr.newConnections <- connection
}

func sendToWebConns(i int, conns *[]*Connection, msg *Message, connMu *sync.Mutex) {
	if i > len(*conns)-1 {
		return
	}
	conn := (*conns)[i]
	if conn.Channel == msg.Channel {
		if err := conn.Websocket.WriteMessage(websocket.TextMessage, msg.Data); err != nil {
			fmt.Printf("\nerr writing to socket: %s", err)

			// This needs cleaning, not nice passing the lock in, can we use mutate?
			connMu.Lock()
			*conns = removeConn(*conns, conn)
			connMu.Unlock()

			sendToWebConns(i, conns, msg, connMu)
			fmt.Printf("\n no. conns: %d", len(*conns))
			return
		}
	}
	i++
	sendToWebConns(i, conns, msg, connMu)
}

func connHandler(rr *RedisReceiver) {
	defer rr.close.Wg.Done()

	exit := make(chan bool)
	*rr.close.Exit = append(*rr.close.Exit, exit)

	connMu := &sync.Mutex{}
	conns := make([]*Connection, 0)

	mutateConns := func(conn *Connection, fn mutateConnection) {
		connMu.Lock()
		defer connMu.Unlock()
		conns = fn(conns, conn)
	}

	ensureRedisChannelUnsubscribe := func(msg *Message) {
		// TODO delay this?
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
				fmt.Printf("\nunsubscribing channel: %s", msg.Channel)
				rr.pubSubConn.Unsubscribe(msg.Channel)
			}
		}
	}

	for {
		select {
		case <-exit:
			fmt.Println("exiting conn handler")
			rr.destroyWebsockets(conns)
			return
		case msg := <-rr.messages:
			fmt.Printf("\ngot msg: %s", msg)
			fmt.Printf("\nno of conns before: %d", len(conns))

			sendToWebConns(0, &conns, msg, connMu)
			fmt.Printf("\nno conns after: %d", len(conns))

			go ensureRedisChannelUnsubscribe(msg)
		case conn := <-rr.newConnections:
			go mutateConns(conn, appendConn)
		}
	}
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
		fmt.Printf("\nconns: %#v\nconn: %#v", conns, remove)
		panic("Conn not found")
	}
	copy(conns[i:], conns[i+1:]) // shift down
	conns[len(conns)-1] = nil    // nil last element
	return conns[:len(conns)-1]  // truncate slice
}
