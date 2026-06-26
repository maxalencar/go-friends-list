package main

import (
    "flag"
    "log"
    "net/http"
    "strconv"
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
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatalf("health check server failed: %v", err)
    }
}
