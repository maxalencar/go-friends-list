package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"go-friends-list/pkg/model"
)

var (
	aConns  = make(map[net.Conn]model.Payload)
	iConns  = make(chan net.Conn)
	dConns  = make(chan net.Conn)
	payload model.Payload
)

// NewTCPServer creates a new tcp Server using given protocol
// and addr.
func NewTCPServer(addr string) (Server, error) {
	return &TCPServer{
		addr: addr,
	}, nil
}

// TCPServer holds the structure of our TCP implementation.
type TCPServer struct {
	addr   string
	server net.Listener
}

// Run starts the TCP Server.
func (t *TCPServer) Run() (err error) {
	t.server, err = net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}
	defer t.Close()

	go t.broadcaster()

	for {
		conn, err := t.server.Accept()
		if err != nil {
			log.Printf("could not accept connection: %v", err)
			continue
		}

		if conn == nil {
			log.Printf("no Connection")
			continue
		}

		go t.handleConn(conn)
	}
}

// Close shuts down the TCP Server
func (t *TCPServer) Close() (err error) {
	return t.server.Close()
}

// broadcaster - it broadcasts messages based on the selected channel
func (t *TCPServer) broadcaster() {
	for {
		select {
		case iConn := <-iConns:
			log.Printf("user %d is online\n", aConns[iConn].UserID)

			t.notifyFriends(aConns[iConn], true)
		case dConn := <-dConns:
			log.Printf("user %d is offline\n", aConns[dConn].UserID)

			t.notifyFriends(aConns[dConn], false)
			delete(aConns, dConn)
		}
	}
}

// handleConn - it decodes the payload and add the incoming connection into the active connections map
func (t *TCPServer) handleConn(conn net.Conn) {
	// we create a decoder that reads directly from the socket
	d := json.NewDecoder(conn)

	if err := d.Decode(&payload); err != nil {
		fmt.Printf("error decoding: %v", err)
	}

	// add the new active connection to the map of active connections with the payload sent to identify the user and his friends
	aConns[conn] = payload
	iConns <- conn

	for {
		_, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			break
		}
	}

	// it closes their connection.
	dConns <- conn
}

// notifyFriends - it notifies the user friends his status
func (t *TCPServer) notifyFriends(payload model.Payload, isOnline bool) {
	for _, f := range payload.Friends {
		for k, v := range aConns {
			if f == v.UserID {
				log.Printf("notifying user %d", v.UserID)

				if _, err := k.Write([]byte(fmt.Sprintf("{\"user_id\": %d, \"online\": %t}\n", payload.UserID, isOnline))); err != nil {
					log.Printf("error on writing to connection; err %v", err)
					continue
				}
			}
		}
	}
}
