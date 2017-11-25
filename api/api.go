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
	code   string
	detail string
}

type bagCollection struct {
}

func setErrResponse(w http.ResponseWriter, err *errResponse) {
	body, _ := json.Marshal(err)
	w.Write(body)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusBadRequest)
}

// UpdateBagHandler handler to update bags
func UpdateBagHandler(h *hub.Hub, rw *redis.Writer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		bag := vars["bag"]

		fmt.Printf("I got here %s", bag)

		if bag == "" {
			setErrResponse(w, &errResponse{code: "missing-bag", detail: "bag cannot be empty."})
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			setErrResponse(w, &errResponse{code: "missing-payload", detail: "payload cannot be empty."})
			return
		}

		// update in memory
		_ = h.SetBag(bag, body)

		// publish to redis
		rw.Publish(&redis.Message{Channel: bag, Data: body})

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleWebsocket applies web scoket connection
func HandleWebsocket(h *hub.Hub, rr *redis.Receiver) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		bag := vars["bag"]

		if bag == "" {
			setErrResponse(w, &errResponse{code: "missing-bag", detail: "bag cannot be empty."})
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		fmt.Printf("fail websocket: %s", err)
		if err != nil {
			setErrResponse(w, &errResponse{code: "websocket-upgrade", detail: "unable to upgrade to websockets."})
			return
		}

		go func() {
			err = rr.Run(bag)
			if err != nil {
				setErrResponse(w, &errResponse{code: "run", detail: "failed to run."})
				return
			}
		}()

		rr.Register(&redis.Connection{Websocket: ws, Channel: bag})

		// return the in memory collection
		data, _ := h.GetBag(bag)

		json := string(data)

		ws.WriteJSON(json)

		//rr.DeRegister(&redis.Connection{Websocket: ws, Channel: bag})

		//ws.WriteMessage(websocket.CloseMessage, []byte{})
	}
}
