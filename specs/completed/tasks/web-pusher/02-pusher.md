---
task: 2
status: complete
backpressure: "go test ./internal/web/... -run TestSocketPusher"
depends_on: [1]
---

# Socket Pusher Core

**Parent spec**: `/specs/WEB-PUSHER.md`
**Task**: #2 of 3

## Objective

Implement SocketPusher that connects to a Unix socket, subscribes to the event bus, and forwards events as newline-delimited JSON.

## Dependencies

### Task Dependencies
- Task #1: Types (WireEvent, GraphPayload, PusherConfig)

### Package Dependencies
- `choo/internal/events`
- `choo/internal/scheduler`
- `encoding/json`
- `net`
- `sync`
- `context`

## Deliverables

### Files to Create

```
internal/web/
└── pusher.go       # CREATE: SocketPusher implementation
└── pusher_test.go  # CREATE: Unit tests
```

### Types to Implement

```go
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
```

### Functions to Implement

```go
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
	// Convert scheduler.Graph to GraphPayload
	// Include nodes, edges, and levels
}

// Start connects to the socket and begins forwarding events
// Subscribes to the event bus and runs the push loop in a goroutine
// Returns error if initial connection fails
func (p *SocketPusher) Start(ctx context.Context) error {
	// 1. Attempt initial connection
	// 2. Subscribe to event bus
	// 3. Start push loop goroutine
	// 4. Send initial graph payload if set
}

// Close stops the pusher and releases resources
// Blocks until the push loop exits
func (p *SocketPusher) Close() error {
	// 1. Signal done
	// 2. Wait for goroutine
	// 3. Close connection
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
	// 1. Read events from channel
	// 2. Convert to WireEvent
	// 3. JSON encode with newline delimiter
	// 4. Write to socket with timeout
	// 5. Handle disconnection with backoff retry
}

// connect establishes connection to the Unix socket
func (p *SocketPusher) connect() error {
	// net.Dial("unix", p.cfg.SocketPath)
}

// writeEvent sends a single event over the socket
func (p *SocketPusher) writeEvent(e events.Event) error {
	// 1. Convert events.Event to WireEvent
	// 2. JSON marshal
	// 3. Write with newline
	// 4. Respect WriteTimeout
}
```

### Test Cases

```go
func TestSocketPusher_NewSocketPusher(t *testing.T) {
	// Test default config values are applied
}

func TestSocketPusher_SetGraph(t *testing.T) {
	// Test graph conversion from scheduler.Graph
}

func TestSocketPusher_Connected(t *testing.T) {
	// Test Connected() returns correct state
}

func TestSocketPusher_StartClose(t *testing.T) {
	// Test lifecycle with mock socket
}

func TestSocketPusher_EventForwarding(t *testing.T) {
	// Test events are forwarded correctly
	// Use a Unix socket pair for testing
}

func TestSocketPusher_Reconnect(t *testing.T) {
	// Test reconnection with backoff
}
```

### Implementation Notes

1. Use `net.Dial("unix", path)` for Unix socket connection
2. Events are newline-delimited JSON (`json.Encoder` with `\n` separator)
3. Write timeout prevents blocking on slow consumers
4. Reconnection uses exponential backoff: `min(backoff * 2, maxBackoff)`
5. Event bus subscription via `bus.Subscribe(handler)`
6. Graph conversion iterates over `graph.GetLevels()` and extracts nodes/edges

## Backpressure

### Validation Command

```bash
go test ./internal/web/... -run TestSocketPusher
```

### Must Pass
| Test | Assertion |
|-------|-----------|
| TestSocketPusher_NewSocketPusher | Default config applied |
| TestSocketPusher_SetGraph | Graph converted correctly |
| TestSocketPusher_Connected | State tracked correctly |
| TestSocketPusher_StartClose | Clean lifecycle |
| TestSocketPusher_EventForwarding | Events serialized and sent |

### CI Compatibility
- [x] No external API keys
- [x] No network access (Unix sockets are local)
- [x] Runs in <60 seconds

## NOT In Scope
- CLI flag integration (task #3)
- Web UI implementation
- HTTP/WebSocket transport (Unix socket only)
