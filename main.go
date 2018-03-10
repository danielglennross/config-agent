package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/logger"
	"github.com/danielglennross/config-agent/routing"
	"github.com/danielglennross/config-agent/store"
	"github.com/twinj/uuid"
)

/*
websocket:
curl --include --no-buffer --header "Connection: Upgrade" --header "Upgrade: websocket" --header "Host: localhost:8080" --header "Origin: http://localhost:8080" --header "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" --header "Sec-WebSocket-Version: 13" --header "X-Correlation-Token: test" http://localhost:8080/config/testbag

update bag:
curl -X PUT -d "{\"key1\":\"value\"}" -H "X-Correlation-Token: test" http://localhost:8080/config/testbag
*/

var (
	log = logger.NewLogger(logger.Fields{
		"file": "main",
	})
)

func main() {
	uuid.Init()
	logger.Init(logger.InfoLevel)

	close := &err.Close{
		Mu:   &sync.Mutex{},
		Exit: &[]chan bool{},
		Wg:   &sync.WaitGroup{},
	}

	redisPool, err := broadcast.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		log.Fatal("Failed to fetch redis pool", err)
		os.Exit(1)
	}

	st := store.NewRedisBagStore(redisPool)
	ws := broadcast.NewWebSocketManager(close)
	br := broadcast.NewRedisReceiver(redisPool, close, ws)
	bw := broadcast.NewRedisWriter(redisPool, close)

	br.Init()
	bw.Init()

	go bw.Run()

	srv := &http.Server{Addr: ":8080"}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal("Failed to run server", err)
			os.Exit(1)
		}
	}()

	http.Handle("/", routing.NewRouter(st, br, bw))

	close.Wg.Add(1)
	go handleSignal(close)

	log.Info("Server running", logger.Fields{
		"add": srv.Addr,
	})

	close.Wg.Wait()

	redisPool.Close()
	srv.Shutdown(nil)

	log.InfoMsg("App killed")
}

func handleSignal(close *err.Close) {
	defer close.Wg.Done()

	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	sig := <-c
	log.Info("Received signal, stopping gracefully", logger.Fields{
		"signal": sig,
	})

	fmt.Printf("no. of exits: %d\n", len(*close.Exit))

	for _, exit := range *close.Exit {
		exit <- true
	}

	signal.Stop(c)
}
