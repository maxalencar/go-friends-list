package server

import (
	"errors"
	"strings"
)

// Server defines the minimum contract our
// TCP and UDP server implementations must satisfy.
type Server interface {
	Run() error
	Close() error
}

// NewServer - it creates a new Server using given protocol and addr.
func NewServer(protocol, addr string) (Server, error) {
	switch strings.ToLower(protocol) {
	case "tcp":
		return NewTCPServer(addr)
	case "udp":
		return NewUDPServer(addr)
	}
	return nil, errors.New("invalid protocol given")
}
