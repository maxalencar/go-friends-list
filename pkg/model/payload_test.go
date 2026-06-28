package model

import "testing"

func TestMarkRead(t *testing.T) {
    msg := ChatMessage{ID: "msg-3", Status: Sent}
    msg.MarkRead(1234567890)
    if msg.Status != Read {
        t.Fatalf("expected status %s, got %s", Read, msg.Status)
    }
    if msg.ReadAt != 1234567890 {
        t.Fatalf("expected ReadAt 1234567890, got %d", msg.ReadAt)
    }
}
