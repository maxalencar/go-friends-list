package model

// Payload - payload expected by the client
type Payload struct {
	UserID  int   `json:"user_id"`
	Friends []int `json:"friends"`
}
