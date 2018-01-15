package redis

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

// todo - figure out when to remove a channel from here (when all disconnected?) - DONE
// todo - THIS IS SINGLETON for all test servers, make per-server

// var establishedChannels []string

// Receiver receives messages from Redis and broadcasts them to all
// registered websocket connections that are Registered.
type Receiver struct {
	mu                  *sync.Mutex
	pool                *redis.Pool
	pubSubConn          *redis.PubSubConn
	close               *err.Close
	establishedChannels []string

	messages       chan *Message
	newConnections chan *Connection
	rmConnections  chan *Connection
}

// NewReceiver creates a redisReceiver that will use the provided
// rredis.Pool.
func NewReceiver(pool *redis.Pool, close *err.Close) *Receiver {
	return &Receiver{
		mu:             &sync.Mutex{},
		pool:           pool,
		pubSubConn:     nil,
		close:          close,
		messages:       make(chan *Message, 1000), // 1000 is arbitrary
		newConnections: make(chan *Connection),
		rmConnections:  make(chan *Connection),
	}
}

// Init init pubsub
func (rr *Receiver) Init() {
	conn := rr.pool.Get()
	rr.pubSubConn = &redis.PubSubConn{Conn: conn}
}

//Destroy pubsub
func (rr *Receiver) Destroy() {
	rr.pubSubConn.Close()
}

// Wait wait
func (rr *Receiver) Wait(_ time.Time) error {
	time.Sleep(time.Second * 10)
	return nil
}

// Run receives pubsub messages from Redis after establishing a connection.
// When a valid message is received it is broadcast to all connected websockets
func (rr *Receiver) Run(channel string) error {
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
			//receiver <- psc.Receive()
			receiver <- rr.pubSubConn.Receive()
		}()
		select {
		case <-exit:
			fmt.Println("exiting receiver run")
			rr.Destroy()
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
func (rr *Receiver) Broadcast(msg *Message) {
	rr.messages <- msg
}

// Register the websocket connection with the receiver.
func (rr *Receiver) Register(connection *Connection) {
	rr.newConnections <- connection
}

// DeRegister the connection by closing it and removing it from our list.
func (rr *Receiver) DeRegister(connection *Connection) {
	rr.rmConnections <- connection
}

func sendToWebConns(i int, conns *[]*Connection, msg *Message) {
	if i > len(*conns)-1 {
		return
	}
	conn := (*conns)[i]
	if conn.Channel == msg.Channel {
		if err := conn.Websocket.WriteMessage(websocket.TextMessage, msg.Data); err != nil {
			fmt.Printf("\nerr writing to socket: %s", err)
			*conns = removeConn(*conns, conn)
			sendToWebConns(i, conns, msg)
			fmt.Printf("\n no. conns: %d", len(*conns))
			return
		}
	}
	i++
	sendToWebConns(i, conns, msg)
}

func connHandler(rr *Receiver) {
	defer rr.close.Wg.Done()

	exit := make(chan bool)
	*rr.close.Exit = append(*rr.close.Exit, exit)

	conns := make([]*Connection, 0)
	for {
		select {
		case <-exit:
			fmt.Println("exiting conn handler")
			return
		case msg := <-rr.messages:
			fmt.Printf("\ngot msg: %s", msg)
			fmt.Printf("\nno of conns before: %d", len(conns))

			//fmt.Printf("conn[0] %v\n", conns[0])

			sendToWebConns(0, &conns, msg)
			fmt.Printf("\nno conns after: %d", len(conns))

			// TODO delay this?
			go func() {
				rr.mu.Lock()
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
				rr.mu.Unlock()
			}()

		case conn := <-rr.newConnections:
			conns = append(conns, conn)
		case conn := <-rr.rmConnections:
			conns = removeConn(conns, conn)
		}
	}
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
