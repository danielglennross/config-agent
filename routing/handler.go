package routing

import (
	"github.com/danielglennross/config-agent/api"
	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/store"
	"github.com/gorilla/mux"
)

// NewRouter ctor
func NewRouter(store store.BagStore, br broadcast.Receiver, bw broadcast.Writer) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/config/{bag}", api.HandleWebsocket(store, br)).Methods("GET")
	r.HandleFunc("/config/{bag}", api.UpdateBagHandler(store, bw)).Methods("PUT")
	return r
}
