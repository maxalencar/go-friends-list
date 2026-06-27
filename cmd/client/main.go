package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/signal"
    "bufio"
    "syscall"
    "time"

    "go-friends-list/pkg/model"
)

import "sync"

var (
    // Add message status tracking variables
        messageStatuses = make(map[string]model.MessageStatus)
    // Pending ACK tracking for reliable UDP
    pendingAcks   = make(map[string]chan struct{}) // map message ID to ack channel
    pendingMutex  sync.Mutex
    ackTimeout    time.Duration = time.Second // default 1s, can be overridden by flag
    maxRetries     int           = 5          // default, can be overridden by flag
    nextSeq        int           = 1          // monotonic sequence per client
    payload        model.Payload
    conn           net.Conn
    toUser         int // recipient user ID from flag
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    var port int
    var payloadString, protocol string
    flag.IntVar(&port, "port", 8080, "TCP Port.")
    flag.StringVar(&payloadString, "payload", "{\"user_id\":0,\"friends\":[]}", "User Identification.")
    flag.StringVar(&protocol, "protocol", "tcp", "Protocol used, currently supporting tcp and udp.")
    flag.IntVar(&toUser, "to", 0, "Recipient user ID for outgoing chat messages (default 0 means none)")
    flag.Parse()

    conn, err := net.Dial(protocol, fmt.Sprintf(":%d", port))
    if err != nil {
        log.Fatal(err)
    }
    _ = conn.Close()

        err = json.Unmarshal([]byte(payloadString), &payload)
    if err != nil {
        log.Fatalf("Wrong payload sent. it should follow the format: '{\"user_id\": 1, \"friends\": [2,3,4]}'; err: %v", err)
    }

    if payload.UserID == 0 {
        log.Fatalln("invalid User")
    }

    // Send initial payload
    writeMessage(conn, payloadString)

    // Start monitoring for read receipts
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            log.Println("Checking for read receipts...")
        }
    }()

    // Start monitoring incoming messages
    go func() {
        for {
            msg, err := readMessage(conn)
            if err != nil {
                continue
            }
            handleIncomingMessage(msg)
        }
    }()

    // Start user input loop for sending messages
    go func() {
        scanner := bufio.NewScanner(os.Stdin)
        for scanner.Scan() {
            text := scanner.Text()
            if text == "" {
                continue
            }
            sendChatMessage(conn, text)
        }
    }()

    <-ctx.Done()
    log.Println("shutdown signal received, closing client...")
}

func writeMessage(conn net.Conn, msg string) {
    if _, err := conn.Write([]byte(msg + "\n")); err != nil {
        log.Printf("could not write payload to TCP server: %v", err)
    }
}

func sendChatMessage(conn net.Conn, text string) {
    // Create new message with status tracking and sequence number
    msg := model.NewMessage(payload.UserID, toUser, text)
    msg.Seq = nextSeq
    nextSeq++
        messageStatuses[msg.ID] = model.Sent

    data, _ := json.Marshal(msg)
    // Send the message and start ACK handling
    ackCh := make(chan struct{})
    pendingMutex.Lock()
    pendingAcks[msg.ID] = ackCh
    pendingMutex.Unlock()
    if _, err := conn.Write(append(data, '\n')); err != nil {
        log.Printf("error sending message %s: %v", msg.ID, err)
    }

    go func(m model.ChatMessage, ch chan struct{}) {
        retries := 0
        for {
            select {
            case <-ch:
                // ACK received
                return
            case <-time.After(ackTimeout):
                if retries >= maxRetries {
                    log.Printf("❌ Delivery failed for message %s after %d retries", m.ID, retries)
                    pendingMutex.Lock()
                    delete(pendingAcks, m.ID)
                    pendingMutex.Unlock()
                    return
                }
                retries++
                log.Printf("Retransmitting msg %s (attempt %d)", m.ID, retries)
                data, _ := json.Marshal(m)
                if _, err := conn.Write(append(data, '\n')); err != nil {
                    log.Printf("error retransmitting message %s: %v", m.ID, err)
                }
            }
        }
    }( *msg, ackCh)
}

func readMessage(conn net.Conn) (model.ChatMessage, error) {
    // Read and process incoming messages with status updates
    scanner := bufio.NewScanner(conn)
    if !scanner.Scan() {
        if err := scanner.Err(); err != nil {
            return model.ChatMessage{}, err
        }
        return model.ChatMessage{}, io.EOF
    }
    line := scanner.Bytes()
    // Try to unmarshal as ChatMessage first
    var msg model.ChatMessage
    if err := json.Unmarshal(line, &msg); err != nil {
        return model.ChatMessage{}, err
    }
    // Detect ACK messages (empty Content and no Status)
    if msg.Content == "" && msg.Status == "" && msg.Seq != 0 {
        // It's an ACK
        pendingMutex.Lock()
        if ch, ok := pendingAcks[msg.ID]; ok {
            close(ch)
            delete(pendingAcks, msg.ID)
        }
        pendingMutex.Unlock()
        // No further processing needed
        return model.ChatMessage{}, nil
    }
    // Update local status tracking for sent messages
    if msg.Status != model.Sent && msg.ID != "" {
        // Record the latest status for this message ID
        messageStatuses[msg.ID] = msg.Status
    }
    return msg, nil
}

func handleIncomingMessage(msg model.ChatMessage) {
    // Update message status display
    if msg.Status == model.Read {
        fmt.Printf("[READ] %s: %d: %s\n", time.Now().Format("15:04:05"), msg.From, msg.Content)
        return
    }

    statusSymbol := "⏳" // Default: In transit
    switch msg.Status {
    case model.Sent:
        statusSymbol = "⏳" // Sending...
    case model.Delivered:
        statusSymbol = "✓" // Delivered
    case model.Read:
        statusSymbol = "✓" // Read (handled above)
    }

    // Check if message has been read
    if msg.Status == model.Delivered && isCurrentUserRecipient(msg) {
        // Send read receipt
        sendReadReceipt(msg)
    }

    // Display message
    fmt.Printf("[%s] %d: %s\n", statusSymbol, msg.From, msg.Content)
}

func isCurrentUserRecipient(msg model.ChatMessage) bool {
    // Check if current user is the intended recipient of the message
    return msg.To == payload.UserID
}

func sendReadReceipt(msg model.ChatMessage) {
    receipt := model.ChatMessage{
        ID:      msg.ID,
        From:    payload.UserID,
        To:      msg.From,
        Content: "", // Read receipt
        Status:  model.Read,
    }
    data, _ := json.Marshal(receipt)
    writeMessage(conn, string(data))
}

