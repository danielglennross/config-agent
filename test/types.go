package test

import "net/http/httptest"

// Server extendable httptest.Server
type Server struct {
	*httptest.Server
}

// Config key value
type Config struct {
	Key string
}
