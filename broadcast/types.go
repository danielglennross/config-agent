package broadcast

import "github.com/gorilla/websocket"

// Message channel -> data
type Message struct {
	Channel         string
	Data            []byte
	DeliveryAttempt int
}

// Connection channel -> web socket connection
type Connection struct {
	Channel   string
	Websocket *websocket.Conn
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
