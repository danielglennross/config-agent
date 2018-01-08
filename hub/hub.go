package hub

import (
	"github.com/garyburd/redigo/redis"
)

// Hub hub
type Hub struct {
	pool *redis.Pool
}

// NewHub ctor
func NewHub(pool *redis.Pool) *Hub {
	return &Hub{pool: pool}
}

// SetBag sets the bag
func (hub *Hub) SetBag(bag string, data []byte) error {
	conn := hub.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", bag, data)
	if err != nil {
		return err
	}

	return nil
}

// GetBag gets the bag
func (hub *Hub) GetBag(bag string) ([]byte, error) {
	conn := hub.pool.Get()
	defer conn.Close()

	val, err := conn.Do("GET", bag)
	if err != nil {
		return nil, err
	}

	if val == nil {
		return nil, nil
	}

	return val.([]byte), nil
}

// DelBag gets the bag
func (hub *Hub) DelBag(bag string) error {
	conn := hub.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", bag)
	if err != nil {
		return err
	}

	return nil
}
