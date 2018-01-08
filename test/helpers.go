package test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	redigo "github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
)

// PopulateBag populate bag
func (server *Server) PopulateBag(bag string, config *Config) (*http.Response, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", server.URL+"/config/"+bag, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-Token", "token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateWebSocket init web socket for bag
func (server *Server) CreateWebSocket(bag string) (*websocket.Conn, error) {
	ws := strings.Replace(server.URL, "http://", "", -1)

	addr := flag.String("addr", ws, "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/config/" + bag}

	header := http.Header{"X-Correlation-Token": []string{"token"}}

	c, res, err := websocket.DefaultDialer.Dial(u.String(), header)
	body, _ := ioutil.ReadAll(res.Body)
	fmt.Println("response Body:", string(body))
	if err != nil {
		return nil, err
	}
	return c, nil
}

// VerifyChannelRemoved verifies a channel is removed
func (server *Server) VerifyChannelRemoved(redisPool *redigo.Pool, bag string) (bool, error) {
	_, err := server.PopulateBag(bag, &Config{})
	_, err = server.PopulateBag(bag, &Config{})
	if err != nil {
		return false, err
	}

	time.Sleep(time.Millisecond * 500) // wait for server job to unsubscribe

	conn := redisPool.Get()
	defer conn.Close()

	val, err := conn.Do("PUBSUB", "CHANNELS")
	if err != nil {
		return false, err
	}

	channels := val.([]interface{})
	return len(channels) == 0, nil
}
