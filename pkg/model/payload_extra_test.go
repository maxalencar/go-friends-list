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
