package broadcast

import (
	"time"

	"github.com/danielglennross/config-agent/err"
	"github.com/danielglennross/config-agent/logger"
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

// RedisWriter publishes messages to the Redis CHANNEL
type RedisWriter struct {
	connFactory func() redis.Conn
	conn        redis.Conn
	close       *err.Close

	messages chan *DeliverableTrackedMessage
	log      *logger.Logger
}

// NewRedisWriter ctor
func NewRedisWriter(pool *redis.Pool, close *err.Close, log *logger.Logger) Writer {
	return &RedisWriter{
		connFactory: func() redis.Conn {
			return pool.Get()
		},
		conn:     nil,
		close:    close,
		messages: make(chan *DeliverableTrackedMessage, 10000),
		log:      log,
	}
}

// Init init
func (rw *RedisWriter) Init() {
	rw.conn = rw.connFactory()
}

// Publish to Redis via channel.
func (rw *RedisWriter) Publish(message *DeliverableTrackedMessage) {
	rw.messages <- message
}

// Run the main redisWriter loop that publishes incoming messages to Redis.
func (rw *RedisWriter) Run() {
	rw.close.Wg.Add(1)
	defer rw.close.Wg.Done()

	exit := make(chan bool)
	rw.close.Mu.Lock()
	*rw.close.Exit = append(*rw.close.Exit, exit)
	rw.close.Mu.Unlock()

	writeToRedis := func(msg *DeliverableTrackedMessage) error {
		data, err := PackWriterMessage(msg)
		if err != nil {
			return errors.Wrap(err, "Unable to serialize message for Redis")
		}

		if err := rw.conn.Send("PUBLISH", msg.Channel, data); err != nil {
			return errors.Wrap(err, "Unable to publish message to Redis")
		}

		if err := rw.conn.Flush(); err != nil {
			return errors.Wrap(err, "Unable to flush published message to Redis")
		}

		return nil
	}

	reDeliver := func(msg *DeliverableTrackedMessage) {
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
			rw.log.InfoMsg("disposing writer")
			dispose()
			rw.log.InfoMsg("disposed writer")
			return
		case msg := <-rw.messages:
			if err := writeToRedis(msg); err != nil && msg.DeliveryAttempt < 3 {
				go reDeliver(msg)
			}
		}
	}
}
