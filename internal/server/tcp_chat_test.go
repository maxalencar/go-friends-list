package server

import (
    "bufio"
    "encoding/json"
    "net"
    "testing"
    "time"

    "go-friends-list/pkg/model"
)

// TestChatMessageRouting verifies that a chat message sent by one user is delivered to the intended recipient.
func TestChatMessageRouting(t *testing.T) {
    // Initialize a TCPServer with necessary maps and channels.
    srv := &TCPServer{
        aConns:          make(map[net.Conn]model.Payload),
        iConns:          make(chan net.Conn, 10),
        dConns:          make(chan net.Conn, 1),
        storedMessages:  make(map[string]model.ChatMessage),
        lastSeq:         make(map[int]int),
    }

    // Create pipe connections for two users.
    aConn, aPeer := net.Pipe()
    bConn, bPeer := net.Pipe()
    _ = aConn.Close()
    _ = bConn.Close()
    _ = aPeer.Close()
    _ = bPeer.Close()

    // Register connections in the server's map.
    srv.aConns[aConn] = model.Payload{UserID: 1, Friends: []int{2}}
    srv.aConns[bConn] = model.Payload{UserID: 2, Friends: []int{1}}

    // Start server handlers for each connection.
    go srv.handleConn(aConn)
    go srv.handleConn(bConn)

    // Give the goroutines a moment to start.
    time.Sleep(10 * time.Millisecond)

    // Send initial payloads (registration) for both users.
    payloadA := model.Payload{UserID: 1, Friends: []int{2}}
    payloadB := model.Payload{UserID: 2, Friends: []int{1}}
    // Encode payloads directly to the peer ends (the server reads from the other side).
    encA := json.NewEncoder(aPeer)
    encB := json.NewEncoder(bPeer)
    if err := encA.Encode(payloadA); err != nil {
        t.Fatalf("failed to send payload A: %v", err)
    }
    if err := encB.Encode(payloadB); err != nil {
        t.Fatalf("failed to send payload B: %v", err)
    }

    // Give server time to process registration.
    time.Sleep(10 * time.Millisecond)

    // Send a chat message from user 1 to user 2.
    chat := model.ChatMessage{From: 1, To: 2, Content: "hello", ID: "msg-xyz", Timestamp: time.Now().Unix()}
    chatBytes, _ := json.Marshal(chat)
    if _, err := aPeer.Write(append(chatBytes, '\n')); err != nil {
        t.Fatalf("failed to write chat message: %v", err)
    }

    // Read from user 2's connection and skip non‑chat messages (e.g., online notifications).
    r := bufio.NewReader(bPeer)
    var received model.ChatMessage
    for {
        line, err := r.ReadString('\n')
        if err != nil {
            t.Fatalf("failed to read from recipient: %v", err)
        }
        if err := json.Unmarshal([]byte(line), &received); err == nil && received.To != 0 {
            // Got a chat message.
            break
        }
        // ignore other messages
    }
    if received.From != chat.From || received.To != chat.To || received.Content != chat.Content {
        t.Fatalf("chat message mismatch: got %+v want %+v", received, chat)
    }
    // Close peers to allow handleConn goroutines to exit.
    _ = aPeer.Close()
    _ = bPeer.Close()
}
