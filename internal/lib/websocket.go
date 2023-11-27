package lib

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ThreadSafeWebSocket wraps a websocket.Conn and allows many readers and writers to
// read/write the conn from goroutines without having to track safe access.
// This comes with the caveat that all writes block eachother, and similarly for reads.
// See https://pkg.go.dev/github.com/gorilla/websocket?utm_source=godoc#hdr-Concurrency.
type ThreadSafeWebSocket struct {
	c       *websocket.Conn
	writeMu *sync.Mutex
	readMu  *sync.Mutex
}

func NewThreadSafeWebSocket(c *websocket.Conn) ThreadSafeWebSocket {
	return ThreadSafeWebSocket{c, &sync.Mutex{}, &sync.Mutex{}}
}

func (s ThreadSafeWebSocket) ReadMessage() (int, []byte, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	return s.c.ReadMessage()
}

func (s ThreadSafeWebSocket) WriteMessage(messageType int, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.c.WriteMessage(messageType, data)
}
