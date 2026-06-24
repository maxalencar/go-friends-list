package protocol

import (
	"go-friends-list/internal/server"
	"testing"
)

func TestNewServer_TCP(t *testing.T) {
	s, err := NewServer("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatalf("expected non-nil server")
	}
	// ensure it implements the Server interface from the main package
	var _ server.Server = s
}

func TestNewServer_UDP(t *testing.T) {
	s, err := NewServer("udp", "localhost:0")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatalf("expected non-nil server")
	}
	var _ server.Server = s
}

func TestNewServer_Invalid(t *testing.T) {
	_, err := NewServer("foobar", "localhost:0")
	if err == nil {
		t.Fatalf("expected error for invalid protocol")
	}
}
