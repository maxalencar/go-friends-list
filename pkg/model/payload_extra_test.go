package model

import "testing"

func TestIsGroupMessage(t *testing.T) {
    msg := ChatMessage{GroupID: 5}
    if !msg.IsGroupMessage() {
        t.Fatalf("expected group message detection")
    }
    msg2 := ChatMessage{GroupID: 0}
    if msg2.IsGroupMessage() {
        t.Fatalf("expected non-group message detection")
    }
}

func TestNewMessage(t *testing.T) {
	m := NewMessage(1, 2, "hello")
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if m.From != 1 || m.To != 2 || m.Content != "hello" {
		t.Fatalf("unexpected fields: %+v", m)
	}
	if m.Status != Sent {
		t.Fatalf("expected Sent status, got %s", m.Status)
	}
}

func TestIsBroadcast(t *testing.T) {
    msg := ChatMessage{To: 0}
    if !msg.IsBroadcast() {
        t.Fatalf("expected broadcast detection")
    }
    msg2 := ChatMessage{To: 2}
    if msg2.IsBroadcast() {
        t.Fatalf("expected non-broadcast detection")
    }
}
