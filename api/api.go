package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type errResponse struct {
	Code   string
	Detail string
}

type bagCollection struct {
}

func setErrResponse(w http.ResponseWriter, err *errResponse) {
	body, _ := json.Marshal(err)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, string(body))
}

// UpdateBagHandler handler to update bags
func UpdateBagHandler(h *hub.Hub, rw *redis.Writer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationToken := r.Header.Get("X-Correlation-Token")
		if correlationToken == "" {
			setErrResponse(w, &errResponse{Code: "missing-correlation-token", Detail: "correlation token cannot be empty."})
			return
		}

		vars := mux.Vars(r)
		bag := vars["bag"]

		fmt.Printf("\nI got here %s", bag)

		if bag == "" {
			setErrResponse(w, &errResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			setErrResponse(w, &errResponse{Code: "missing-payload", Detail: "payload cannot be empty."})
			return
		}

		// update in redis
		_ = h.SetBag(bag, body)

		// publish to redis
		rw.Publish(&redis.Message{Channel: bag, Data: body})

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleWebsocket applies web scoket connection
func HandleWebsocket(h *hub.Hub, rr *redis.Receiver) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationToken := r.Header.Get("X-Correlation-Token")
		if correlationToken == "" {
			setErrResponse(w, &errResponse{Code: "missing-correlation-token", Detail: "correlation token cannot be empty."})
			return
		}

		vars := mux.Vars(r)
		bag := vars["bag"]
		if bag == "" {
			setErrResponse(w, &errResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("\nfail websocket: %s", err)
			setErrResponse(w, &errResponse{Code: "websocket-upgrade", Detail: "unable to upgrade to websockets."})
			return
		}

		go func() {
			err = rr.Run(bag)
			if err != nil {
				setErrResponse(w, &errResponse{Code: "run", Detail: "failed to run."})
				return
			}
		}()

		rr.Register(&redis.Connection{Websocket: ws, Channel: bag})

		//return the in memory collection
		data, _ := h.GetBag(bag)

		fmt.Printf("\n hub bag: %s", data)

		ws.WriteMessage(websocket.TextMessage, data)

		//for {
		//	mt, data, err := ws.ReadMessage()
		//	fmt.Printf("\n mt: %d - data: %s - error: %s", mt, data, err)

		//ws.Close()
		//	rr.DeRegister(&redis.Connection{Websocket: ws, Channel: bag})
		// if err != nil {
		// 	if websocket.IsCloseError(err, websocket.CloseGoingAway) || err == io.EOF {
		// 		return
		// 	}
		// }
		// switch mt {
		// case websocket.TextMessage:
		// 	fmt.Printf("read websocket: %s", data)
		// default:
		// 	fmt.Println("Unknown Message!")
		// }
		//}

		//rr.DeRegister(&redis.Connection{Websocket: ws, Channel: bag})

		//ws.WriteMessage(websocket.CloseMessage, []byte{})
	}
}
