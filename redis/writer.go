package redis

import (
	"fmt"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

// Writer publishes messages to the Redis CHANNEL
type Writer struct {
	pool  *redis.Pool
	close *err.Close

	messages chan *Message
}

// NewWriter ctor
func NewWriter(pool *redis.Pool, close *err.Close) *Writer {
	return &Writer{
		pool:     pool,
		close:    close,
		messages: make(chan *Message, 10000),
	}
}

// Run the main redisWriter loop that publishes incoming messages to Redis.
func (rw *Writer) Run() {
	conn := rw.pool.Get()
	defer conn.Close()

	rw.close.Wg.Add(1)
	defer rw.close.Wg.Done()

	exit := make(chan bool)
	*rw.close.Exit = append(*rw.close.Exit, exit)

	for {
		select {
		case <-exit:
			fmt.Println("exiting writer run")
			return
		case msg := <-rw.messages:
			if err := writeToRedis(conn, msg); err != nil {
				rw.Publish(msg) // attempt to redeliver later
			}
		}
	}
}

func writeToRedis(conn redis.Conn, message *Message) error {
	if err := conn.Send("PUBLISH", message.Channel, message.Data); err != nil {
		return errors.Wrap(err, "Unable to publish message to Redis")
	}
	if err := conn.Flush(); err != nil {
		return errors.Wrap(err, "Unable to flush published message to Redis")
	}
	return nil
}

// Publish to Redis via channel.
func (rw *Writer) Publish(message *Message) {
	rw.messages <- message
}
