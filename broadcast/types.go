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

// WebSocketMsg webSocketMsg
type WebSocketMsg struct {
	Data []byte
	Conn *websocket.Conn
}

// Receiver receiver
type Receiver interface {
	Init()
	Run(channel string)
	Broadcast(msg *Message)
	Register(connection *Connection)

	Message(webSocketMsg *WebSocketMsg)
	WsChan() chan error
}

// Writer writer
type Writer interface {
	Init()
	Run()
	Publish(message *Message)
}
