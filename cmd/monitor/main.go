package main

import (
    "context"
    "flag"
    "log"
    "net/http"
    "os/signal"
    "strconv"
    "syscall"
)

func main() {
    var port int
    flag.IntVar(&port, "port", 9090, "port for health check server")
    flag.Parse()

    http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

    addr := ":" + strconv.Itoa(port)
    log.Printf("starting health check server on %s", addr)
    server := &http.Server{Addr: addr}

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("health check server error: %v", err)
        }
    }()

    <-ctx.Done()
    log.Println("shutdown signal received, closing monitor...")
    if err := server.Shutdown(context.Background()); err != nil {
        log.Printf("monitor shutdown error: %v", err)
    }
}
