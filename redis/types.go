package redis

import "github.com/gorilla/websocket"

// Message channel -> data
type Message struct {
	Channel string
	Data    []byte
}

// Connection channel -> web socket connection
type Connection struct {
	Channel   string
	Websocket *websocket.Conn
}
