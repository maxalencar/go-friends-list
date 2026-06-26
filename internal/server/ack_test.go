package server

import (
    "bytes"
    "net"
    "testing"
    "time"
    "go-friends-list/pkg/model"
)

// mockConn implements net.Conn for in‑memory testing of ACK functions.
type mockConn struct {
    buf bytes.Buffer
}

func (m *mockConn) Read(b []byte) (n int, err error)  { return m.buf.Read(b) }
func (m *mockConn) Write(b []byte) (n int, err error) { return m.buf.Write(b) }
func (m *mockConn) Close() error                     { return nil }
func (m *mockConn) LocalAddr() net.Addr              { return nil }
func (m *mockConn) RemoteAddr() net.Addr             { return nil }
func (m *mockConn) SetDeadline(t time.Time) error   { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestSendAndWaitForAck(t *testing.T) {
    mc := &mockConn{}
    ack := AckMessage{ID: "msg-1", From: 2, To: 1, Seq: 10, Status: model.Delivered}
    if err := sendAck(mc, ack); err != nil {
        t.Fatalf("sendAck failed: %v", err)
    }
    // Now wait for same ack
    if err := waitForAck(mc, "msg-1", 100*time.Millisecond); err != nil {
        t.Fatalf("waitForAck did not receive expected ack: %v", err)
    }
}
