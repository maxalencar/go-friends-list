package protocol

import (
	"errors"
	"strings"

	"go-friends-list/internal/server"
)

// NewServer creates a server implementation based on the provided protocol.
// It mirrors the behaviour of the original NewServer in the top-level server
// package but lives in its own subpackage so that the factory can be unit
// tested in isolation.
func NewServer(protocol, addr string) (server.Server, error) {
	switch strings.ToLower(protocol) {
	case "tcp":
		return server.NewTCPServer(addr)
	case "udp":
		return server.NewUDPServer(addr)
	default:
		return nil, errors.New("invalid protocol given")
	}
}
