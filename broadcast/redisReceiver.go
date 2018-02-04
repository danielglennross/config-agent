package broadcast

import (
	"fmt"
	"sync"
	"time"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
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

	messages chan *Message
	wsm      Messanger
}

// NewRedisReceiver creates a redisReceiver that will use the provided
// redis.Pool.
func NewRedisReceiver(pool *redis.Pool, close *err.Close, wsm Messanger) Receiver {
	return &RedisReceiver{
		mu:       &sync.Mutex{},
		pubSubMu: &sync.Mutex{},
		pubSubConnFactory: func() *redis.PubSubConn {
			conn := pool.Get()
			return &redis.PubSubConn{Conn: conn}
		},
		pubSubConn: nil,
		close:      close,
		messages:   make(chan *Message, 1000), // 1000 is arbitrary
		wsm:        wsm,
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

// Register the websocket connection with the receiver.
func (rr *RedisReceiver) Register(connection *Connection) {
	rr.wsm.Register(connection)
}

func (rr *RedisReceiver) broadcaster() {
	defer rr.close.Wg.Done()

	exitBroadcaster := make(chan bool)
	rr.close.Mu.Lock()
	*rr.close.Exit = append(*rr.close.Exit, exitBroadcaster)
	rr.close.Mu.Unlock()

	checkChannel := func(msg *Message) {
		rr.mu.Lock()
		defer rr.mu.Unlock()

		found := rr.wsm.ConnectionExists(msg.Channel)

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

	for {
		select {
		case <-exitBroadcaster:
			fmt.Println("disposing broadcaster")
			rr.wsm.Dispose()
			fmt.Println("disposed broadcaster")
			return
		case msg := <-rr.messages:
			rr.wsm.Send(msg)
			go checkChannel(msg)
		}
	}
}
