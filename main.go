package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/redis"
	"github.com/danielglennross/config-agent/routing"
)

var (
	log = logrus.WithField("app", "config-agent")
)

func main() {
	close := &err.Close{
		Exit: &[]chan bool{},
		Wg:   &sync.WaitGroup{},
	}

	redisPool, err := redis.NewRedisPoolFromURL("redis://localhost:6379")
	if err != nil {
		fmt.Printf("error: %s", err)
		return
	}

	h := hub.NewHub(redisPool)
	rr := redis.NewReceiver(redisPool, close)
	rw := redis.NewWriter(redisPool, close)

	rr.Init()
	go rw.Run()

	srv := &http.Server{Addr: ":8080"}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.WithError(err).Fatal("Failed to run server")
			os.Exit(1)
		}
	}()

	r := routing.NewRouter(h, rr, rw)
	http.Handle("/", r)

	close.Wg.Add(1)
	go handleSignal(close)

	close.Wg.Wait()

	//rr.Destroy()
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
	fmt.Printf("received %s signal, stopping profiles gracefully\n", sig)

	fmt.Printf("no: exit channels %d\n", len(*close.Exit))
	for _, exit := range *close.Exit {
		exit <- true
	}

	signal.Stop(c)
}
