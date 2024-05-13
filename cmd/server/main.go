package main

import (
	"flag"
	"fmt"
	"log"

	"go-friends-list/internal/server"
)

func main() {
	var port int
	var protocol string

	flag.IntVar(&port, "port", 8080, "Port.")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol used, currently supporting tcp and udp.")
	flag.Parse()

	server, err := server.NewServer(protocol, fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalln(err)
	}

	if err = server.Run(); err != nil {
		log.Fatalln(err)
	}
}
