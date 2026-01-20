package web

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSocketServer_NewSocketServer(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	store := NewStore()
	hub := NewHub()

	server := NewSocketServer(socketPath, store, hub)

	if server.Path() != socketPath {
		t.Errorf("Path() = %q, want %q", server.Path(), socketPath)
	}
}

func TestSocketServer_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	// Start should create socket file
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Check socket file exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("socket file was not created")
	}

	// Stop should remove socket file
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Check socket file is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file was not removed")
	}
}

func TestSocketServer_RemovesStaleSocket(t *testing.T) {
	// Use /tmp directly to avoid path length issues on macOS
	socketPath := filepath.Join("/tmp", fmt.Sprintf("choo-test-%d.sock", time.Now().UnixNano()))
	defer os.Remove(socketPath)

	// Create stale socket file
	staleFile, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("failed to create stale file: %v", err)
	}
	staleFile.Close()

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	// Start should remove stale socket and create new one
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Should be able to connect (proving it's a real socket, not the stale file)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Errorf("failed to connect to socket: %v", err)
	} else {
		conn.Close()
	}
}

func TestSocketServer_AcceptsConnection(t *testing.T) {
	// Use /tmp directly to avoid path length issues on macOS
	socketPath := filepath.Join("/tmp", fmt.Sprintf("choo-test-%d.sock", time.Now().UnixNano()))
	defer os.Remove(socketPath)

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Connection succeeds
}

func TestSocketServer_ParsesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Write an orch.started event
	event := Event{
		Type: "orch.started",
		Time: time.Now(),
		Payload: json.RawMessage(`{
			"unit_count": 3,
			"parallelism": 2,
			"graph": {
				"nodes": [{"id": "unit1", "level": 0}],
				"edges": [],
				"levels": [["unit1"]]
			}
		}`),
	}

	eventJSON, _ := json.Marshal(event)
	fmt.Fprintf(conn, "%s\n", eventJSON)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Check store received event
	snapshot := store.Snapshot()
	if snapshot.Status != "running" {
		t.Errorf("store status = %q, want %q", snapshot.Status, "running")
	}
	if snapshot.Parallelism != 2 {
		t.Errorf("store parallelism = %d, want 2", snapshot.Parallelism)
	}
}

func TestSocketServer_BroadcastsEvents(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "t.sock")

	store := NewStore()
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Register SSE client
	client := NewClient("test-client")
	hub.Register(client)

	// Give registration time to process
	time.Sleep(10 * time.Millisecond)

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Write event
	event := Event{
		Type: "test.event",
		Time: time.Now(),
	}
	eventJSON, _ := json.Marshal(event)
	fmt.Fprintf(conn, "%s\n", eventJSON)

	// SSE client should receive event
	select {
	case received := <-client.events:
		if received.Type != "test.event" {
			t.Errorf("received event type = %q, want %q", received.Type, "test.event")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestSocketServer_SetsConnected(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Initially not connected
	if store.Snapshot().Connected {
		t.Error("store should not be connected initially")
	}

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Give connection time to be handled
	time.Sleep(50 * time.Millisecond)

	// Should be connected
	if !store.Snapshot().Connected {
		t.Error("store should be connected after connection")
	}

	// Close connection
	conn.Close()

	// Give disconnection time to be handled
	time.Sleep(50 * time.Millisecond)

	// Should be disconnected
	if store.Snapshot().Connected {
		t.Error("store should be disconnected after connection closed")
	}
}

func TestSocketServer_HandlesMalformedJSON(t *testing.T) {
	// Use /tmp directly to avoid path length issues on macOS
	socketPath := filepath.Join("/tmp", fmt.Sprintf("choo-test-%d.sock", time.Now().UnixNano()))
	defer os.Remove(socketPath)

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Write malformed JSON
	fmt.Fprintf(conn, "{invalid json}\n")

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Write valid event
	event := Event{
		Type: "test.event",
		Time: time.Now(),
	}
	eventJSON, _ := json.Marshal(event)
	fmt.Fprintf(conn, "%s\n", eventJSON)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Should still be connected (didn't crash)
	if !store.Snapshot().Connected {
		t.Error("server should still be connected after malformed JSON")
	}
}

func TestSocketServer_DefaultSocketPath(t *testing.T) {
	// Test with XDG_RUNTIME_DIR set
	tmpDir := t.TempDir()
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Unsetenv("XDG_RUNTIME_DIR")

	path := defaultSocketPath()
	expectedPrefix := filepath.Join(tmpDir, "choo")
	if !strings.HasPrefix(path, expectedPrefix) {
		t.Errorf("with XDG_RUNTIME_DIR, path = %q, should have prefix %q", path, expectedPrefix)
	}
	if !strings.HasSuffix(path, "web.sock") {
		t.Errorf("path = %q, should end with web.sock", path)
	}

	// Test without XDG_RUNTIME_DIR
	os.Unsetenv("XDG_RUNTIME_DIR")
	path = defaultSocketPath()
	home, _ := os.UserHomeDir()
	expectedPrefix = filepath.Join(home, ".choo")
	if !strings.HasPrefix(path, expectedPrefix) {
		t.Errorf("without XDG_RUNTIME_DIR, path = %q, should have prefix %q", path, expectedPrefix)
	}
	if !strings.HasSuffix(path, "web.sock") {
		t.Errorf("path = %q, should end with web.sock", path)
	}
}

func TestSocketServer_LargeEvents(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	store := NewStore()
	hub := NewHub()
	server := NewSocketServer(socketPath, store, hub)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Create large payload (near 1MB)
	largePayload := strings.Repeat("x", 900*1024) // 900KB
	event := Event{
		Type:    "test.large",
		Time:    time.Now(),
		Payload: json.RawMessage(fmt.Sprintf(`{"data":"%s"}`, largePayload)),
	}

	eventJSON, _ := json.Marshal(event)
	fmt.Fprintf(conn, "%s\n", eventJSON)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Should still be connected (event parsed successfully)
	if !store.Snapshot().Connected {
		t.Error("server should still be connected after large event")
	}
}
