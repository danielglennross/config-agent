package main

import (
	"fmt"
	"sync"
	"testing"

	"github.com/danielglennross/config-agent/hub"
	"github.com/danielglennross/config-agent/test"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
)

func createServerAndRedisPool() (server *test.Server, destroyServer func() error, pool *redigo.Pool, err error) {
	pool, err = test.CreateRedisPool(func(c redigo.Conn) (err error) {
		_, err = c.Do("FLUSHALL")
		return
	})

	if err != nil {
		return
	}

	server, destroyServer, err = test.CreateServer(pool, func(h *hub.Hub) error {
		return h.DelBag("testbag")
	})

	return
}

func createServersAndRedisPool(numberOfServers int) (servers []*test.Server, destroyServers func() error, pool *redigo.Pool, err error) {
	pool, err = test.CreateRedisPool(func(c redigo.Conn) (err error) {
		_, err = c.Do("FLUSHALL")
		return
	})

	if err != nil {
		return
	}

	var cleanUp []func() error
	for i := 0; i < numberOfServers; i++ {
		server, destroyServer, err := test.CreateServer(pool, func(h *hub.Hub) error {
			return h.DelBag("testbag")
		})
		if err != nil {
			return nil, nil, nil, err
		}

		servers = append(servers, server)
		cleanUp = append(cleanUp, destroyServer)
	}

	destroyServers = func() error {
		for _, clean := range cleanUp {
			err := clean()
			if err != nil {
				return err
			}
		}
		return nil
	}

	return
}

func Test_SingleServer_BagExists_SingleWebSocket_BagUpdated_WebSocketUpdated(t *testing.T) {
	server, destroyServer, pool, err := createServerAndRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

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
			item := &test.Config{}
			err := c.ReadJSON(item)
			if err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			fmt.Printf("\nGOT MSG: %s", item)

			if item.Key != config[i].Key {
				t.Errorf("websocket message incorrect %s", item)
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

	removed, err := server.VerifyChannelRemoved(pool, "testbag", &test.Config{})
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

func Test_SingleServer_BagMissing_SingleWebSocket_BagUpdated_WebSocketUpdated(t *testing.T) {
	server, destroyServer, pool, err := createServerAndRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

	config := []test.Config{
		test.Config{Key: ""},
		test.Config{Key: "updated-value"},
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
			item := &test.Config{}
			err := c.ReadJSON(item)
			if i > 0 && err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			fmt.Printf("\nGOT MSG: %s", item)

			if item.Key != config[i].Key {
				t.Errorf("websocket message incorrect %s", item)
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

	removed, err := server.VerifyChannelRemoved(pool, "testbag", &test.Config{})
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

func Test_SingleServer_BagExists_SingleWebSocket_BagUpdatedTwice_WebSocketUpdated(t *testing.T) {
	server, destroyServer, pool, err := createServerAndRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

	config := []test.Config{
		test.Config{Key: "value"},
		test.Config{Key: "updated-value-1"},
		test.Config{Key: "updated-value-2"},
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
			item := &test.Config{}
			err := c.ReadJSON(item)
			if err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			fmt.Printf("\nGOT MSG: %s", item)

			if item.Key != config[i].Key {
				t.Errorf("websocket message incorrect %s", item)
				return
			}
		}
	}()

	_, err = server.PopulateBag("testbag", &config[1])

	_, err = server.PopulateBag("testbag", &config[2])

	wg.Wait()

	err = c.Close()
	if err != nil {
		t.Fatal(err)
		return
	}

	removed, err := server.VerifyChannelRemoved(pool, "testbag", &test.Config{})
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

func Test_SingleServer_BagExists_SingleWebSocket_DisconnectWebSocket_ConnnectWebSocket(t *testing.T) {
	server, destroyServer, pool, err := createServerAndRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

	config := &test.Config{Key: "value"}

	listen := func(wg *sync.WaitGroup, c *websocket.Conn) {
		defer wg.Done()
		item := &test.Config{}
		err := c.ReadJSON(item)
		if err != nil {
			t.Errorf("failed to read websocket msg %s", err)
			return
		}

		fmt.Printf("\nGOT MSG: %s", item)

		if item.Key != config.Key {
			t.Errorf("websocket message incorrect %s", item)
			return
		}
	}

	_, err = server.PopulateBag("testbag", config)
	if err != nil {
		t.Fatal(err)
		return
	}

	for i := 0; i < 2; i++ {
		c, err := server.CreateWebSocket("testbag")
		if err != nil {
			t.Fatal(err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go listen(&wg, c)

		wg.Wait()

		err = c.Close()
		if err != nil {
			t.Fatal(err)
			return
		}

		removed, err := server.VerifyChannelRemoved(pool, "testbag", config)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !removed {
			t.Errorf("channel was not removed")
			return
		}
	}

	destroyServer()
}

func Test_SingleServer_BagExists_SingleWebSocket_DisconnectWebSocket_UpdateBag_ConnnectWebSocket(t *testing.T) {
	server, destroyServer, pool, err := createServerAndRedisPool()
	if err != nil {
		t.Fatal(err)
		return
	}

	listen := func(wg *sync.WaitGroup, c *websocket.Conn, config *test.Config) {
		defer wg.Done()
		item := &test.Config{}
		err := c.ReadJSON(item)
		if err != nil {
			t.Errorf("failed to read websocket msg %s", err)
			return
		}

		fmt.Printf("\nGOT MSG: %s", item)

		if item.Key != config.Key {
			t.Errorf("websocket message incorrect %s", item)
			return
		}
	}

	config := []*test.Config{
		&test.Config{Key: "value"},
		&test.Config{Key: "updated-value"},
	}

	for i := 0; i < len(config); i++ {
		_, err = server.PopulateBag("testbag", config[i])
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

		go listen(&wg, c, config[i])

		wg.Wait()

		err = c.Close()
		if err != nil {
			t.Fatal(err)
			return
		}

		removed, err := server.VerifyChannelRemoved(pool, "testbag", config[i])
		if err != nil {
			t.Fatal(err)
			return
		}
		if !removed {
			t.Errorf("channel was not removed")
			return
		}
	}

	destroyServer()
}

func Test_MultiServer_BagExists_ConnectMultiWebSockets(t *testing.T) {
	servers, destroyServers, pool, err := createServersAndRedisPool(2)
	if err != nil {
		t.Fatal(err)
		return
	}

	config := test.Config{Key: "value"}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag", &config)
	if err != nil {
		t.Fatal(err)
		return
	}

	var wgs sync.WaitGroup
	wgs.Add(len(servers))

	for _, s := range servers {
		go func(server *test.Server) {
			defer wgs.Done()
			c, err := server.CreateWebSocket("testbag")
			if err != nil {
				t.Fatal(err)
				return
			}

			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()
				item := &test.Config{}
				err := c.ReadJSON(item)
				if err != nil {
					t.Errorf("failed to read websocket msg %s", err)
					return
				}

				fmt.Printf("\nGOT MSG: %s", item)

				if item.Key != config.Key {
					t.Errorf("websocket message incorrect %s", item)
					return
				}
			}()

			wg.Wait()

			err = c.Close()
			if err != nil {
				t.Fatal(err)
				return
			}
		}(s)
	}

	wgs.Wait()

	wgs.Add(len(servers))

	for _, s := range servers {
		go func(server *test.Server) {
			removed, err := server.VerifyChannelRemoved(pool, "testbag", &config)
			if err != nil {
				t.Fatal(err)
				return
			}
			if !removed {
				t.Errorf("channel was not removed")
				return
			}
		}(s)
	}

	destroyServers()
}

func Test_MultiServer_BagExists_ConnectMultiWebSockets_BagUpdated_WebSocketsUpdated(t *testing.T) {
	servers, destroyServers, pool, err := createServersAndRedisPool(2)
	if err != nil {
		t.Fatal(err)
		return
	}

	config := &test.Config{Key: "value"}

	readWebsocket := func(c *websocket.Conn, wg *sync.WaitGroup) {
		defer wg.Done()
		item := &test.Config{}
		err := c.ReadJSON(item)
		if err != nil {
			t.Errorf("failed to read websocket msg %s", err)
			return
		}

		fmt.Printf("\nGOT MSG: %s", item)

		if item.Key != config.Key {
			t.Errorf("websocket message incorrect %s", item)
			return
		}
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag", config)
	if err != nil {
		t.Fatal(err)
		return
	}

	var wgs sync.WaitGroup
	wgs.Add(len(servers))

	var conns []*websocket.Conn
	for _, s := range servers {
		go func(server *test.Server) {
			defer wgs.Done()
			c, err := server.CreateWebSocket("testbag")
			if err != nil {
				t.Fatal(err)
				return
			}
			conns = append(conns, c)

			var wg sync.WaitGroup
			wg.Add(1)

			go readWebsocket(c, &wg)

			wg.Wait()
		}(s)
	}

	wgs.Wait()

	config = &test.Config{Key: "updated-value"}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag", config)
	if err != nil {
		t.Fatal(err)
		return
	}

	wgs.Add(len(servers))

	for _, conn := range conns {
		go func(c *websocket.Conn) {
			defer wgs.Done()

			var wg sync.WaitGroup
			wg.Add(1)

			go readWebsocket(c, &wg)

			wg.Wait()

			err = c.Close()
			if err != nil {
				t.Fatal(err)
				return
			}
		}(conn)
	}

	wgs.Wait()

	wgs.Add(len(servers))

	for _, s := range servers {
		go func(server *test.Server) {
			removed, err := server.VerifyChannelRemoved(pool, "testbag", &test.Config{})
			if err != nil {
				t.Fatal(err)
				return
			}
			if !removed {
				t.Errorf("channel was not removed")
				return
			}
		}(s)
	}

	destroyServers()
}

func Test_MultiServer_BagExists_ConnectWebsocket_BagUpdated_ConnectSecondWebsocket_WebSocketsUpdated(t *testing.T) {
	servers, destroyServers, pool, err := createServersAndRedisPool(2)
	if err != nil {
		t.Fatal(err)
		return
	}

	config := []*test.Config{
		&test.Config{Key: "value"},
		&test.Config{Key: "updated-value"},
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag", config[0])
	if err != nil {
		t.Fatal(err)
		return
	}

	c0, err := servers[0].CreateWebSocket("testbag")
	if err != nil {
		t.Fatal(err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < len(config); i++ {
			item := &test.Config{}
			err := c0.ReadJSON(item)
			if err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			fmt.Printf("\nGOT MSG: %s", item)

			if item.Key != config[i].Key {
				t.Errorf("websocket message incorrect %s", item)
				return
			}
		}
	}()

	_, err = servers[0].PopulateBag("testbag", config[1])

	wg.Wait()

	c1, err := servers[1].CreateWebSocket("testbag")
	if err != nil {
		t.Fatal(err)
		return
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		item := &test.Config{}
		err := c1.ReadJSON(item)
		if err != nil {
			t.Errorf("failed to read websocket msg %s", err)
			return
		}

		fmt.Printf("\nGOT MSG: %s", item)

		if item.Key != config[1].Key {
			t.Errorf("websocket message incorrect %s", item)
			return
		}
	}()

	wg.Wait()

	for _, s := range servers {
		go func(server *test.Server) {
			removed, err := server.VerifyChannelRemoved(pool, "testbag", &test.Config{})
			if err != nil {
				t.Fatal(err)
				return
			}
			if !removed {
				t.Errorf("channel was not removed")
				return
			}
		}(s)
	}

	destroyServers()
}

func Test_MultiServer_MultipleBagExists_ConnectMultiWebSockets(t *testing.T) {
	servers, destroyServers, pool, err := createServersAndRedisPool(2)
	if err != nil {
		t.Fatal(err)
		return
	}

	config0 := &test.Config{Key: "value-0"}
	config1 := &test.Config{Key: "value-1"}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag0", config0)
	if err != nil {
		t.Fatal(err)
		return
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag1", config1)
	if err != nil {
		t.Fatal(err)
		return
	}

	var wgs sync.WaitGroup
	wgs.Add(len(servers))

	connectWebSocket := func(server *test.Server, bag string, config *test.Config) {
		defer wgs.Done()
		c, err := server.CreateWebSocket(bag)
		if err != nil {
			t.Fatal(err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			item := &test.Config{}
			err := c.ReadJSON(item)
			if err != nil {
				t.Errorf("failed to read websocket msg %s", err)
				return
			}

			fmt.Printf("\nGOT MSG: %s", item)

			if item.Key != config.Key {
				t.Errorf("websocket message incorrect %s", item)
				return
			}
		}()

		wg.Wait()

		err = c.Close()
		if err != nil {
			t.Fatal(err)
			return
		}
	}

	go connectWebSocket(servers[0], "testbag0", config0)
	go connectWebSocket(servers[1], "testbag1", config1)

	wgs.Wait()

	wgs.Add(len(servers))

	cleanUp := func(server *test.Server, bag string, config *test.Config) {
		defer wgs.Done()
		removed, err := server.VerifyChannelRemoved(pool, bag, config)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !removed {
			t.Errorf("channel was not removed")
			return
		}
	}

	go cleanUp(servers[0], "testbag0", config0)
	go cleanUp(servers[1], "testbag1", config1)

	destroyServers()
}

func Test_MultiServer_MultipleBagExists_ConnectMultiWebSockets_BagsUpdated_WebSocketsUpdated(t *testing.T) {
	servers, destroyServers, pool, err := createServersAndRedisPool(2)
	if err != nil {
		t.Fatal(err)
		return
	}

	config0 := &test.Config{Key: "value-0"}
	config1 := &test.Config{Key: "value-1"}

	readWebsocket := func(c *websocket.Conn, config *test.Config, wg *sync.WaitGroup) {
		defer wg.Done()
		item := &test.Config{}
		err := c.ReadJSON(item)
		if err != nil {
			t.Errorf("failed to read websocket msg %s", err)
			return
		}

		fmt.Printf("\nGOT MSG: %s", item)

		if item.Key != config.Key {
			t.Errorf("websocket message incorrect %s", item)
			return
		}
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag0", config0)
	if err != nil {
		t.Fatal(err)
		return
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag1", config1)
	if err != nil {
		t.Fatal(err)
		return
	}

	c0 := make(chan *websocket.Conn)
	c1 := make(chan *websocket.Conn)
	createWebSocket := func(server *test.Server, bag string, config *test.Config, connChan chan *websocket.Conn) {
		c, err := server.CreateWebSocket(bag)
		if err != nil {
			t.Fatal(err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go readWebsocket(c, config, &wg)

		wg.Wait()
		connChan <- c
	}

	go createWebSocket(servers[0], "testbag0", config0, c0)
	go createWebSocket(servers[1], "testbag1", config1, c1)

	conn0 := <-c0
	conn1 := <-c1

	config0 = &test.Config{Key: "updated-value-0"}
	config1 = &test.Config{Key: "updated-value-1"}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag0", config0)
	if err != nil {
		t.Fatal(err)
		return
	}

	// populate only on one server
	_, err = servers[0].PopulateBag("testbag1", config1)
	if err != nil {
		t.Fatal(err)
		return
	}

	var wgs sync.WaitGroup
	wgs.Add(len(servers))

	readAndCloseWebsocket := func(c *websocket.Conn, config *test.Config) {
		defer wgs.Done()

		var wg sync.WaitGroup
		wg.Add(1)

		go readWebsocket(c, config, &wg)

		wg.Wait()

		err = c.Close()
		if err != nil {
			t.Fatal(err)
			return
		}
	}

	go readAndCloseWebsocket(conn0, config0)
	go readAndCloseWebsocket(conn1, config1)

	wgs.Wait()

	wgs.Add(len(servers))

	cleanUp := func(server *test.Server, bag string, config *test.Config) {
		defer wgs.Done()
		removed, err := server.VerifyChannelRemoved(pool, bag, config)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !removed {
			t.Errorf("channel was not removed")
			return
		}
	}

	go cleanUp(servers[0], "testbag0", config0)
	go cleanUp(servers[1], "testbag1", config1)

	destroyServers()
}
