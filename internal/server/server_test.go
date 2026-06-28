package server

import (
    "bytes"
    "encoding/json"
    "net"
    "testing"
    "time"

    "go-friends-list/pkg/model"
)

// testConn implements net.Conn with an in‑memory buffer.
type testConn struct {
    bytes.Buffer
}

func (m *testConn) Read(b []byte) (int, error)               { return m.Buffer.Read(b) }
func (m *testConn) Write(b []byte) (int, error)              { return m.Buffer.Write(b) }
func (m *testConn) Close() error                              { return nil }
func (m *testConn) LocalAddr() net.Addr                       { return nil }
func (m *testConn) RemoteAddr() net.Addr                      { return nil }
func (m *testConn) SetDeadline(t time.Time) error            { return nil }
func (m *testConn) SetReadDeadline(t time.Time) error        { return nil }
func (m *testConn) SetWriteDeadline(t time.Time) error       { return nil }

func TestIsConnected(t *testing.T) {
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload)}
    conn := &testConn{}
    srv.aConns[conn] = model.Payload{UserID: 42, Friends: []int{1, 2}}
    if !srv.isConnected(42) {
        t.Fatalf("expected user 42 to be connected")
    }
    if srv.isConnected(99) {
        t.Fatalf("expected user 99 to be not connected")
    }
}

func TestNotifyFriendsBroadcast(t *testing.T) {
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload)}
    friend := &testConn{}
    srv.aConns[friend] = model.Payload{UserID: 2, Friends: []int{1}}
    payload := model.Payload{UserID: 1, Friends: []int{2}}
    srv.notifyFriends(payload, true)
    expected := "{\"user_id\":1,\"online\":true}\n"
    if friend.String() != expected {
        t.Fatalf("unexpected broadcast: got %q want %q", friend.String(), expected)
    }
    // offline notification
    friend.Reset()
    srv.notifyFriends(payload, false)
    expected = "{\"user_id\":1,\"online\":false}\n"
    if friend.String() != expected {
        t.Fatalf("unexpected offline broadcast: got %q want %q", friend.String(), expected)
    }
}

func TestSendToUserWithStatus(t *testing.T) {
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), storedMessages: make(map[string]model.ChatMessage)}
    user := &testConn{}
    srv.aConns[user] = model.Payload{UserID: 2, Friends: []int{1}}
    msg := model.ChatMessage{ID: "msg-123", From: 1, To: 2, Content: "hello", Status: model.Sent}
    srv.sendToUserWithStatus(2, msg, model.Delivered)
    var received model.ChatMessage
    if err := json.Unmarshal(user.Bytes(), &received); err != nil {
        t.Fatalf("invalid JSON: %v", err)
    }
    if received.ID != msg.ID || received.Content != msg.Content {
        t.Fatalf("message mismatch: %+v", received)
    }
    if _, ok := srv.storedMessages[msg.ID]; !ok {
        t.Fatalf("storedMessages not updated")
    }
}

func TestBroadcastToGroupWithStatus(t *testing.T) {
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), storedMessages: make(map[string]model.ChatMessage)}
    connA := &testConn{}
    connB := &testConn{}
    srv.aConns[connA] = model.Payload{UserID: 2, Friends: []int{1}}
    srv.aConns[connB] = model.Payload{UserID: 3, Friends: []int{1}}
    msg := model.ChatMessage{ID: "msg-g", From: 1, GroupID: 99, Content: "groupmsg", Status: model.Sent}
    srv.broadcastToGroupWithStatus(99, msg, model.Delivered)
    var mA, mB model.ChatMessage
    if err := json.Unmarshal(connA.Bytes(), &mA); err != nil { t.Fatalf("json error A: %v", err) }
    if err := json.Unmarshal(connB.Bytes(), &mB); err != nil { t.Fatalf("json error B: %v", err) }
    if mA.ID != msg.ID || mB.ID != msg.ID {
        t.Fatalf("broadcast IDs mismatch: %v %v", mA.ID, mB.ID)
    }
    if _, ok := srv.storedMessages[msg.ID]; !ok {
        t.Fatalf("storedMessages not updated after broadcast")
    }
}

func TestUpdateMessageStatusNotifiesSender(t *testing.T) {
    srv := &TCPServer{aConns: make(map[net.Conn]model.Payload), storedMessages: make(map[string]model.ChatMessage)}
    sender := &testConn{}
    srv.aConns[sender] = model.Payload{UserID: 1, Friends: []int{2}}
    recv := &testConn{}
    srv.aConns[recv] = model.Payload{UserID: 2, Friends: []int{1}}
    msg := model.ChatMessage{ID: "msg-foo", From: 1, To: 2, Status: model.Sent}
    srv.updateMessageStatus(msg)
    var upd model.ChatMessage
    if err := json.Unmarshal(sender.Bytes(), &upd); err != nil { t.Fatalf("json error: %v", err) }
    if upd.ID != msg.ID || upd.Status != msg.Status {
        t.Fatalf("unexpected status update: %+v", upd)
    }
}

func TestNewMessageCreatesUniqueID(t *testing.T) {
    m1 := model.NewMessage(1, 2, "hi")
    m2 := model.NewMessage(1, 2, "hi again")
    if m1.ID == "" || m2.ID == "" {
        t.Fatalf("expected non-empty IDs")
    }
    if m1.ID == m2.ID {
        t.Fatalf("expected unique IDs, got duplicate %s", m1.ID)
    }
}
