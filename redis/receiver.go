package redis

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

// todo - figure out when to remove a channel from here (when all disconnected?)
var establishedChannels []string

// Receiver receives messages from Redis and broadcasts them to all
// registered websocket connections that are Registered.
type Receiver struct {
	pool *redis.Pool

	messages       chan *Message
	newConnections chan *Connection
	rmConnections  chan *Connection
}

// NewReceiver creates a redisReceiver that will use the provided
// rredis.Pool.
func NewReceiver(pool *redis.Pool) *Receiver {
	return &Receiver{
		pool:           pool,
		messages:       make(chan *Message, 1000), // 1000 is arbitrary
		newConnections: make(chan *Connection),
		rmConnections:  make(chan *Connection),
	}
}

// Wait wait
func (rr *Receiver) Wait(_ time.Time) error {
	time.Sleep(time.Second * 10)
	return nil
}

// Run receives pubsub messages from Redis after establishing a connection.
// When a valid message is received it is broadcast to all connected websockets
func (rr *Receiver) Run(channel string) error {
	for _, existing := range establishedChannels {
		if existing == channel {
			return nil
		}
	}

	establishedChannels = append(establishedChannels, channel)

	conn := rr.pool.Get()
	defer conn.Close()
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(channel)
	go rr.connHandler()
	for {
		v := psc.Receive()
		fmt.Printf("got msg type: %s - %v", v, v)
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

func (rr *Receiver) connHandler() {
	conns := make([]*Connection, 0)
	for {
		select {
		case msg := <-rr.messages:
			fmt.Printf("got msg: %s\n", msg)
			fmt.Printf("no of connect: %d", len(conns))

			for _, conn := range conns {
				if conn.Channel == msg.Channel {
					// send to all connected clients
					s := string(msg.Data)

					if err := conn.Websocket.WriteJSON(s); err != nil {
						fmt.Printf("err writing to socket: %s", err)
						conns = removeConn(conns, conn)
					}
					fmt.Println("wrote to socket")
				}
			}
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
		fmt.Printf("conns: %#v\nconn: %#v\n", conns, remove)
		panic("Conn not found")
	}
	copy(conns[i:], conns[i+1:]) // shift down
	conns[len(conns)-1] = nil    // nil last element
	return conns[:len(conns)-1]  // truncate slice
}
