package broadcast

import (
	"fmt"
	"sync"

	"github.com/danielglennross/config-agent/err"
	"github.com/gorilla/websocket"
)

// WebSocketManager ws manager
type WebSocketManager struct {
	connMu *sync.Mutex
	conns  []*Connection
	close  *err.Close
	mapp   *sync.Map
}

type deleteWs struct {
	id  string
	err error
}

// NewWebSocketManager ctor
func NewWebSocketManager(close *err.Close) *WebSocketManager {
	return &WebSocketManager{
		connMu: &sync.Mutex{},
		conns:  make([]*Connection, 0),
		close:  close,
		mapp:   &sync.Map{},
	}
}

// Register sync web socket sending
func (wsm *WebSocketManager) Register(connection *Connection) {
	exit := make(chan bool)
	wsm.close.Mu.Lock()
	*wsm.close.Exit = append(*wsm.close.Exit, exit)
	wsm.close.Mu.Unlock()

	wsm.mutateConns(connection, appendConn)

	webSocketChan := make(chan *Connection)
	wsm.mapp.Store(connection.ID, webSocketChan)
	go func() {
		for {
			select {
			case <-exit:
				fmt.Println("disposed websocket messenger")
				return
			case conn := <-webSocketChan:
				conn.WebSocketSent <- conn.Websocket.WriteMessage(websocket.TextMessage, conn.Data)
			}
		}
	}()
	webSocketChan <- connection
}

// Send message
func (wsm *WebSocketManager) Send(msg *Message) {
	var iter func(i int)
	iter = func(i int) {
		wsm.connMu.Lock()
		if i > len(wsm.conns)-1 {
			wsm.connMu.Unlock()
			return
		}
		wsm.connMu.Unlock()

		conn := wsm.conns[i]
		if conn.Channel == msg.Channel {
			result, ok := wsm.mapp.Load(conn.ID)
			if ok {
				chanConnection := result.(chan *Connection)
				chanConnection <- &Connection{ // copy connection w/ new data from redis
					ID:            conn.ID,
					Data:          msg.Data,
					Channel:       conn.Channel,
					Websocket:     conn.Websocket,
					WebSocketSent: conn.WebSocketSent,
				}
				if err := <-conn.WebSocketSent; err != nil {
					wsm.mutateConns(conn, removeConn)
					wsm.mapp.Delete(conn.ID)
					iter(i)
					return
				}
			}
		}
		i++
		iter(i)
	}
	iter(0)
}

// ConnectionExists check channel
func (wsm *WebSocketManager) ConnectionExists(channel string) bool {
	wsm.connMu.Lock()
	defer wsm.connMu.Unlock()

	found := false
	for _, conn := range wsm.conns {
		if conn.Channel == channel {
			found = true
		}
	}
	return found
}

// Dispose dispose
func (wsm *WebSocketManager) Dispose() {
	wsm.connMu.Lock()
	defer wsm.connMu.Unlock()

	if len(wsm.conns) > 0 {
		deleteReq := make(chan deleteWs)

		go func() {
			for _, c := range wsm.conns {
				deleteReq <- deleteWs{id: c.ID, err: c.Websocket.Close()}
			}
			close(deleteReq)
		}()

		for dr := range deleteReq {
			if dr.err != nil {
				fmt.Printf("\n closing web socket error: %s", dr.err)
			} else {
				wsm.mapp.Delete(dr.id)
			}
		}
	}
}

func (wsm *WebSocketManager) mutateConns(conn *Connection, fn mutateConnection) {
	wsm.connMu.Lock()
	defer wsm.connMu.Unlock()
	wsm.conns = fn(wsm.conns, conn)
}

// AppendConn add connection
func appendConn(conns []*Connection, add *Connection) []*Connection {
	conns = append(conns, add)
	return conns
}

// RemoveConn remove connection
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
		// log
		return conns
	}

	copy(conns[i:], conns[i+1:]) // shift down
	conns[len(conns)-1] = nil    // nil last element
	return conns[:len(conns)-1]  // truncate slice
}
