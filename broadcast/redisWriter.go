package broadcast

import (
	"fmt"
	"time"

	"github.com/danielglennross/config-agent/err"
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

// RedisWriter publishes messages to the Redis CHANNEL
type RedisWriter struct {
	connFactory func() redis.Conn
	conn        redis.Conn
	close       *err.Close

	messages chan *Message
}

// NewRedisWriter ctor
func NewRedisWriter(pool *redis.Pool, close *err.Close) Writer {
	return &RedisWriter{
		connFactory: func() redis.Conn {
			return pool.Get()
		},
		conn:     nil,
		close:    close,
		messages: make(chan *Message, 10000),
	}
}

// Init init
func (rw *RedisWriter) Init() {
	rw.conn = rw.connFactory()
}

// Publish to Redis via channel.
func (rw *RedisWriter) Publish(message *Message) {
	rw.messages <- message
}

// Run the main redisWriter loop that publishes incoming messages to Redis.
func (rw *RedisWriter) Run() {
	rw.close.Wg.Add(1)
	defer rw.close.Wg.Done()

	exit := make(chan bool)
	*rw.close.Exit = append(*rw.close.Exit, exit)

	writeToRedis := func(msg *Message) error {
		if err := rw.conn.Send("PUBLISH", msg.Channel, msg.Data); err != nil {
			return errors.Wrap(err, "Unable to publish message to Redis")
		}

		if err := rw.conn.Flush(); err != nil {
			return errors.Wrap(err, "Unable to flush published message to Redis")
		}

		return nil
	}

	reDeliver := func(msg *Message) {
		time.Sleep(time.Millisecond * 300)
		msg.DeliveryAttempt++
		rw.Publish(msg)
	}

	dispose := func() {
		rw.conn.Close()
	}

	for {
		select {
		case <-exit:
			fmt.Println("disposing writer")
			dispose()
			fmt.Println("disposed writer")
			return
		case msg := <-rw.messages:
			if err := writeToRedis(msg); err != nil && msg.DeliveryAttempt < 3 {
				go reDeliver(msg)
			}
		}
	}
}
