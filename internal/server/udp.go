package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"go-friends-list/pkg/model"
)

type UDPServer struct {
    heartbeatInterval time.Duration
    // lastSeq tracks the last sequence number seen from each client (by UserID)
    lastSeq map[int]int

	addr          string
	server        net.PacketConn
	aAddresses    map[string]model.Payload
	iAddresses    chan string
	dAddresses    chan string
	hbaAddresses  map[string]bool
	udpConnMu    sync.RWMutex
}

func NewUDPServer(addr string) (Server, error) {
	return &UDPServer{
        addr:               addr,
        aAddresses:         make(map[string]model.Payload),
        iAddresses:         make(chan string),
        dAddresses:         make(chan string),
        hbaAddresses:      make(map[string]bool),
        heartbeatInterval: 5 * time.Second,
        lastSeq:            make(map[int]int),
    }, nil
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
		case iAddress := <-u.iAddresses:
			u.udpConnMu.RLock()
			payload := u.aAddresses[iAddress]
			u.udpConnMu.RUnlock()
			log.Printf("user %d is online\n", payload.UserID)

			u.notifyFriends(payload, true)
		case dAddress := <-u.dAddresses:
			u.udpConnMu.RLock()
			payload := u.aAddresses[dAddress]
			u.udpConnMu.RUnlock()
			log.Printf("user %d is offline\n", payload.UserID)

			u.notifyFriends(payload, false)
			u.udpConnMu.Lock()
			delete(u.aAddresses, dAddress)
			delete(u.hbaAddresses, dAddress)
			u.udpConnMu.Unlock()
		}
	}
}

// handleConn - it decodes the payload and add the incoming connection into the active connections map
func (u *UDPServer) handleConn(addr net.Addr, cmd []byte) {
    addrString := addr.String()

    // Heartbeat handling
    if string(cmd) == "beat" {
        u.udpConnMu.Lock()
        u.hbaAddresses[addrString] = true
        u.udpConnMu.Unlock()
        return
    }

    // Try to unmarshal as a ChatMessage first (reliable UDP payload)
    var chatMsg model.ChatMessage
    if err := json.Unmarshal(cmd, &chatMsg); err == nil && chatMsg.ID != "" {
        // Duplicate detection using sequence numbers
        senderID := chatMsg.From
        seq := chatMsg.Seq
        u.udpConnMu.Lock()
        last, ok := u.lastSeq[senderID]
        if ok && seq <= last {
            // Duplicate – still send ACK
            u.udpConnMu.Unlock()
            u.sendAck(addr, chatMsg)
            return
        }
        // Record newest sequence
        u.lastSeq[senderID] = seq
        u.udpConnMu.Unlock()

        // TODO: broadcast chat message to friends (not implemented yet)
        // For now, just ACK the sender to satisfy retransmission logic.
        u.sendAck(addr, chatMsg)
        return
    }

    // Fallback: treat as initial Payload registration
    var pl model.Payload
    if err := json.Unmarshal(cmd, &pl); err != nil {
        log.Printf("error unmarshaling payload %s: %v", string(cmd), err)
        return
    }
    u.udpConnMu.Lock()
    u.aAddresses[addrString] = pl
    u.udpConnMu.Unlock()
    u.iAddresses <- addrString
}

// sendAck sends an acknowledgment for a received ChatMessage.
func (u *UDPServer) sendAck(addr net.Addr, msg model.ChatMessage) {
    ack := model.ChatMessage{
        ID:  msg.ID,
        From: msg.From,
        To:   msg.To,
        Seq:  msg.Seq,
        Content: "",
        Status:  "",
    }
    data, err := json.Marshal(ack)
    if err != nil {
        log.Printf("failed to marshal ACK: %v", err)
        return
    }
    // Append newline to match client read logic
    data = append(data, '\n')
    if _, err := u.server.WriteTo(data, addr); err != nil {
        log.Printf("error sending ACK to %s: %v", addr.String(), err)
    }
}

// notifyFriends - it notifies the user friends his status
func (u *UDPServer) notifyFriends(payload model.Payload, isOnline bool) {
	for _, f := range payload.Friends {
		for k, v := range u.aAddresses {
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
		time.Sleep(u.heartbeatInterval)
		log.Printf("Checking heartbeat for active client(s): %d", len(u.hbaAddresses))

		for k, v := range u.hbaAddresses {
			if !v {
				u.dAddresses <- k
			} else {
				u.hbaAddresses[k] = false
			}
		}

	}
}
