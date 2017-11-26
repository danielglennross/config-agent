package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/danielglennross/config-agent/api"
	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/redis"
)

func main() {
	redisPool, err := redis.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		fmt.Printf("error: %s", err)
		return
	}

	h := hub.NewHub(redisPool)
	rr := redis.NewReceiver(redisPool)
	rw := redis.NewWriter(redisPool)

	go func() {
		err = rw.Run()
		if err != nil {
			fmt.Printf("error: %s", err)
			return
		}
	}()

	r := makeRouter(h, rr, rw)
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

func makeRouter(h *hub.Hub, rr *redis.Receiver, rw *redis.Writer) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/config/{bag}", api.HandleWebsocket(h, rr)).Methods("GET")
	r.HandleFunc("/config/{bag}", api.UpdateBagHandler(h, rw)).Methods("PUT")
	return r
}
