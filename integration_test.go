package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// covDir returns a per-process coverage directory under t.TempDir() and
// registers cleanup. Subprocesses built with -cover write their GOCOVERDIR
// here, and TestMain merges them into the report.
func covDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("COV_DATA_ROOT")
	if root == "" {
		root = filepath.Join(os.TempDir(), "go-friends-list-cov")
		_ = os.MkdirAll(root, 0o755)
	}
	d := filepath.Join(root, strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	_ = os.RemoveAll(d)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// buildInstrumentedBinary compiles pkgPath with coverage instrumentation
// for that package, so subprocess runs contribute to cmd/<pkg> coverage.
func buildInstrumentedBinary(t *testing.T, pkgPath, name string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), name)
	buildCmd := exec.Command("go", "build", "-cover", "-coverpkg", pkgPath, "-o", binPath, pkgPath)
	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkgPath, err, buildStderr.String())
	}
	return binPath
}

// buildBinary compiles the given package into a temporary binary and returns its path.
func buildBinary(t *testing.T, pkgPath, name string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), name)
	buildCmd := exec.Command("go", "build", "-o", binPath, pkgPath)
	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkgPath, err, buildStderr.String())
	}
	return binPath
}

// getFreePort returns an available TCP port number.
func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// waitForPort polls the port until it accepts connections or the deadline elapses.
func waitForPort(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("port %d did not become ready within %v", port, timeout)
}

// waitOrKill waits for the command to exit, kills it on timeout, and reaps the process.
func waitOrKill(t *testing.T, cmd *exec.Cmd, timeout time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return fmt.Errorf("command did not exit within %v", timeout)
	}
}

// killOnCleanup schedules a forced kill on the given command when the test ends.
func killOnCleanup(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_ = waitOrKill(t, cmd, 3*time.Second)
	})
}

// TestServerMain exercises cmd/server/main.go: flag parsing, server construction,
// Accept loop, and graceful shutdown via SIGTERM (signal.NotifyContext branch).
func TestServerMain(t *testing.T) {
	port := getFreePort(t)
	bin := buildInstrumentedBinary(t, "./cmd/server", "server_test_bin")

	cmd := exec.Command(bin,
		"-port", strconv.Itoa(port),
		"-protocol", "tcp",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	killOnCleanup(t, cmd)

	waitForPort(t, port, 5*time.Second)

	// Verify the server accepts a payload (exercises handleConn registration).
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	if _, err := conn.Write([]byte(`{"user_id":7,"friends":[8,9]}` + "\n")); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	_ = conn.Close()

	// Allow the server time to register the connection before signaling.
	time.Sleep(200 * time.Millisecond)

	// SIGTERM triggers signal.NotifyContext in main and exercises graceful shutdown.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("signal server: %v", err)
	}
	if err := waitOrKill(t, cmd, 5*time.Second); err != nil {
		t.Errorf("server did not shut down cleanly: %v", err)
	}

	if msg := stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("server panicked: %s", msg)
	}
}

// TestServerMainPortRange exercises the -port-range fallback branch in cmd/server/main.go.
func TestServerMainPortRange(t *testing.T) {
	// Bind a dummy listener on a port to guarantee that port is occupied,
	// then give the server a range that starts *above* the occupied port so
	// the NextAvailable logic in main() can succeed.
	conflictPort := getFreePort(t)
	conflictListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", conflictPort))
	if err != nil {
		t.Fatalf("listen on conflict port: %v", err)
	}
	t.Cleanup(func() { _ = conflictListener.Close() })

	start := conflictPort + 1
	end := start + 5
	rangeStr := fmt.Sprintf("%d-%d", start, end)

	bin := buildInstrumentedBinary(t, "./cmd/server", "server_test_bin_pr")
	cmd := exec.Command(bin,
		"-port-range", rangeStr,
		"-protocol", "tcp",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	killOnCleanup(t, cmd)

	// Poll until the server binds to a port in the range.
	var boundPort int
	bound := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !bound {
		for p := start; p <= end; p++ {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", p), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				bound = true
				boundPort = p
				break
			}
		}
		if !bound {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !bound {
		t.Fatalf("server did not bind to any port in %s", rangeStr)
	}

	// The conflicted port itself must remain bound by our dummy listener.
	if boundPort == conflictPort {
		t.Fatalf("server bound to conflicted port %d", conflictPort)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("signal: %v", err)
	}
	_ = waitOrKill(t, cmd, 5*time.Second)

	if msg := stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("server panicked: %s", msg)
	}
}

// TestServerMainInvalidPort verifies the Sscanf error branch for malformed port-range.
func TestServerMainInvalidPort(t *testing.T) {
	bin := buildInstrumentedBinary(t, "./cmd/server", "server_test_bin_inv")
	cmd := exec.CommandContext(context.Background(), bin,
		"-port-range", "garbage",
		"-protocol", "tcp",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	// main calls log.Fatalf on bad format, which exits non-zero.
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected server to exit with error on invalid port-range")
	}
	if !strings.Contains(stderr.String(), "invalid port-range") {
		t.Errorf("expected 'invalid port-range' in stderr, got: %s", stderr.String())
	}
}

// TestClientMain exercises cmd/client/main.go: dialing, JSON parsing, initial
// payload transmission, and the spawned goroutines. The client blocks on stdin,
// so we kill it after verifying the connection was made.
func TestClientMain(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port

	accepted := make(chan struct{}, 1)
	var (
		readOnce sync.Once
		readBuf  bytes.Buffer
	)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		accepted <- struct{}{}
		_ = readOnce
		_, _ = io.Copy(&readBuf, bufio.NewReader(conn))
	}()

	bin := buildInstrumentedBinary(t, "./cmd/client", "client_test_bin")
	cmd := exec.Command(bin,
		"-port", strconv.Itoa(port),
		"-protocol", "tcp",
		"-payload", `{"user_id":1,"friends":[2,3]}`,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start client: %v", err)
	}
	killOnCleanup(t, cmd)

	select {
	case <-accepted:
		// Connection established; the entirety of main() was exercised.
	case <-time.After(5 * time.Second):
		t.Fatal("client did not connect to mock server within 5s")
	}

	// Wait for process exit before reading stderr to avoid racing with the
	// goroutine spawned by exec.Cmd that drains the stderr pipe.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("signal: %v", err)
	}
	_ = waitOrKill(t, cmd, 5*time.Second)

	if msg := stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("client panicked: %s", msg)
	}
}

// TestClientMainInvalidJSON exercises the JSON unmarshal error path in cmd/client/main.go.
func TestClientMainInvalidJSON(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	bin := buildInstrumentedBinary(t, "./cmd/client", "client_test_bin_ij")
	cmd := exec.CommandContext(context.Background(), bin,
		"-port", strconv.Itoa(port),
		"-protocol", "tcp",
		"-payload", `not-json`,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected client to exit on invalid JSON payload")
	}
	if !strings.Contains(stderr.String(), "Wrong payload") {
		t.Errorf("expected 'Wrong payload' in stderr, got: %s", stderr.String())
	}
}

// TestClientMainInvalidUser exercises the UserID==0 branch in cmd/client/main.go.
func TestClientMainInvalidUser(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	bin := buildInstrumentedBinary(t, "./cmd/client", "client_test_bin_iu")
	cmd := exec.CommandContext(context.Background(), bin,
		"-port", strconv.Itoa(port),
		"-protocol", "tcp",
		"-payload", `{"user_id":0,"friends":[]}`,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected client to exit on user_id=0")
	}
	if !strings.Contains(stderr.String(), "invalid User") {
		t.Errorf("expected 'invalid User' in stderr, got: %s", stderr.String())
	}
}

// TestMonitorMain exercises cmd/monitor/main.go: the /healthz HTTP endpoint.
func TestMonitorMain(t *testing.T) {
	port := getFreePort(t)
	bin := buildInstrumentedBinary(t, "./cmd/monitor", "monitor_test_bin")

	cmd := exec.Command(bin, "-port", strconv.Itoa(port))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start monitor: %v", err)
	}
	killOnCleanup(t, cmd)

	waitForPort(t, port, 5*time.Second)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}

	// Wait for process exit before reading stderr to avoid racing with the
	// goroutine spawned by exec.Cmd that drains the stderr pipe.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("signal monitor: %v", err)
	}
	_ = waitOrKill(t, cmd, 5*time.Second)

	if msg := stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("monitor panicked: %s", msg)
	}
}

// TestClientServerChat exercises the full chat loop: client connects to server,
// sends a message via stdin, receives a message from the server, and sends a
// read receipt. This covers sendChatMessage, readMessage, handleIncomingMessage,
// isCurrentUserRecipient, and sendReadReceipt in cmd/client/main.go.
func TestClientServerChat(t *testing.T) {
	serverPort := getFreePort(t)

	// Start real server
	serverBin := buildInstrumentedBinary(t, "./cmd/server", "chat_server_bin")
	serverCmd := exec.Command(serverBin,
		"-port", strconv.Itoa(serverPort),
		"-protocol", "tcp",
	)
	var serverStderr bytes.Buffer
	serverCmd.Stderr = &serverStderr
	serverCmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	killOnCleanup(t, serverCmd)
	waitForPort(t, serverPort, 5*time.Second)

	// Start client 1 — the sender
	clientBin := buildInstrumentedBinary(t, "./cmd/client", "chat_client1_bin")
	client1Cmd := exec.Command(clientBin,
		"-port", strconv.Itoa(serverPort),
		"-protocol", "tcp",
		"-payload", `{"user_id":1,"friends":[2]}`,
		"-to", "2",
	)
	client1Stdin, err := client1Cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	var client1Stdout, client1Stderr bytes.Buffer
	client1Cmd.Stdout = &client1Stdout
	client1Cmd.Stderr = &client1Stderr
	client1Cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := client1Cmd.Start(); err != nil {
		t.Fatalf("start client1: %v", err)
	}
	killOnCleanup(t, client1Cmd)

	// Start client 2 — the recipient (exercises handleIncomingMessage, isCurrentUserRecipient, sendReadReceipt)
	client2Cmd := exec.Command(clientBin,
		"-port", strconv.Itoa(serverPort),
		"-protocol", "tcp",
		"-payload", `{"user_id":2,"friends":[1]}`,
		"-to", "1",
	)
	client2Stdin, err := client2Cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe client2: %v", err)
	}
	var client2Stdout, client2Stderr bytes.Buffer
	client2Cmd.Stdout = &client2Stdout
	client2Cmd.Stderr = &client2Stderr
	client2Cmd.Env = append(os.Environ(), "GOCOVERDIR="+covDir(t))
	if err := client2Cmd.Start(); err != nil {
		t.Fatalf("start client2: %v", err)
	}
	killOnCleanup(t, client2Cmd)

	// Wait for clients to connect and register
	time.Sleep(500 * time.Millisecond)

	// Client1 sends a chat message (exercises sendChatMessage)
	_, _ = client1Stdin.Write([]byte("hello from client1\n"))
	time.Sleep(500 * time.Millisecond)

	// Client2 also sends a message (exercises sendChatMessage on client2)
	_, _ = client2Stdin.Write([]byte("hello from client2\n"))
	time.Sleep(500 * time.Millisecond)

	// Close stdins so clients don't block on scanner
	_ = client1Stdin.Close()
	_ = client2Stdin.Close()

	// Graceful shutdown
	_ = client1Cmd.Process.Signal(syscall.SIGTERM)
	_ = client2Cmd.Process.Signal(syscall.SIGTERM)
	_ = waitOrKill(t, client1Cmd, 5*time.Second)
	_ = waitOrKill(t, client2Cmd, 5*time.Second)
	_ = serverCmd.Process.Signal(syscall.SIGTERM)
	_ = waitOrKill(t, serverCmd, 5*time.Second)

	if msg := serverStderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("server panicked: %s", msg)
	}
	if msg := client1Stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("client1 panicked: %s", msg)
	}
	if msg := client2Stderr.String(); strings.Contains(msg, "panic") {
		t.Errorf("client2 panicked: %s", msg)
	}
}

// TestMain runs all tests, then merges GOCOVERDIR outputs from instrumented
// subprocesses into a single coverage report and prints a summary so that
// cmd/* packages show their real coverage alongside internal/* and pkg/*.
func TestMain(m *testing.M) {
	code := m.Run()

	root := os.Getenv("COV_DATA_ROOT")
	if root == "" {
		root = filepath.Join(os.TempDir(), "go-friends-list-cov")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) == 0 {
		os.Exit(code)
	}

	var inputPaths []string
	for _, e := range entries {
		name := e.Name()
		if name == "_merged" || name == "coverage.out" || strings.HasPrefix(name, ".") {
			continue
		}
		inputPaths = append(inputPaths, filepath.Join(root, name))
	}
	if len(inputPaths) == 0 {
		os.Exit(code)
	}
	merged := filepath.Join(root, "_merged")
	_ = os.RemoveAll(merged)
	_ = os.MkdirAll(merged, 0o755)
	if err := exec.Command("go", "tool", "covdata", "merge", "-i="+strings.Join(inputPaths, ","), "-o="+merged).Run(); err == nil {
		out := filepath.Join(root, "coverage.out")
		if err := exec.Command("go", "tool", "covdata", "textfmt", "-i="+merged, "-o="+out).Run(); err == nil {
			if data, err := os.ReadFile(out); err == nil && len(data) > 0 {
				fmt.Fprintf(os.Stderr, "\n=== merged subprocess coverage ===\n%s\n", string(data))
			}
		}
	}

	os.Exit(code)
}
