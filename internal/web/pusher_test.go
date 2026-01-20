package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/scheduler"
)

func TestSocketPusher_NewSocketPusher(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	t.Run("applies default config values", func(t *testing.T) {
		cfg := PusherConfig{
			SocketPath: "/tmp/test.sock",
		}
		p := NewSocketPusher(bus, cfg)

		if p.cfg.BufferSize != 1000 {
			t.Errorf("expected BufferSize=1000, got %d", p.cfg.BufferSize)
		}
		if p.cfg.WriteTimeout != 5*time.Second {
			t.Errorf("expected WriteTimeout=5s, got %v", p.cfg.WriteTimeout)
		}
		if p.cfg.ReconnectBackoff != 100*time.Millisecond {
			t.Errorf("expected ReconnectBackoff=100ms, got %v", p.cfg.ReconnectBackoff)
		}
		if p.cfg.MaxReconnectBackoff != 5*time.Second {
			t.Errorf("expected MaxReconnectBackoff=5s, got %v", p.cfg.MaxReconnectBackoff)
		}
	})

	t.Run("preserves custom config values", func(t *testing.T) {
		cfg := PusherConfig{
			SocketPath:          "/tmp/test.sock",
			BufferSize:          500,
			WriteTimeout:        10 * time.Second,
			ReconnectBackoff:    200 * time.Millisecond,
			MaxReconnectBackoff: 10 * time.Second,
		}
		p := NewSocketPusher(bus, cfg)

		if p.cfg.BufferSize != 500 {
			t.Errorf("expected BufferSize=500, got %d", p.cfg.BufferSize)
		}
		if p.cfg.WriteTimeout != 10*time.Second {
			t.Errorf("expected WriteTimeout=10s, got %v", p.cfg.WriteTimeout)
		}
		if p.cfg.ReconnectBackoff != 200*time.Millisecond {
			t.Errorf("expected ReconnectBackoff=200ms, got %v", p.cfg.ReconnectBackoff)
		}
		if p.cfg.MaxReconnectBackoff != 10*time.Second {
			t.Errorf("expected MaxReconnectBackoff=10s, got %v", p.cfg.MaxReconnectBackoff)
		}
	})
}

func TestSocketPusher_SetGraph(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a simple graph: A -> B -> C (C depends on B, B depends on A)
	units := []*discovery.Unit{
		{ID: "A", DependsOn: []string{}},
		{ID: "B", DependsOn: []string{"A"}},
		{ID: "C", DependsOn: []string{"B"}},
	}

	graph, err := scheduler.NewGraph(units)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
	p := NewSocketPusher(bus, cfg)
	p.SetGraph(graph, 2)

	if p.graph == nil {
		t.Fatal("expected graph to be set")
	}

	// Check nodes
	if len(p.graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(p.graph.Nodes))
	}

	// Check edges - B depends on A, C depends on B
	if len(p.graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(p.graph.Edges))
	}

	// Check levels - should have 3 levels: [A], [B], [C]
	if len(p.graph.Levels) != 3 {
		t.Errorf("expected 3 levels, got %d", len(p.graph.Levels))
	}

	// Verify level contents
	if len(p.graph.Levels[0]) != 1 || p.graph.Levels[0][0] != "A" {
		t.Errorf("expected level 0 to be [A], got %v", p.graph.Levels[0])
	}
	if len(p.graph.Levels[1]) != 1 || p.graph.Levels[1][0] != "B" {
		t.Errorf("expected level 1 to be [B], got %v", p.graph.Levels[1])
	}
	if len(p.graph.Levels[2]) != 1 || p.graph.Levels[2][0] != "C" {
		t.Errorf("expected level 2 to be [C], got %v", p.graph.Levels[2])
	}
}

func TestSocketPusher_Connected(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
	p := NewSocketPusher(bus, cfg)

	// Initially not connected
	if p.Connected() {
		t.Error("expected not connected initially")
	}
}

func TestSocketPusher_StartClose(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a temporary socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		connCh <- conn
	}()

	cfg := PusherConfig{
		SocketPath:   socketPath,
		WriteTimeout: 1 * time.Second,
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Should be connected now
	if !p.Connected() {
		t.Error("expected connected after start")
	}

	// Wait for server to accept
	select {
	case conn := <-connCh:
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connection")
	}

	// Close the pusher
	if err := p.Close(); err != nil {
		t.Errorf("close error: %v", err)
	}

	// Should not be connected after close
	if p.Connected() {
		t.Error("expected not connected after close")
	}
}

func TestSocketPusher_EventForwarding(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a temporary socket with short path (macOS has socket path limit)
	socketPath := filepath.Join(os.TempDir(), "choo-ef.sock")
	os.Remove(socketPath) // Clean up any existing socket
	defer os.Remove(socketPath)

	// Start a listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections and read events
	eventCh := make(chan WireEvent, 10)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var event WireEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				continue
			}
			eventCh <- event
		}
	}()

	cfg := PusherConfig{
		SocketPath:   socketPath,
		WriteTimeout: 1 * time.Second,
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer p.Close()

	// Emit an event
	taskNum := 1
	bus.Emit(events.Event{
		Type: events.UnitStarted,
		Unit: "test-unit",
		Task: &taskNum,
	})

	// Wait for the event
	select {
	case received := <-eventCh:
		if received.Type != string(events.UnitStarted) {
			t.Errorf("expected type %s, got %s", events.UnitStarted, received.Type)
		}
		if received.Unit != "test-unit" {
			t.Errorf("expected unit test-unit, got %s", received.Unit)
		}
		if received.Task == nil || *received.Task != 1 {
			t.Errorf("expected task 1, got %v", received.Task)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSocketPusher_Reconnect(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Create a temporary socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	// Accept first connection
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		connCh <- conn
	}()

	cfg := PusherConfig{
		SocketPath:          socketPath,
		WriteTimeout:        1 * time.Second,
		ReconnectBackoff:    10 * time.Millisecond,
		MaxReconnectBackoff: 50 * time.Millisecond,
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer p.Close()

	// Get first connection and close it to simulate disconnect
	var firstConn net.Conn
	select {
	case firstConn = <-connCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first connection")
	}

	// Start accepting second connection before closing first
	eventCh := make(chan WireEvent, 10)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var event WireEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				continue
			}
			eventCh <- event
		}
	}()

	// Close first connection to trigger reconnect
	firstConn.Close()

	// Emit an event - should trigger reconnect and send
	bus.Emit(events.Event{
		Type: events.UnitCompleted,
		Unit: "reconnect-test",
	})

	// Wait for the event on new connection
	select {
	case received := <-eventCh:
		if received.Type != string(events.UnitCompleted) {
			t.Errorf("expected type %s, got %s", events.UnitCompleted, received.Type)
		}
		if received.Unit != "reconnect-test" {
			t.Errorf("expected unit reconnect-test, got %s", received.Unit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reconnected event")
	}

	listener.Close()
}

func TestSocketPusher_StartFailsWithoutSocket(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Use a non-existent socket path
	cfg := PusherConfig{
		SocketPath: "/tmp/nonexistent-" + t.Name() + ".sock",
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()
	err := p.Start(ctx)

	if err == nil {
		p.Close()
		t.Fatal("expected error when starting without socket")
	}

	// Cleanup just in case
	os.Remove(cfg.SocketPath)
}
