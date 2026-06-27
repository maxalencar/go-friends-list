# GO Friends List

This is a simple friends‑list notification application supporting the TCP protocol, demonstrating how multiple connections can broadcast messages through channels.

## Features Implemented
- **TCP Server**: Full chat capability with message status tracking (`sent`, `delivered`, `read`).
- **Client**:
  - Dynamic `-to` flag to specify the recipient of outgoing chat messages.
  - Proper read‑receipt handling and status symbols (`⏳`, `✓`, `[READ]`, `❌`).
  - Configurable ACK timeout (`-ack-timeout`) and maximum retransmissions (`-max-retries`).
  - Supports TCP protocol.
- **Refactor & Concurrency**:
  - All mutable server state (`connections`, `channels`, etc.) encapsulated within `TCPServer` and `UDPServer` structs.
  - Thread‑safe access using `sync.RWMutex`.
- **Graceful shutdown** on SIGINT/SIGTERM.
- **Testing**:
  - Unit and integration tests for TCP server start‑up, handshake and message delivery.

## TODO (Remaining Work)
- **Enhanced Client UI**: Better terminal UI for displaying friend list and offline/online status.

## Getting Set Up

Before running the application, ensure you have Go installed.

### Go

[Go](https://golang.org/) is an open source programming language that makes it easy to build simple, reliable, and efficient software.

## Running the server

```bash
go run cmd/server/main.go -protocol tcp -port 8080
```

## Running the client

You can run multiple instances of the client. Provide a JSON payload with the user ID and friends list. You can also specify the recipient of chat messages using the `-to` flag, and adjust ACK behavior with the new flags.

```bash
go run cmd/client/main.go \
  -payload '{"user_id":1,"friends":[2,3,4]}' \
  -to 2 \
  -protocol tcp \
  -port 8080 \
  -ack-timeout 1000 \
  -max-retries 5
```

### UDP client example (reliable)

```bash
go run cmd/client/main.go \
  -payload '{"user_id":1,"friends":[2,3,4]}' \
  -to 2 \
  -protocol udp \
  -port 8080 \
  -ack-timeout 1000 \
  -max-retries 5
```

## Usage of client:

```
-payload string
    User Identification. (default "{\"user_id\": 0, \"friends\": []}")
-to int
    Recipient user ID for outgoing chat messages (default 0 means none)
-port int
    TCP/UDP Port. (default 8080)
-protocol string
    Protocol used, currently supporting tcp and udp. (default "tcp")
-heartbeat int
    Heartbeat interval for UDP server (client does not use this flag).
-ack-timeout int
    ACK timeout in milliseconds for reliable UDP (default 1000)
-max-retries int
    Maximum retransmission attempts for UDP messages (default 5)
```

## Lint / static analysis

```bash
go vet ./...
```

---

[@maxalencar](https://github.com/maxalencar)
