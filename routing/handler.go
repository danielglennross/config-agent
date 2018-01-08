package routing

import (
	"github.com/danielglennross/config-agent/api"
	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/redis"
	"github.com/gorilla/mux"
)

// NewRouter ctor
func NewRouter(h *hub.Hub, rr *redis.Receiver, rw *redis.Writer) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/config/{bag}", api.HandleWebsocket(h, rr)).Methods("GET")
	r.HandleFunc("/config/{bag}", api.UpdateBagHandler(h, rw)).Methods("PUT")
	return r
}
