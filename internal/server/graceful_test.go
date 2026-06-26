// importers: []
// callers: []
// affected_api: []
// data_schemas: []
// verbatim_instruction: Add a test `internal/server/graceful_test.go` to verify `Close()` stops the server without error.

package server_test

import (
    "net/http"
    "testing"
    "time"

    "go-friends-list/internal/server/protocol"
)

func TestGracefulShutdown(t *testing.T) {
    // Create a TCP server listening on a random port
    srv, err := protocol.NewServer("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("failed to create server: %v", err)
    }

    // Run the server in a separate goroutine
    runCh := make(chan error, 1)
    go func() {
        runCh <- srv.Run()
    }()

    // Give the server a moment to start
    time.Sleep(100 * time.Millisecond)

    // Initiate graceful shutdown
    if err := srv.Close(); err != nil {
        t.Fatalf("failed to close server: %v", err)
    }

    // Wait for Run to return
    select {
    case err := <-runCh:
        if err != nil && err != http.ErrServerClosed {
            t.Fatalf("server run returned unexpected error: %v", err)
        }
    case <-time.After(2 * time.Second):
        t.Fatalf("server did not shut down within timeout")
    }
}
