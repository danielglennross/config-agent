package store

import (
	"github.com/garyburd/redigo/redis"
)

// RedisBagStore redis bag store
type RedisBagStore struct {
	pool *redis.Pool
}

// NewRedisBagStore ctor
func NewRedisBagStore(pool *redis.Pool) BagStore {
	return &RedisBagStore{pool: pool}
}

// Set sets the bag
func (store *RedisBagStore) Set(bag string, data []byte) error {
	conn := store.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", bag, data)
	if err != nil {
		return err
	}

	return nil
}

// Get gets the bag
func (store *RedisBagStore) Get(bag string) ([]byte, error) {
	conn := store.pool.Get()
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

// Del deletes the bag
func (store *RedisBagStore) Del(bag string) error {
	conn := store.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", bag)
	if err != nil {
		return err
	}

	return nil
}
