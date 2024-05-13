package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"go-friends-list/pkg/model"
)

var (
	aAddresses   = make(map[string]model.Payload)
	iAddresses   = make(chan string)
	dAddresses   = make(chan string)
	hbaAddresses = make(map[string]bool)
)

// NewUDPServer creates a new udp Server using given protocol
// and addr.
func NewUDPServer(addr string) (Server, error) {
	return &UDPServer{
		addr: addr,
	}, nil
}

// UDPServer holds the structure of our UDP implementation.
type UDPServer struct {
	addr   string
	server net.PacketConn
}

// Run starts the TCP Server.
func (u *UDPServer) Run() (err error) {
	u.server, err = net.ListenPacket("udp", u.addr)
	if err != nil {
		return errors.New("could not listen on UDP")
	}
	defer u.Close()

	go u.heartbeatCheck()
	go u.broadcaster()

	for {
		buf := make([]byte, 2048)
		n, conn, err := u.server.ReadFrom(buf)
		if err != nil {
			log.Printf("could not read packets from the connection: %v", err)
			continue
		}
		if conn == nil {
			log.Printf("no Connection")
			continue
		}

		go u.handleConn(conn, buf[:n])
	}
}

// Close shuts down the TCP Server
func (u *UDPServer) Close() (err error) {
	return u.server.Close()
}

// broadcaster - it broadcasts messages based on the selected channel
func (u *UDPServer) broadcaster() {
	for {
		select {
		case iAddress := <-iAddresses:
			log.Printf("user %d is online\n", aAddresses[iAddress].UserID)

			u.notifyFriends(aAddresses[iAddress], true)
		case dAddress := <-dAddresses:
			log.Printf("user %d is offline\n", aAddresses[dAddress].UserID)

			u.notifyFriends(aAddresses[dAddress], false)
			delete(aAddresses, dAddress)
			delete(hbaAddresses, dAddress)
		}
	}
}

// handleConn - it decodes the payload and add the incoming connection into the active connections map
func (u *UDPServer) handleConn(addr net.Addr, cmd []byte) {
	addrString := addr.String()

	// if the message sent is beat, it
	if string(cmd) == "beat" {
		hbaAddresses[addrString] = true
	} else {
		err := json.Unmarshal([]byte(cmd), &payload)
		if err != nil {
			log.Printf("error unmarshaling object %s: err: %v", string(cmd), err)
		}

		// add the new active connection to the map of active connections with the payload sent to identify the user and his friends
		aAddresses[addrString] = payload
		iAddresses <- addrString
	}
}

// notifyFriends - it notifies the user friends his status
func (u *UDPServer) notifyFriends(payload model.Payload, isOnline bool) {
	for _, f := range payload.Friends {
		for k, v := range aAddresses {
			if f == v.UserID {
				log.Printf("notifying user %d", v.UserID)

				laddr, err := net.ResolveUDPAddr("udp", k)
				if err != nil {
					log.Println(err)
					continue
				}

				_, err = u.server.WriteTo([]byte(fmt.Sprintf("{\"user_id\": %d, \"online\": %t}\n", payload.UserID, isOnline)), laddr)
				if err != nil {
					log.Printf("error on writing to connection; err %v", err)
					continue
				}
			}
		}
	}
}

// heartbeatCheck - it checks for active connections, if a user hasn't send any message for at least 5 sec we consider they are disconnected
func (u *UDPServer) heartbeatCheck() {
	for {
		time.Sleep(time.Duration(5) * time.Second)
		log.Printf("Checking heartbeat for active client(s): %d", len(hbaAddresses))

		for k, v := range hbaAddresses {
			if !v {
				dAddresses <- k
			} else {
				hbaAddresses[k] = false
			}
		}

	}
}
