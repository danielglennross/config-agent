package broadcast

import (
	"github.com/gorilla/websocket"
)

// Message channel -> data
type Message struct {
	Channel         string
	Data            []byte
	DeliveryAttempt int
}

// Connection channel -> web socket connection
type Connection struct {
	ID            string
	Data          []byte
	Channel       string
	Websocket     *websocket.Conn
	WebSocketSent chan error
}

// Messanger messanger
type Messanger interface {
	Send(msg *Message)
	ConnectionExists(channel string) bool
	Register(connection *Connection)
	Dispose()
}

// Receiver receiver
type Receiver interface {
	Init()
	Run(channel string)
	Broadcast(msg *Message)
	Register(connection *Connection)
}

// Writer writer
type Writer interface {
	Init()
	Run()
	Publish(message *Message)
}
