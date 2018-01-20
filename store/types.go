package store

// BagStore interface
type BagStore interface {
	Set(bag string, data []byte) error
	Get(bag string) ([]byte, error)
	Del(bag string) error
}
