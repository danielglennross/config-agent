package main

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/test"
)

func TestMyServer(t *testing.T) {
	pool, err := test.CreateRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

	server, destroyServer, err := test.CreateServer(pool, func(h *hub.Hub) error {
		return h.DelBag("testbag")
	})

	if err != nil {
		t.Fatal(err)
		return
	}
	defer server.Close()

	config := []test.Config{
		test.Config{Key: "value"},
		test.Config{Key: "updated-value"},
	}

	_, err = server.PopulateBag("testbag", &config[0])
	if err != nil {
		t.Fatal(err)
		return
	}

	c, err := server.CreateWebSocket("testbag")
	if err != nil {
		t.Fatal(err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < len(config); i++ {
			_, message, err := c.ReadMessage()
			if err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			// remove trailing quotes, and escaped quotes
			message = message[1 : len(message)-2]
			message = bytes.Replace(message, []byte("\\\""), []byte("\""), -1)

			var c test.Config
			err = json.Unmarshal(message, &c)
			if c.Key != config[i].Key {
				t.Errorf("websocket message incorrect %s", message)
				return
			}
		}
	}()

	_, err = server.PopulateBag("testbag", &config[1])

	wg.Wait()

	err = c.Close()
	if err != nil {
		t.Fatal(err)
		return
	}

	removed, err := server.VerifyChannelRemoved(pool, "testbag")
	if err != nil {
		t.Fatal(err)
		return
	}
	if !removed {
		t.Errorf("channel was not removed")
		return
	}

	destroyServer()
}
