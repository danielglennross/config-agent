package broadcast

import (
	"fmt"

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

func (rw *RedisWriter) destroyRedis() {
	rw.conn.Close()
}

// Run the main redisWriter loop that publishes incoming messages to Redis.
func (rw *RedisWriter) Run() {
	rw.close.Wg.Add(1)
	defer rw.close.Wg.Done()

	exit := make(chan bool)
	*rw.close.Exit = append(*rw.close.Exit, exit)

	for {
		select {
		case <-exit:
			fmt.Println("exiting writer run")
			rw.destroyRedis()
			return
		case msg := <-rw.messages:
			if err := writeToRedis(rw.conn, msg); err != nil {
				rw.Publish(msg) // attempt to redeliver later
			}
		}
	}
}

// Publish to Redis via channel.
func (rw *RedisWriter) Publish(message *Message) {
	rw.messages <- message
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
