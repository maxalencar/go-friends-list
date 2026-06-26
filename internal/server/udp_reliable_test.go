package server

import (
    "bytes"
    "encoding/json"
    "net"
    "testing"
    "time"

    "go-friends-list/pkg/model"
)

type mockConnUDP struct {
    buf bytes.Buffer
}

func (m *mockConnUDP) Read(b []byte) (int, error)  { return m.buf.Read(b) }
func (m *mockConnUDP) Write(b []byte) (int, error) { return m.buf.Write(b) }
func (m *mockConnUDP) Close() error               { return nil }
func (m *mockConnUDP) LocalAddr() net.Addr        { return nil }
func (m *mockConnUDP) RemoteAddr() net.Addr       { return nil }
func (m *mockConnUDP) SetDeadline(t time.Time) error   { return nil }
func (m *mockConnUDP) SetReadDeadline(t time.Time) error { return nil }
func (m *mockConnUDP) SetWriteDeadline(t time.Time) error { return nil }

func TestSendAckAndDecode(t *testing.T) {
    mc := &mockConnUDP{}
    // Prepare an AckMessage using the model-level ChatMessage with empty content for ACK.
    ack := AckMessage{ID: "msg-1", From: 2, To: 1, Seq: 10, Status: model.Delivered}
    // Use the server's sendAck function.
    if err := sendAck(mc, ack); err != nil {
        t.Fatalf("sendAck failed: %v", err)
    }
    // Decode the data back.
    dec := json.NewDecoder(mc)
    var decoded AckMessage
    if err := dec.Decode(&decoded); err != nil {
        t.Fatalf("failed to decode ack: %v", err)
    }
    if decoded.ID != ack.ID || decoded.Status != ack.Status {
        t.Fatalf("decoded ack mismatch: got %+v want %+v", decoded, ack)
    }
}
