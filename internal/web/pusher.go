package web

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/scheduler"
)

// SocketPusher forwards events to a Unix socket for web UI consumption
type SocketPusher struct {
	cfg     PusherConfig
	bus     *events.Bus
	conn    net.Conn
	mu      sync.RWMutex
	eventCh chan events.Event
	done    chan struct{}
	wg      sync.WaitGroup
	graph   *GraphPayload
}

// NewSocketPusher creates a pusher that will connect to the configured socket
// Does not connect until Start() is called
func NewSocketPusher(bus *events.Bus, cfg PusherConfig) *SocketPusher {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 5 * time.Second
	}
	if cfg.ReconnectBackoff <= 0 {
		cfg.ReconnectBackoff = 100 * time.Millisecond
	}
	if cfg.MaxReconnectBackoff <= 0 {
		cfg.MaxReconnectBackoff = 5 * time.Second
	}

	return &SocketPusher{
		cfg:     cfg,
		bus:     bus,
		eventCh: make(chan events.Event, cfg.BufferSize),
		done:    make(chan struct{}),
	}
}

// SetGraph configures the graph payload for initial handshake
// Must be called before Start() for graph data to be sent
func (p *SocketPusher) SetGraph(graph *scheduler.Graph, parallelism int) {
	if graph == nil {
		return
	}

	levels := graph.GetLevels()

	// Build nodes and edges from graph
	var nodes []NodePayload
	var edges []EdgePayload

	for _, level := range levels {
		for _, nodeID := range level {
			deps := graph.GetDependencies(nodeID)
			node := NodePayload{
				ID:        nodeID,
				Label:     nodeID,
				Status:    "pending",
				Tasks:     0, // Task count not available from graph alone
				DependsOn: deps,
			}
			nodes = append(nodes, node)

			// Add edges for each dependency
			for _, dep := range deps {
				edges = append(edges, EdgePayload{
					From: dep,
					To:   nodeID,
				})
			}
		}
	}

	p.graph = &GraphPayload{
		Nodes:  nodes,
		Edges:  edges,
		Levels: levels,
	}
}

// Start connects to the socket and begins forwarding events
// Subscribes to the event bus and runs the push loop in a goroutine
// Returns error if initial connection fails
func (p *SocketPusher) Start(ctx context.Context) error {
	// Attempt initial connection
	if err := p.connect(); err != nil {
		return err
	}

	// Subscribe to event bus
	p.bus.Subscribe(func(e events.Event) {
		select {
		case p.eventCh <- e:
			// Delivered
		default:
			// Channel full, drop event
		}
	})

	// Start push loop goroutine
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.pushLoop(ctx)
	}()

	// Send initial graph payload if set
	if p.graph != nil {
		wireEvent := WireEvent{
			Type:    "graph",
			Time:    time.Now(),
			Payload: p.graph,
		}
		// Ignore error for initial graph push - not critical
		_ = p.writeWireEvent(wireEvent)
	}

	return nil
}

// Close stops the pusher and releases resources
// Blocks until the push loop exits
func (p *SocketPusher) Close() error {
	close(p.done)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		return err
	}
	return nil
}

// Connected returns true if currently connected to the socket
func (p *SocketPusher) Connected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.conn != nil
}

// pushLoop reads from eventCh and writes to socket
// Handles reconnection with exponential backoff
func (p *SocketPusher) pushLoop(ctx context.Context) {
	backoff := p.cfg.ReconnectBackoff

	for {
		select {
		case <-p.done:
			return
		case <-ctx.Done():
			return
		case e := <-p.eventCh:
			if err := p.writeEvent(e); err != nil {
				// Connection failed, attempt reconnect with backoff
			reconnectLoop:
				for {
					select {
					case <-p.done:
						return
					case <-ctx.Done():
						return
					case <-time.After(backoff):
						if err := p.connect(); err != nil {
							// Exponential backoff
							backoff = min(backoff*2, p.cfg.MaxReconnectBackoff)
							continue
						}
						// Reconnected successfully, reset backoff
						backoff = p.cfg.ReconnectBackoff
						// Try to write the event again (ignore error, will retry on next event)
						_ = p.writeEvent(e)
						break reconnectLoop
					}
				}
			}
		}
	}
}

// connect establishes connection to the Unix socket
func (p *SocketPusher) connect() error {
	conn, err := net.Dial("unix", p.cfg.SocketPath)
	if err != nil {
		return err
	}

	p.mu.Lock()
	// Close existing connection if any
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn = conn
	p.mu.Unlock()

	return nil
}

// writeEvent sends a single event over the socket
func (p *SocketPusher) writeEvent(e events.Event) error {
	// Convert events.Event to WireEvent
	wireEvent := WireEvent{
		Type:    string(e.Type),
		Time:    e.Time,
		Unit:    e.Unit,
		Task:    e.Task,
		PR:      e.PR,
		Payload: e.Payload,
		Error:   e.Error,
	}

	return p.writeWireEvent(wireEvent)
}

// writeWireEvent sends a WireEvent over the socket
func (p *SocketPusher) writeWireEvent(wireEvent WireEvent) error {
	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()

	if conn == nil {
		return net.ErrClosed
	}

	// Set write deadline
	if err := conn.SetWriteDeadline(time.Now().Add(p.cfg.WriteTimeout)); err != nil {
		return err
	}

	// JSON encode with newline delimiter
	data, err := json.Marshal(wireEvent)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = conn.Write(data)
	return err
}
