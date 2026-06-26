package server

import (
    "encoding/json"
    "log"
    "net"
    "strings"
    "sync"
    "time"

    "github.com/google/uuid"

    "go-friends-list/pkg/model"
)

type TCPServer struct {
    addr           string
    server         net.Listener
    storedMessages map[string]model.ChatMessage
    aConns         map[net.Conn]model.Payload
    iConns         chan net.Conn
    dConns         chan net.Conn
    connMu         sync.RWMutex
    // lastSeq tracks the highest sequence number seen per user ID to suppress duplicates.
    lastSeq        map[int]int
}

func NewTCPServer(addr string) (Server, error) {
    return &TCPServer{
        addr:    addr,
        aConns:  make(map[net.Conn]model.Payload),
        iConns:  make(chan net.Conn, 10),
        dConns:  make(chan net.Conn, 1),
        // initialise storedMessages and lastSeq maps
        storedMessages: make(map[string]model.ChatMessage),
        lastSeq:        make(map[int]int),
    }, nil
}

// Run starts the TCP Server
func (t *TCPServer) Run() (err error) {
    t.server, err = net.Listen("tcp", t.addr)
    if err != nil {
        return err
    }
    defer t.Close()
    t.storedMessages = make(map[string]model.ChatMessage)
    t.lastSeq = make(map[int]int)

    go t.broadcaster()
    go t.messageStatusMonitor()

    for {
        conn, err := t.server.Accept()
        if err != nil {
            if strings.Contains(err.Error(), "use of closed network connection") {
                return nil
            }
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
        case iConn := <-t.iConns:
            t.connMu.RLock()
            payload := t.aConns[iConn]
            t.connMu.RUnlock()
            log.Printf("user %d is online", payload.UserID)
            t.notifyFriends(payload, true)
        case dConn := <-t.dConns:
            t.connMu.RLock()
            payload := t.aConns[dConn]
            t.connMu.RUnlock()
            log.Printf("user %d is offline", payload.UserID)
            t.notifyFriends(payload, false)
            t.connMu.Lock()
            delete(t.aConns, dConn)
            t.connMu.Unlock()
        }
    }
}

// handleConn - it decodes the payload and add the incoming connection into the active connections map
func (t *TCPServer) handleConn(conn net.Conn) {
    // First, read the initial payload from the client to register the user.
    // The client sends a JSON payload string, not a ChatMessage.
    d := json.NewDecoder(conn)
    var pl model.Payload
    if err := d.Decode(&pl); err != nil {
        log.Printf("failed to decode initial payload: %v", err)
        conn.Close()
        return
    }
    // Ensure storedMessages map is initialized.
    if t.storedMessages == nil {
        t.storedMessages = make(map[string]model.ChatMessage)
    }
    // Register connection safely
    t.connMu.Lock()
    t.aConns[conn] = pl
    t.connMu.Unlock()
    // Notify other users that this user is now online
    t.iConns <- conn

    // Now handle chat messages from this connection.
    for {
        var chatMsg model.ChatMessage
        if err := d.Decode(&chatMsg); err != nil {
            // Assume connection closed or read error; break loop.
            break
        }
        // Generate a unique message ID if not present
        if chatMsg.ID == "" {
            chatMsg.ID = uuid.New().String()
        }
        // Duplicate detection based on sequence number (if present) and ID
        // If the message ID already exists, treat it as a duplicate and ignore.
        t.connMu.RLock()
        _, exists := t.storedMessages[chatMsg.ID]
        t.connMu.RUnlock()
        if exists {
            // Still send ACK to satisfy client expectations
            t.updateMessageStatus(chatMsg)
            continue
        }
        // Store the message with Sent status
        t.connMu.Lock()
        chatMsg.Status = model.Sent
        t.storedMessages[chatMsg.ID] = chatMsg
        t.connMu.Unlock()

        // Deliver the message with initial Delivered status
        if chatMsg.IsGroupMessage() {
            t.broadcastToGroupWithStatus(chatMsg.GroupID, chatMsg, model.Delivered)
        } else {
            t.sendToUserWithStatus(chatMsg.To, chatMsg, model.Delivered)
        }

        // If recipient is online, immediately mark as Delivered and notify sender
        if t.isConnected(chatMsg.To) {
            t.connMu.Lock()
            chatMsg.Status = model.Delivered
            t.storedMessages[chatMsg.ID] = chatMsg
            t.connMu.Unlock()
            t.updateMessageStatus(chatMsg)
        }
    }
    // Connection closed: notify offline and clean up
    select { case t.dConns <- conn: default: }
}

// notifyFriends - it notifies the user friends his status
func (t *TCPServer) notifyFriends(payload model.Payload, isOnline bool) {
    t.connMu.RLock()
    defer t.connMu.RUnlock()
    // Prepare concise status JSON
    statusPayload := struct {
        UserID int  `json:"user_id"`
        Online bool `json:"online"`
    }{
        UserID: payload.UserID,
        Online: isOnline,
    }
    data, _ := json.Marshal(statusPayload)
    data = append(data, '\n')
    for _, f := range payload.Friends {
        for k, v := range t.aConns {
            if f == v.UserID {
                if _, err := k.Write(data); err != nil {
                    log.Printf("error notifying friend %d about user %d: %v", f, payload.UserID, err)
                }
            }
        }
    }
}

// sendToUserWithStatus sends a message to a specific user with status tracking
func (t *TCPServer) sendToUserWithStatus(to int, msg model.ChatMessage, status model.MessageStatus) {
    // Locate the recipient's connection and send the message.
    var sent bool
    t.connMu.RLock()
    for k, v := range t.aConns {
        if v.UserID == to {
            // Preserve existing status; caller can set it before calling.
            data, _ := json.Marshal(msg)
            if _, err := k.Write(append(data, '\n')); err != nil {
                log.Printf("error sending message %s to user %d: %v", msg.ID, to, err)
            } else {
                sent = true
            }
            break
        }
    }
    t.connMu.RUnlock()
    if sent {
        // Update stored message status safely
        t.connMu.Lock()
        t.storedMessages[msg.ID] = msg
        t.connMu.Unlock()
    }
}

// broadcastToGroupWithStatus sends a message to all group members with status tracking
func (t *TCPServer) broadcastToGroupWithStatus(groupID int, msg model.ChatMessage, status model.MessageStatus) {
    // For now, broadcast to all connected users except the sender.
    var sent bool
    t.connMu.RLock()
    for k, v := range t.aConns {
        if v.UserID != msg.From { // skip sender
            // Preserve existing status; caller can set it.
            data, _ := json.Marshal(msg)
            if _, err := k.Write(append(data, '\n')); err != nil {
                log.Printf("error broadcasting message %s to user %d: %v", msg.ID, v.UserID, err)
            } else {
                sent = true
            }
        }
    }
    t.connMu.RUnlock()
    if sent {
        t.connMu.Lock()
        t.storedMessages[msg.ID] = msg
        t.connMu.Unlock()
    }
}

// updateMessageStatus updates the status of a message and notifies the sender
func (t *TCPServer) updateMessageStatus(msg model.ChatMessage) {
    t.storedMessages[msg.ID] = msg
    t.connMu.RLock()
    for k, v := range t.aConns {
        if v.UserID == msg.From {
            statusUpdate := model.ChatMessage{
                ID:     msg.ID,
                Status: msg.Status,
            }
            data, _ := json.Marshal(statusUpdate)
            if _, err := k.Write(append(data, '\n')); err != nil {
                log.Printf("error sending status update for message %s to user %d: %v", msg.ID, v.UserID, err)
            }
        }
    }
    t.connMu.RUnlock()
}

// isConnected checks if a user is currently online
func (t *TCPServer) isConnected(userID int) bool {
    t.connMu.RLock()
    defer t.connMu.RUnlock()
    for _, p := range t.aConns {
        if p.UserID == userID {
            return true
        }
    }
    return false
}

// messageStatusMonitor periodically checks for status updates
func (t *TCPServer) messageStatusMonitor() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        <-ticker.C
        // Implement logic to check for status updates from clients
        // For now, just log the stored messages
        log.Printf("Stored messages: %v", t.storedMessages)
    }
}
