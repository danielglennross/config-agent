package test

import (
	"fmt"
	"net/http/httptest"
	"sync"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/redis"
	"github.com/danielglennross/config-agent/routing"
	redigo "github.com/garyburd/redigo/redis"
)

// CreateRedisPool create a redis pool
func CreateRedisPool() (*redigo.Pool, error) {
	redisPool, err := redis.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		return nil, err
	}
	return redisPool, nil
}

// CreateServer create a test server
func CreateServer(redisPool *redigo.Pool, setupHub func(h *hub.Hub) error) (*Server, func() error, error) {
	close := &err.Close{
		Exit: &[]chan bool{},
		Wg:   &sync.WaitGroup{},
	}

	h := hub.NewHub(redisPool)
	err := setupHub(h)
	if err != nil {
		return nil, nil, err
	}

	rr := redis.NewReceiver(redisPool, close)
	rw := redis.NewWriter(redisPool, close)

	rr.Init()
	go rw.Run()

	server := httptest.NewServer(routing.NewRouter(h, rr, rw))
	fmt.Println(server.URL)

	var tearDowns []func()
	tearDowns = append(tearDowns, func() {
		for _, exit := range *close.Exit {
			exit <- true
		}
	})

	destroyServer := func() error {
		if len(tearDowns) > 0 {
			for _, fn := range tearDowns {
				fn()
			}
		}

		server.CloseClientConnections()
		server.Close()
		return nil
	}

	return &Server{server}, destroyServer, nil
}
