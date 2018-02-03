package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/danielglennross/config-agent/broadcast"
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

	badRequest  = setErrResponse(http.StatusBadRequest)
	serverError = setErrResponse(http.StatusInternalServerError)
)

type errResponse struct {
	Code   string
	Detail string
}

func setErrResponse(httpCode int) func(w http.ResponseWriter, err *errResponse) {
	return func(w http.ResponseWriter, err *errResponse) {
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
		correlationToken := r.Header.Get("X-Correlation-Token")
		if correlationToken == "" {
			badRequest(w, &errResponse{Code: "missing-correlation-token", Detail: "correlation token cannot be empty."})
			return
		}

		vars := mux.Vars(r)
		bag := vars["bag"]

		if bag == "" {
			badRequest(w, &errResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			badRequest(w, &errResponse{Code: "missing-payload", Detail: "payload cannot be empty."})
			return
		}

		// update in redis
		err = store.Set(bag, body)
		if err != nil {
			serverError(w, &errResponse{Code: "update-bag-fail", Detail: "failed to update bag."})
			return
		}

		// publish to redis
		writer.Publish(&broadcast.Message{Channel: bag, Data: body, DeliveryAttempt: 0})

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleWebsocket applies web scoket connection
func HandleWebsocket(store store.BagStore, br broadcast.Receiver) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		correlationToken := r.Header.Get("X-Correlation-Token")
		if correlationToken == "" {
			badRequest(w, &errResponse{Code: "missing-correlation-token", Detail: "correlation token cannot be empty."})
			return
		}

		vars := mux.Vars(r)
		bag := vars["bag"]
		if bag == "" {
			badRequest(w, &errResponse{Code: "missing-bag", Detail: "bag cannot be empty."})
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("\nfail websocket: %s", err)
			serverError(w, &errResponse{Code: "websocket-upgrade", Detail: "unable to upgrade to websockets."})
			return
		}

		go br.Run(bag)

		//return the in memory collection
		data, err := store.Get(bag)
		if err != nil {
			serverError(w, &errResponse{Code: "get-bag-fail", Detail: "failed to get bag."})
			return
		}

		connection := &broadcast.Connection{
			Websocket:     ws,
			Channel:       bag,
			Id:            uuid.NewV4().String(),
			Data:          data,
			WebSocketSent: make(chan error),
		}

		br.Register(connection)

		br.Message(connection)
		<-connection.WebSocketSent

		//ws.WriteMessage(websocket.TextMessage, data)
	}
}
