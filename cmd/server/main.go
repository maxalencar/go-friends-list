package main

// importers: []
// callers: []
// affected_api: []
// data_schemas: []
// verbatim_instruction: Add OS signal handling to `cmd/server/main.go` so that the server shuts down cleanly on SIGINT/SIGTERM, calling `server.Close()`.

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	srv "go-friends-list/internal/server"
)

func main() {
	// Set up OS signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var port int
	var protocol string
	var portRange string

	flag.IntVar(&port, "port", 8080, "Port.")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol used, currently supporting tcp and udp.")
	flag.StringVar(&portRange, "port-range", "", "Optional port range in the form start-end for fallback if default port is unavailable.")
	flag.Parse()

	// If a port range is provided, attempt to bind to the first available port within the range.
	var svr srv.Server
	var err error
	if portRange != "" {
		var start, end int
		if n, err := fmt.Sscanf(portRange, "%d-%d", &start, &end); n != 2 || err != nil {
			log.Fatalf("invalid port-range format: %s (expected start-end)", portRange)
		}
		bound := false
		for p := start; p <= end; p++ {
			svr, err = srv.NewTCPServer(fmt.Sprintf(":%d", p))
			if err == nil {
				port = p
				bound = true
				break
			}
		}
		if !bound {
			log.Fatalf("could not bind to any port in range %s", portRange)
		}
	} else {
		svr, err = srv.NewTCPServer(fmt.Sprintf(":%d", port))
	}

	if err != nil {
		log.Fatalln(err)
	}

	// Run server in a separate goroutine
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- svr.Run()
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received, closing server...")
	if err := svr.Close(); err != nil {
		log.Printf("Error closing server: %v", err)
	}

	// Wait for server.Run to finish
	if err := <-runErrCh; err != nil && err != http.ErrServerClosed {
		log.Printf("Server exited with error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
