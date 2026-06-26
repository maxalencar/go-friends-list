package model

import (
	"github.com/google/uuid"
)

// MessageStatus defines the delivery status of a message
type MessageStatus string

const (
	Sent      MessageStatus = "sent"      // Message has been sent
	Delivered MessageStatus = "delivered" // Message reached recipient's device
	Read      MessageStatus = "read"      // Recipient has read the message
)

// ChatMessage represents a chat message with status tracking
type ChatMessage struct {
    // Sequence number for ordering and ACK tracking
    Seq int `json:"seq,omitempty"`
    // Retransmission count (optional, for debugging)
    Retries int `json:"retries,omitempty"`

	ID          string        `json:"id,omitempty"` // Unique message identifier
	From        int           `json:"from"`
	To          int           `json:"to"`
	GroupID     int           `json:"group_id,omitempty"`
	Content     string        `json:"content"`
	MediaType   string        `json:"media_type,omitempty"`
	Timestamp   int64         `json:"timestamp"`
	ReadAt      int64         `json:"read_at,omitempty"`
	Status      MessageStatus `json:"status,omitempty"`
	RecipientID int           `json:"recipient_id,omitempty"` // For tracking individual recipients in group chats
}

// Payload represents the data sent from client to server to identify the user and friends.
// It matches the JSON payload used by the client flag "payload".
type Payload struct {
	UserID  int   `json:"user_id"`
	Friends []int `json:"friends"`
}

// Helper functions
func (m *ChatMessage) MarkRead(ts int64) {
    m.Status = Read
    m.ReadAt = ts
}
func (m *ChatMessage) IsGroupMessage() bool {
	return m.GroupID > 0
}

func (m *ChatMessage) IsBroadcast() bool {
	return m.To == 0
}

// AckMessage is sent by the receiver to acknowledge a ChatMessage
type AckMessage struct {
    ID   string `json:"id"`
    From int    `json:"from"`
    To   int    `json:"to"`
    Seq  int    `json:"seq"`
}

// NewMessage creates a new message with a unique ID
func NewMessage(from, to int, content string) *ChatMessage {
	return &ChatMessage{
		ID:      uuid.New().String(),
		From:    from,
		To:      to,
		Content: content,
		Status:  Sent,
	}
}
