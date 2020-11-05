package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go-friends-list/model"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	var port int
	var payloadString, protocol string

	flag.IntVar(&port, "port", 8080, "TCP Port.")
	flag.StringVar(&payloadString, "payload", "{\"user_id\": 0, \"friends\": []}", "User Identification.")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol used, currently supporting tcp and udp.")
	flag.Parse()

	conn, err := net.Dial(protocol, fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var payload model.Payload
	err = json.Unmarshal([]byte(payloadString), &payload)
	if err != nil {
		log.Fatalf("wrong payload sent. it should follow the format: '{\"user_id\": 1, \"friends\": [2,3,4]}'; err: %v", err)
	}

	if payload.UserID == 0 {
		log.Fatalln("invalid User")
	}

	switch strings.ToLower(protocol) {
	case "tcp":
		writeMessage(conn, payloadString)
	case "udp":
		writeUDP(conn, payloadString)
	}

}

func writeUDP(conn net.Conn, msg string) {
	beat := []byte("beat")

	go func() {
		for {
			n, err := conn.Write(beat)
			if err != nil {
				fmt.Printf("UDP write error %v\n", err)
			}
			if n != len(beat) {
				fmt.Printf("Can't write enough (?Server Down?)\n")
			}

			time.Sleep(1 * time.Second)
		}
	}()

	writeMessage(conn, msg)
}

func writeMessage(conn net.Conn, msg string) {
	if _, err := conn.Write([]byte(msg)); err != nil {
		log.Printf("could not write payload to TCP server: %v", err)
	}

	mustCopy(os.Stdout, conn)
}

func mustCopy(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
}
