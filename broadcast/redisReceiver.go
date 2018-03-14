package broadcast

import (
	"sync"
	"time"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/logger"
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

	messages chan *TrackedMessage
	wsm      Messenger

	log *logger.Logger
}

// NewRedisReceiver creates a redisReceiver that will use the provided
// redis.Pool.
func NewRedisReceiver(pool *redis.Pool, close *err.Close, wsm Messenger, log *logger.Logger) Receiver {
	return &RedisReceiver{
		mu:       &sync.Mutex{},
		pubSubMu: &sync.Mutex{},
		pubSubConnFactory: func() *redis.PubSubConn {
			conn := pool.Get()
			return &redis.PubSubConn{Conn: conn}
		},
		pubSubConn: nil,
		close:      close,
		messages:   make(chan *TrackedMessage, 1000), // 1000 is arbitrary
		wsm:        wsm,
		log:        log,
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
func (rr *RedisReceiver) Broadcast(msg *TrackedMessage) {
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
			rr.log.ErrorMsg("waiting on close pub sub timeout")
		case <-closeResult:
			rr.log.InfoMsg("waiting on close pub sub success")
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
			rr.log.InfoMsg("disposing listener")
			dispose()
			rr.log.InfoMsg("disposed listener")
			return
		case v := <-receiver:
			switch v.(type) {
			case redis.Message:
				msg, err := GetReceiverMessage(v.(redis.Message).Data, channel)
				if err != nil {
					rr.log.Error("Failed to read redis message", err)
					continue
				}

				rr.Broadcast(msg)
			case redis.Subscription:
				continue
			case error:
				rr.log.Error("Error while subscribed to Redis channel", v.(error))
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

	checkChannel := func(msg *TrackedMessage) {
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
			rr.log.InfoMsg("disposing broadcaster")
			rr.wsm.Dispose()
			rr.log.InfoMsg("disposed broadcaster")
			return
		case msg := <-rr.messages:
			rr.wsm.Send(msg)
			go checkChannel(msg)
		}
	}
}
