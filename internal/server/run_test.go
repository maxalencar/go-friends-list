package server

import (
    "testing"
    "time"
)

func TestNewTCPServerAndRun(t *testing.T) {
    srvI, err := NewTCPServer("127.0.0.1:0")
    if err != nil {
        t.Fatalf("NewTCPServer error: %v", err)
    }
    srv, ok := srvI.(*TCPServer)
    if !ok {
        t.Fatalf("expected *TCPServer type")
    }
    if srv == nil {
        t.Fatalf("server not properly initialised")
    }
    // Run server in goroutine and stop after short delay
    done := make(chan error, 1)
    go func() {
        done <- srv.Run()
    }()
    // Give it a moment to start listening
    time.Sleep(20 * time.Millisecond)
    // Close server; this should cause Run to exit with nil
    if err := srv.Close(); err != nil {
        t.Fatalf("Close error: %v", err)
    }
    select {
    case err := <-done:
        if err != nil {
            t.Fatalf("Run returned error: %v", err)
        }
    case <-time.After(500 * time.Millisecond):
        t.Fatalf("Run did not return after Close")
    }
}
