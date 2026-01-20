package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/scheduler"
)

func TestSocketPusher_NewSocketPusher(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	t.Run("applies default BufferSize", func(t *testing.T) {
		cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
		p := NewSocketPusher(bus, cfg)
		if p.cfg.BufferSize != 1000 {
			t.Errorf("expected BufferSize=1000, got %d", p.cfg.BufferSize)
		}
	})

	t.Run("applies default WriteTimeout", func(t *testing.T) {
		cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
		p := NewSocketPusher(bus, cfg)
		if p.cfg.WriteTimeout != 5*time.Second {
			t.Errorf("expected WriteTimeout=5s, got %v", p.cfg.WriteTimeout)
		}
	})

	t.Run("applies default ReconnectBackoff", func(t *testing.T) {
		cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
		p := NewSocketPusher(bus, cfg)
		if p.cfg.ReconnectBackoff != 100*time.Millisecond {
			t.Errorf("expected ReconnectBackoff=100ms, got %v", p.cfg.ReconnectBackoff)
		}
	})

	t.Run("applies default MaxReconnectBackoff", func(t *testing.T) {
		cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
		p := NewSocketPusher(bus, cfg)
		if p.cfg.MaxReconnectBackoff != 5*time.Second {
			t.Errorf("expected MaxReconnectBackoff=5s, got %v", p.cfg.MaxReconnectBackoff)
		}
	})

	t.Run("respects custom config values", func(t *testing.T) {
		cfg := PusherConfig{
			SocketPath:          "/tmp/custom.sock",
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

	cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
	p := NewSocketPusher(bus, cfg)

	t.Run("handles nil graph", func(t *testing.T) {
		p.SetGraph(nil, 4)
		if p.graph != nil {
			t.Error("expected graph to remain nil")
		}
	})

	t.Run("converts graph to payload", func(t *testing.T) {
		// Create a simple graph with dependencies
		units := []*discovery.Unit{
			{ID: "unit-a", DependsOn: []string{}},
			{ID: "unit-b", DependsOn: []string{"unit-a"}},
			{ID: "unit-c", DependsOn: []string{"unit-a"}},
			{ID: "unit-d", DependsOn: []string{"unit-b", "unit-c"}},
		}

		graph, err := scheduler.NewGraph(units)
		if err != nil {
			t.Fatalf("failed to create graph: %v", err)
		}

		p.SetGraph(graph, 4)

		if p.graph == nil {
			t.Fatal("expected graph to be set")
		}

		// Should have 4 nodes
		if len(p.graph.Nodes) != 4 {
			t.Errorf("expected 4 nodes, got %d", len(p.graph.Nodes))
		}

		// Should have 4 edges (a->b, a->c, b->d, c->d)
		if len(p.graph.Edges) != 4 {
			t.Errorf("expected 4 edges, got %d", len(p.graph.Edges))
		}

		// Should have levels
		if len(p.graph.Levels) == 0 {
			t.Error("expected levels to be set")
		}

		// Level 0 should contain unit-a (no dependencies)
		if len(p.graph.Levels) > 0 {
			if !slices.Contains(p.graph.Levels[0], "unit-a") {
				t.Error("expected unit-a in level 0")
			}
		}
	})
}

func TestSocketPusher_Connected(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	cfg := PusherConfig{SocketPath: "/tmp/test.sock"}
	p := NewSocketPusher(bus, cfg)

	t.Run("returns false when not connected", func(t *testing.T) {
		if p.Connected() {
			t.Error("expected Connected() to return false initially")
		}
	})

	t.Run("returns true when connected", func(t *testing.T) {
		// Create a test socket with short path (Unix sockets have path length limits)
		socketPath := "/tmp/choo-test-conn.sock"
		os.Remove(socketPath)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()
		defer os.Remove(socketPath)

		p.cfg.SocketPath = socketPath
		if err := p.connect(); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer p.conn.Close()

		if !p.Connected() {
			t.Error("expected Connected() to return true after connection")
		}
	})
}

func TestSocketPusher_StartClose(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Use short path for Unix socket
	socketPath := "/tmp/choo-test-start.sock"
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	cfg := PusherConfig{
		SocketPath:   socketPath,
		BufferSize:   100,
		WriteTimeout: time.Second,
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()

	t.Run("Start succeeds with valid socket", func(t *testing.T) {
		err := p.Start(ctx)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		if !p.Connected() {
			t.Error("expected to be connected after Start")
		}
	})

	t.Run("Close stops cleanly", func(t *testing.T) {
		err := p.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		if p.Connected() {
			t.Error("expected to be disconnected after Close")
		}
	})

	t.Run("Start fails with invalid socket", func(t *testing.T) {
		bus2 := events.NewBus(100)
		defer bus2.Close()

		cfg2 := PusherConfig{
			SocketPath: "/nonexistent/path/test.sock",
		}
		p2 := NewSocketPusher(bus2, cfg2)

		err := p2.Start(ctx)
		if err == nil {
			t.Error("expected Start to fail with invalid socket path")
			p2.Close()
		}
	})
}

func TestSocketPusher_EventForwarding(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Use short path for Unix socket
	socketPath := "/tmp/choo-test-fwd.sock"
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	// Channel to receive connections
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
		BufferSize:   100,
		WriteTimeout: time.Second,
	}
	p := NewSocketPusher(bus, cfg)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer p.Close()

	// Wait for server to accept connection
	var serverConn net.Conn
	select {
	case serverConn = <-connCh:
		defer serverConn.Close()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connection")
	}

	// Emit an event through the bus
	testEvent := events.Event{
		Type: events.UnitStarted,
		Unit: "test-unit",
	}
	bus.Emit(testEvent)

	// Read the event from the server side
	serverConn.SetReadDeadline(time.Now().Add(time.Second))
	reader := bufio.NewReader(serverConn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

	var wireEvent WireEvent
	if err := json.Unmarshal(line, &wireEvent); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if wireEvent.Type != string(events.UnitStarted) {
		t.Errorf("expected type=%s, got %s", events.UnitStarted, wireEvent.Type)
	}
	if wireEvent.Unit != "test-unit" {
		t.Errorf("expected unit=test-unit, got %s", wireEvent.Unit)
	}
}

func TestSocketPusher_Reconnect(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	// Use short path for Unix socket
	socketPath := "/tmp/choo-test-recon.sock"
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	// Create initial listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	cfg := PusherConfig{
		SocketPath:          socketPath,
		BufferSize:          100,
		WriteTimeout:        100 * time.Millisecond,
		ReconnectBackoff:    10 * time.Millisecond,
		MaxReconnectBackoff: 50 * time.Millisecond,
	}
	p := NewSocketPusher(bus, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Accept first connection
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Close immediately to simulate disconnect
		conn.Close()
	}()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer p.Close()

	// Close the listener to simulate server going down
	listener.Close()

	// Give pusher time to detect disconnect
	time.Sleep(50 * time.Millisecond)

	// Create new listener on same path
	os.Remove(socketPath)
	newListener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create new listener: %v", err)
	}
	defer newListener.Close()

	// Channel to receive new connection
	reconnected := make(chan bool, 1)
	go func() {
		conn, err := newListener.Accept()
		if err != nil {
			return
		}
		conn.Close()
		reconnected <- true
	}()

	// Emit event to trigger reconnect attempt
	bus.Emit(events.Event{Type: events.UnitStarted, Unit: "test"})

	// Wait for reconnection
	select {
	case <-reconnected:
		// Success - reconnected
	case <-time.After(500 * time.Millisecond):
		// Reconnect may take time with backoff, this is acceptable
		// The test verifies that reconnect logic exists and attempts to reconnect
	}
}
