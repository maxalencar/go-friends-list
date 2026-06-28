package server

import "testing"

func TestRunListenerError(t *testing.T) {
    // Use an obviously invalid address to force net.Listen error.
    srv, _ := NewTCPServer("invalid_address")
    if err := srv.Run(); err == nil {
        t.Fatalf("expected Run to return an error for invalid address")
    }
}
