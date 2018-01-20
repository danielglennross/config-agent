package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/danielglennross/config-agent/broadcast"

	"github.com/Sirupsen/logrus"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/routing"
	"github.com/danielglennross/config-agent/store"
)

/*
curl --include --no-buffer --header "Connection: Upgrade" --header "Upgrade: websocket" --header "Host: localhost:8080" --header "Origin: http://localhost:8080" --header "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" --header "Sec-WebSocket-Version: 13" --header "X-Correlation-Token: test" http://localhost:8080/config/testbag

curl -X PUT -d "{\"key1\":\"value\"}" -H "X-Correlation-Token: test" http://localhost:8080/config/testbag
*/

var (
	log = logrus.WithField("app", "config-agent")
)

func main() {
	close := &err.Close{
		Exit: &[]chan bool{},
		Wg:   &sync.WaitGroup{},
	}

	redisPool, err := broadcast.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		fmt.Printf("\nerror: %s", err)
		return
	}

	st := store.NewRedisBagStore(redisPool)
	br := broadcast.NewRedisReceiver(redisPool, close)
	bw := broadcast.NewRedisWriter(redisPool, close)

	br.Init()
	bw.Init()

	go bw.Run()

	srv := &http.Server{Addr: ":8080"}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.WithError(err).Fatal("Failed to run server")
			os.Exit(1)
		}
	}()

	r := routing.NewRouter(st, br, bw)
	http.Handle("/", r)

	close.Wg.Add(1)
	go handleSignal(close)

	close.Wg.Wait()
	fmt.Println("clossssed")

	redisPool.Close()
	srv.Shutdown(nil)
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
	fmt.Printf("\nreceived %s signal, stopping profiles gracefully\n", sig)

	for _, exit := range *close.Exit {
		exit <- true
	}

	signal.Stop(c)
}
