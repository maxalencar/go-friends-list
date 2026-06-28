package server

import (
    "fmt"
    "encoding/json"
    "log"
    "net"
    "time"

    "go-friends-list/pkg/model"
)

// AckMessage represents an acknowledgment (delivery or read receipt)
// The same struct is used for both TCP and UDP transports.
type AckMessage struct {
    ID     string               `json:"id"`
    From   int                  `json:"from"`
    To     int                  `json:"to"`
    Seq    int                  `json:"seq"`
    Status model.MessageStatus `json:"status"`
}

// sendAck writes an AckMessage on the given connection.
func sendAck(conn net.Conn, ack AckMessage) error {
    data, err := json.Marshal(ack)
    if err != nil {
        return err
    }
    _, err = conn.Write(append(data, '\n'))
    if err != nil {
        log.Printf("failed to send ACK for message %s: %v", ack.ID, err)
    }
    return err
}

// waitForAck blocks until an ACK for the given message ID is received or the timeout expires.
func waitForAck(conn net.Conn, msgID string, timeout time.Duration) error {
    dec := json.NewDecoder(conn)
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        var ack AckMessage
        if err := dec.Decode(&ack); err != nil {
            // If we can't decode yet, just continue waiting.
            continue
        }
        if ack.ID == msgID {
            return nil // ACK received
        }
    }
    return fmt.Errorf("timeout waiting for ACK for message %s", msgID)
}

// processAck updates the stored message status based on an incoming AckMessage.
// The caller is responsible for locating the appropriate server instance and its storedMessages map.
