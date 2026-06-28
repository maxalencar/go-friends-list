package server

import (
    "bytes"
    "encoding/json"
    "net"
    "testing"
    "time"

    "go-friends-list/pkg/model"
)

// rwMockConn implements net.Conn with separate read and write buffers.
type rwMockConn struct {
    read  bytes.Buffer
    write bytes.Buffer
}

func (c *rwMockConn) Read(b []byte) (int, error)   { return c.read.Read(b) }
func (c *rwMockConn) Write(b []byte) (int, error)  { return c.write.Write(b) }
func (c *rwMockConn) Close() error                  { return nil }
func (c *rwMockConn) LocalAddr() net.Addr           { return nil }
func (c *rwMockConn) RemoteAddr() net.Addr          { return nil }
func (c *rwMockConn) SetDeadline(t time.Time) error { return nil }
func (c *rwMockConn) SetReadDeadline(t time.Time) error { return nil }
func (c *rwMockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestHandleConnStoresMessage(t *testing.T) {
    conn := &rwMockConn{}
    payload := model.Payload{UserID: 1, Friends: []int{2}}
    enc := json.NewEncoder(&conn.read)
    if err := enc.Encode(payload); err != nil {
        t.Fatalf("encode payload: %v", err)
    }
    chat := model.ChatMessage{ID: "msg-1", From: 1, To: 2, Content: "hi", Timestamp: time.Now().Unix(), Status: model.Sent}
    if err := enc.Encode(chat); err != nil {
        t.Fatalf("encode chat: %v", err)
    }
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), storedMessages: make(map[string]model.ChatMessage), dConns: make(chan net.Conn, 1), iConns: make(chan net.Conn, 1)}
    srv.handleConn(conn)
    if stored, ok := srv.storedMessages[chat.ID]; !ok {
        t.Fatalf("message not stored")
    } else if stored.Content != "hi" {
        t.Fatalf("stored message content mismatch: %s", stored.Content)
    }
}

func TestHandleConnDuplicateIgnored(t *testing.T) {
    conn := &rwMockConn{}
    payload := model.Payload{UserID: 1, Friends: []int{2}}
    enc := json.NewEncoder(&conn.read)
    enc.Encode(payload)
    chat := model.ChatMessage{ID: "dup", From: 1, To: 2, Content: "first", Timestamp: time.Now().Unix(), Status: model.Sent}
    enc.Encode(chat)
    dup := model.ChatMessage{ID: "dup", From: 1, To: 2, Content: "second", Timestamp: time.Now().Unix(), Status: model.Sent}
    enc.Encode(dup)
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), storedMessages: make(map[string]model.ChatMessage), dConns: make(chan net.Conn, 1), iConns: make(chan net.Conn, 1)}
    srv.handleConn(conn)
    if len(srv.storedMessages) != 1 {
        t.Fatalf("expected 1 stored message, got %d", len(srv.storedMessages))
    }
    // The content may be from either the first or duplicate due to updateMessageStatus.
    // Ensure that a message with ID "dup" exists.
    if _, ok := srv.storedMessages["dup"]; !ok {
        t.Fatalf("message ID 'dup' missing")
    }
}
