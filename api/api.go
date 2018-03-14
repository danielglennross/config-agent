package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/logger"
	"github.com/danielglennross/config-agent/store"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// BadRequest response
	BadRequest = setErrResponse(http.StatusBadRequest)
	// ServerError response
	ServerError = setErrResponse(http.StatusInternalServerError)
)

// ErrResponse common err resp
type ErrResponse struct {
	Code   string
	Detail string
}

func setErrResponse(httpCode int) func(w http.ResponseWriter, err *ErrResponse) {
	return func(w http.ResponseWriter, err *ErrResponse) {
		body, _ := json.Marshal(err)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, string(body))
	}
}

// UpdateBagHandler handler to update bags
func UpdateBagHandler(store store.BagStore, writer broadcast.Writer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		bag := vars["bag"]

		if bag == "" {
			BadRequest(w, &ErrResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			BadRequest(w, &ErrResponse{Code: "missing-payload", Detail: "payload cannot be empty."})
			return
		}

		// update in redis
		err = store.Set(bag, body)
		if err != nil {
			ServerError(w, &ErrResponse{Code: "update-bag-fail", Detail: "failed to update bag."})
			return
		}

		correlationToken := r.Context().Value(logger.ContextKey("CorrelationToken")).(string)

		// publish to redis
		writer.Publish(&broadcast.DeliverableTrackedMessage{
			DeliveryAttempt: 0,
			TrackedMessage: broadcast.TrackedMessage{
				CorrelationToken: correlationToken,
				Message: broadcast.Message{
					Channel: bag,
					Data:    body,
				},
			},
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleWebsocket applies web scoket connection
func HandleWebsocket(store store.BagStore, br broadcast.Receiver) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		bag := vars["bag"]
		if bag == "" {
			BadRequest(w, &ErrResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("\nfail websocket: %s", err)
			ServerError(w, &ErrResponse{Code: "websocket-upgrade", Detail: "unable to upgrade to websockets."})
			return
		}

		go br.Run(bag)

		//return the in memory collection
		data, err := store.Get(bag)
		if err != nil {
			ServerError(w, &ErrResponse{Code: "get-bag-fail", Detail: "failed to get bag."})
			return
		}

		connection := &broadcast.Connection{
			Channel:       bag,
			Data:          data,
			ID:            uuid.NewV4().String(),
			Websocket:     ws,
			WebSocketSent: make(chan error),
		}

		br.Register(connection)
		<-connection.WebSocketSent
	}
}
