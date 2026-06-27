package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"go-friends-list/pkg/model"
)

// failConn wraps a connection whose Write always returns an error.
type failConn struct {
	net.Conn
}

func (f *failConn) Write(b []byte) (int, error)     { return 0, &net.OpError{Err: net.UnknownNetworkError("fail")} }
func (f *failConn) Read(b []byte) (int, error)       { return 0, net.ErrClosed }
func (f *failConn) Close() error                      { return nil }
func (f *failConn) LocalAddr() net.Addr               { return nil }
func (f *failConn) RemoteAddr() net.Addr              { return nil }
func (f *failConn) SetDeadline(t time.Time) error     { return nil }
func (f *failConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *failConn) SetWriteDeadline(t time.Time) error { return nil }

// safeTestConn is a thread-safe in-memory net.Conn for concurrent write.
type safeTestConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *safeTestConn) Read(b []byte) (int, error)              { return 0, net.ErrClosed }
func (c *safeTestConn) Write(b []byte) (int, error)             { c.mu.Lock(); defer c.mu.Unlock(); return c.buf.Write(b) }
func (c *safeTestConn) Close() error                             { return nil }
func (c *safeTestConn) LocalAddr() net.Addr                      { return nil }
func (c *safeTestConn) RemoteAddr() net.Addr                    { return nil }
func (c *safeTestConn) SetDeadline(t time.Time) error           { return nil }
func (c *safeTestConn) SetReadDeadline(t time.Time) error       { return nil }
func (c *safeTestConn) SetWriteDeadline(t time.Time) error      { return nil }

// TestHandleConnValidationFailure covers the validate.Struct error path.
func TestHandleConnValidationFailure(t *testing.T) {
	conn := &rwMockConn{}
	payload := model.Payload{UserID: 0, Friends: []int{2}} // UserID=0 fails required
	enc := json.NewEncoder(&conn.read)
	enc.Encode(payload)

	srv := &TCPServer{
		aConns:         make(map[net.Conn]model.Payload),
		storedMessages: make(map[string]model.ChatMessage),
		dConns:         make(chan net.Conn, 1),
		iConns:         make(chan net.Conn, 1),
	}
	srv.handleConn(conn)
	srv.connMu.RLock()
	_, ok := srv.aConns[conn]
	srv.connMu.RUnlock()
	if ok {
		t.Fatal("expected conn not registered after validation failure")
	}
}

// TestHandleConnDecodeError covers the json.Decode error path.
func TestHandleConnDecodeError(t *testing.T) {
	conn := &rwMockConn{}
	conn.read.WriteString("not-json\n")

	srv := &TCPServer{
		aConns:         make(map[net.Conn]model.Payload),
		storedMessages: make(map[string]model.ChatMessage),
		dConns:         make(chan net.Conn, 1),
		iConns:         make(chan net.Conn, 1),
	}
	srv.handleConn(conn)
	srv.connMu.RLock()
	_, ok := srv.aConns[conn]
	srv.connMu.RUnlock()
	if ok {
		t.Fatal("expected conn not registered after decode failure")
	}
}

// TestHandleConnGroupMessage covers the IsGroupMessage branch.
func TestHandleConnGroupMessage(t *testing.T) {
	recipient := &safeTestConn{}
	sender := &rwMockConn{}

	enc := json.NewEncoder(&sender.read)
	enc.Encode(model.Payload{UserID: 1, Friends: []int{2}})
	groupMsg := model.ChatMessage{ID: "g1", From: 1, GroupID: 10, Content: "hi group", Timestamp: time.Now().Unix(), Status: model.Sent}
	enc.Encode(groupMsg)

	srv := &TCPServer{
		aConns:         make(map[net.Conn]model.Payload),
		storedMessages: make(map[string]model.ChatMessage),
		dConns:         make(chan net.Conn, 1),
		iConns:         make(chan net.Conn, 1),
	}
	srv.aConns[recipient] = model.Payload{UserID: 2, Friends: []int{1}}

	srv.handleConn(sender)

	if _, ok := srv.storedMessages["g1"]; !ok {
		t.Fatal("group message not stored")
	}
}

// TestHandleConnRecipientOnline covers the isConnected path after send.
func TestHandleConnRecipientOnline(t *testing.T) {
	recipient := &safeTestConn{}
	sender := &rwMockConn{}

	enc := json.NewEncoder(&sender.read)
	enc.Encode(model.Payload{UserID: 1, Friends: []int{2}})
	chat := model.ChatMessage{ID: "dm1", From: 1, To: 2, Content: "hi", Timestamp: time.Now().Unix(), Status: model.Sent}
	enc.Encode(chat)

	srv := &TCPServer{
		aConns:         make(map[net.Conn]model.Payload),
		storedMessages: make(map[string]model.ChatMessage),
		dConns:         make(chan net.Conn, 1),
		iConns:         make(chan net.Conn, 1),
	}
	srv.aConns[recipient] = model.Payload{UserID: 2, Friends: []int{1}}

	srv.handleConn(sender)

	stored := srv.storedMessages["dm1"]
	if stored.Status != model.Delivered {
		t.Fatalf("expected Delivered for online recipient, got %s", stored.Status)
	}
}

// TestRunAcceptsConnection covers the Accept loop and nil-conn guard in Run.
func TestRunAcceptsConnection(t *testing.T) {
	srvI, err := NewTCPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewTCPServer: %v", err)
	}
	srv := srvI.(*TCPServer)

	done := make(chan error, 1)
	go func() { done <- srv.Run() }()

	// Give server time to enter Accept()
	time.Sleep(20 * time.Millisecond)

	// 1) Dial a valid client, send payload, then close – hits registration path
	go func() {
		srv.serverMu.Lock()
		addr := srv.server.Addr().String()
		srv.serverMu.Unlock()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		pl := model.Payload{UserID: 7, Friends: []int{8}}
		data, _ := json.Marshal(pl)
		conn.Write(append(data, '\n'))
		time.Sleep(10 * time.Millisecond)
		conn.Close()
	}()

	// 2) Trigger the nil-conn path: close listener so Accept returns (nil, err)
	srv.serverMu.Lock()
	listener := srv.server
	srv.serverMu.Unlock()
	listener.Close()

	time.Sleep(20 * time.Millisecond) // let goroutines drain
	srv.Close() // close any remaining resources

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return after listener close")
	}
}

// TestSendAckWriteError covers the write error branch.
func TestSendAckWriteError(t *testing.T) {
	conn := &failConn{}
	ack := AckMessage{ID: "x", From: 1, To: 2, Seq: 1, Status: model.Sent}
	err := sendAck(conn, ack)
	if err == nil {
		t.Fatal("expected error from sendAck with failing writer")
	}
}

// TestWaitForAckTimeout covers the timeout branch.
func TestWaitForAckTimeout(t *testing.T) {
	conn := &rwMockConn{} // empty read buffer
	err := waitForAck(conn, "missing-msg", 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error from waitForAck")
	}
}

// TestRetryBackoffFactor covers the exponential backoff path.
func TestRetryBackoffFactor(t *testing.T) {
	attempts := 0
	fn := func() error { attempts++; return net.ErrClosed }
	start := time.Now()
	err := Retry(context.Background(), fn, RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, BackoffFactor: 2.0})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	elapsed := time.Since(start)
	if elapsed < 25*time.Millisecond {
		t.Fatalf("expected backoff delay, got %v", elapsed)
	}
}

// TestRetryZeroMaxAttempts covers the MaxAttempts <= 0 guard.
func TestRetryZeroMaxAttempts(t *testing.T) {
	attempts := 0
	fn := func() error { attempts++; return nil }
	err := Retry(context.Background(), fn, RetryPolicy{MaxAttempts: 0, BaseDelay: 1 * time.Millisecond})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (clamped from 0), got %d", attempts)
	}
}

// TestNotifyFriendsWriteError covers the log branch when Write fails.
func TestNotifyFriendsWriteError(t *testing.T) {
	srv := &TCPServer{aConns: make(map[net.Conn]model.Payload)}
	bad := &failConn{}
	srv.aConns[bad] = model.Payload{UserID: 2, Friends: []int{1}}
	// Should not panic — just logs the error
	srv.notifyFriends(model.Payload{UserID: 1, Friends: []int{2}}, true)
}
