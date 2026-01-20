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
	levels := graph.GetLevels()

	// Build nodes and edges from graph
	var nodes []NodePayload
	var edges []EdgePayload

	for _, level := range levels {
		for _, nodeID := range level {
			deps := graph.GetDependencies(nodeID)
			nodes = append(nodes, NodePayload{
				ID:        nodeID,
				Label:     nodeID,
				Status:    "pending",
				Tasks:     0,
				DependsOn: deps,
			})

			// Add edges for each dependency
			for _, dep := range deps {
				edges = append(edges, EdgePayload{
					From: nodeID,
					To:   dep,
				})
			}
		}
	}

	// Build levels as string slices
	levelStrings := make([][]string, len(levels))
	for i, level := range levels {
		levelStrings[i] = make([]string, len(level))
		copy(levelStrings[i], level)
	}

	p.graph = &GraphPayload{
		Nodes:  nodes,
		Edges:  edges,
		Levels: levelStrings,
	}
}

// Start connects to the socket and begins forwarding events
// Subscribes to the event bus and runs the push loop in a goroutine
// Returns error if initial connection fails
func (p *SocketPusher) Start(ctx context.Context) error {
	// 1. Attempt initial connection
	if err := p.connect(); err != nil {
		return err
	}

	// 2. Subscribe to event bus
	p.bus.Subscribe(func(e events.Event) {
		select {
		case p.eventCh <- e:
		default:
			// Buffer full, drop event
		}
	})

	// 3. Start push loop goroutine
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.pushLoop(ctx)
	}()

	// 4. Send initial graph payload if set
	if p.graph != nil {
		graphEvent := WireEvent{
			Type:    string(events.OrchStarted),
			Time:    time.Now(),
			Payload: p.graph,
		}
		p.writeWireEvent(graphEvent)
	}

	return nil
}

// Close stops the pusher and releases resources
// Blocks until the push loop exits
func (p *SocketPusher) Close() error {
	// 1. Signal done
	close(p.done)

	// 2. Wait for goroutine
	p.wg.Wait()

	// 3. Close connection
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
				// Handle disconnection with backoff retry
				for {
					select {
					case <-p.done:
						return
					case <-ctx.Done():
						return
					default:
					}

					time.Sleep(backoff)
					backoff = min(backoff*2, p.cfg.MaxReconnectBackoff)

					if err := p.connect(); err == nil {
						backoff = p.cfg.ReconnectBackoff
						// Retry writing the event
						if err := p.writeEvent(e); err == nil {
							break
						}
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
	p.conn = conn
	p.mu.Unlock()

	return nil
}

// writeEvent sends a single event over the socket
func (p *SocketPusher) writeEvent(e events.Event) error {
	// 1. Convert events.Event to WireEvent
	wire := WireEvent{
		Type:    string(e.Type),
		Time:    e.Time,
		Unit:    e.Unit,
		Task:    e.Task,
		PR:      e.PR,
		Payload: e.Payload,
		Error:   e.Error,
	}

	return p.writeWireEvent(wire)
}

// writeWireEvent sends a WireEvent over the socket
func (p *SocketPusher) writeWireEvent(wire WireEvent) error {
	// 2. JSON marshal
	data, err := json.Marshal(wire)
	if err != nil {
		return err
	}

	// 3. Write with newline
	data = append(data, '\n')

	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()

	if conn == nil {
		return net.ErrClosed
	}

	// 4. Respect WriteTimeout
	if err := conn.SetWriteDeadline(time.Now().Add(p.cfg.WriteTimeout)); err != nil {
		return err
	}

	_, err = conn.Write(data)
	if err != nil {
		// Mark connection as closed
		p.mu.Lock()
		p.conn = nil
		p.mu.Unlock()
	}

	return err
}
