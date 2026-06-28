package server

// Server defines the minimum contract our TCP and UDP server implementations must satisfy.
type Server interface {
    Run() error
    Close() error
}
