package server

import (
	"go-friends-list/pkg/model"
	"net"
	"sync"
	"testing"
	"time"
)

// simpleConn implements net.Conn writing to an internal buffer for inspection.
type simpleConn struct {
	net.Conn
	mu  sync.Mutex
	buf []byte
}

func (c *simpleConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	c.buf = append(c.buf, b...)
	c.mu.Unlock()
	return len(b), nil
}

// bufLocked returns a copy of the buffer under the mutex.
func (c *simpleConn) bufLocked() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, len(c.buf))
	copy(out, c.buf)
	return out
}

func (c *simpleConn) Read(b []byte) (int, error)             { return 0, nil }
func (c *simpleConn) Close() error                            { return nil }
func (c *simpleConn) LocalAddr() net.Addr                    { return nil }
func (c *simpleConn) RemoteAddr() net.Addr                   { return nil }
func (c *simpleConn) SetDeadline(t time.Time) error          { return nil }
func (c *simpleConn) SetReadDeadline(t time.Time) error      { return nil }
func (c *simpleConn) SetWriteDeadline(t time.Time) error     { return nil }

func TestBroadcasterOnlineOffline(t *testing.T) {
	srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), iConns: make(chan net.Conn, 1), dConns: make(chan net.Conn, 1)}
	// friend connection will receive status updates
	friend := &simpleConn{}
	srv.aConns[friend] = model.Payload{UserID: 2, Friends: []int{1}}
	// start broadcaster
	go srv.broadcaster()
	// simulate a user coming online
	userConn := &simpleConn{}
	srv.aConns[userConn] = model.Payload{UserID: 1, Friends: []int{2}}
	srv.iConns <- userConn
	// give goroutine time to process
	time.Sleep(10 * time.Millisecond)
	expectedOnline := "{\"user_id\":1,\"online\":true}\n"
	if got := string(friend.bufLocked()); got != expectedOnline {
		t.Fatalf("expected online notification %q, got %q", expectedOnline, got)
	}
	// reset buffer and simulate offline
	friend.mu.Lock()
	friend.buf = nil
	friend.mu.Unlock()
	srv.dConns <- userConn
	time.Sleep(10 * time.Millisecond)
	expectedOffline := "{\"user_id\":1,\"online\":false}\n"
	if got := string(friend.bufLocked()); got != expectedOffline {
		t.Fatalf("expected offline notification %q, got %q", expectedOffline, got)
	}
}
