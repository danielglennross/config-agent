package broadcast

import (
	"github.com/gorilla/websocket"
)

// DeliverableTrackedMessage Message tracked with meta
type DeliverableTrackedMessage struct {
	DeliveryAttempt int
	TrackedMessage
}

// TrackedMessage Message tracked with meta
type TrackedMessage struct {
	CorrelationToken string
	Message
}

// Message channel -> data
type Message struct {
	Channel string
	Data    []byte
}

// SerializableRedisMessage Message tracked with meta
type SerializableRedisMessage struct {
	CorrelationToken [16]byte
	Data             [256]byte // 8e+6
}

// Connection channel -> web socket connection
type Connection struct {
	ID            string
	Data          []byte
	Channel       string
	Websocket     *websocket.Conn
	WebSocketSent chan error
}

// Messenger messenger
type Messenger interface {
	Send(msg *TrackedMessage)
	ConnectionExists(channel string) bool
	Register(connection *Connection)
	Dispose()
}

// Receiver receiver
type Receiver interface {
	Init()
	Run(channel string)
	Broadcast(msg *TrackedMessage)
	Register(connection *Connection)
}

// Writer writer
type Writer interface {
	Init()
	Run()
	Publish(message *DeliverableTrackedMessage)
}
