package test

import (
	"fmt"
	"net/http/httptest"
	"sync"

	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/logger"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/routing"
	"github.com/danielglennross/config-agent/store"
	redigo "github.com/garyburd/redigo/redis"
)

// CreateRedisPool create a redis pool
func CreateRedisPool(setupPool func(c redigo.Conn) error) (*redigo.Pool, error) {
	redisPool, err := broadcast.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		return nil, err
	}

	c := redisPool.Get()
	defer c.Close()

	err = setupPool(c)
	if err != nil {
		return nil, err
	}

	return redisPool, nil
}

// CreateServer create a test server
func CreateServer(redisPool *redigo.Pool, setupStore func(store store.BagStore) error) (*Server, func() error, error) {
	log := logger.NewLogger(logger.Fields{
		"app": "config-agent-test",
	})
	log.SetLevel(logger.InfoLevel)

	close := &err.Close{
		Mu:   &sync.Mutex{},
		Exit: &[]chan bool{},
		Wg:   &sync.WaitGroup{},
	}

	st := store.NewRedisBagStore(redisPool)
	err := setupStore(st)
	if err != nil {
		return nil, nil, err
	}

	ws := broadcast.NewWebSocketManager(close, log)
	br := broadcast.NewRedisReceiver(redisPool, close, ws, log)
	bw := broadcast.NewRedisWriter(redisPool, close, log)

	br.Init()
	bw.Init()

	go bw.Run()

	server := httptest.NewServer(routing.NewRouter(st, br, bw, log))
	fmt.Println(server.URL)

	var tearDowns []func()
	tearDowns = append(tearDowns, func() {
		redisPool.Close()
	})

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
