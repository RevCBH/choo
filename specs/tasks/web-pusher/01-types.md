---
task: 1
status: pending
backpressure: "go build ./internal/web/..."
depends_on: []
---

# Pusher Types

**Parent spec**: `/specs/WEB-PUSHER.md`
**Task**: #1 of 3

## Objective

Define the wire protocol types for event serialization and graph visualization payload structures.

## Dependencies

### Task Dependencies
- None

### Package Dependencies
- `time` (standard library)

## Deliverables

### Files to Create

```
internal/web/
└── types.go    # CREATE: Wire protocol types
```

### Types to Implement

```go
package web

import "time"

// WireEvent is the JSON structure sent over the socket
// Maps closely to events.Event but with explicit JSON serialization
type WireEvent struct {
	Type    string    `json:"type"`
	Time    time.Time `json:"time"`
	Unit    string    `json:"unit,omitempty"`
	Task    *int      `json:"task,omitempty"`
	PR      *int      `json:"pr,omitempty"`
	Payload any       `json:"payload,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// GraphPayload represents the dependency graph for visualization
type GraphPayload struct {
	Nodes  []NodePayload `json:"nodes"`
	Edges  []EdgePayload `json:"edges"`
	Levels [][]string    `json:"levels"`
}

// NodePayload represents a unit node in the graph
type NodePayload struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Status   string   `json:"status"`
	Tasks    int      `json:"tasks"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// EdgePayload represents a dependency edge
type EdgePayload struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PusherConfig holds configuration for SocketPusher
type PusherConfig struct {
	// SocketPath is the Unix socket path to connect to
	SocketPath string

	// BufferSize is the event channel capacity (default: 1000)
	BufferSize int

	// WriteTimeout is the deadline for socket writes (default: 5s)
	WriteTimeout time.Duration

	// ReconnectBackoff is the initial retry delay (default: 100ms)
	ReconnectBackoff time.Duration

	// MaxReconnectBackoff is the maximum retry delay (default: 5s)
	MaxReconnectBackoff time.Duration
}

// DefaultPusherConfig returns sensible defaults
func DefaultPusherConfig() PusherConfig {
	return PusherConfig{
		SocketPath:          DefaultSocketPath(),
		BufferSize:          1000,
		WriteTimeout:        5 * time.Second,
		ReconnectBackoff:    100 * time.Millisecond,
		MaxReconnectBackoff: 5 * time.Second,
	}
}

// DefaultSocketPath returns the default Unix socket path
// Uses XDG_RUNTIME_DIR if available, otherwise /tmp
func DefaultSocketPath() string {
	// Implementation: check XDG_RUNTIME_DIR, fallback to /tmp/choo.sock
	return "/tmp/choo.sock"
}
```

### Implementation Notes

1. `WireEvent` mirrors `events.Event` for JSON serialization
2. `GraphPayload` provides visualization data for the web UI
3. `NodePayload.Status` values: "pending", "running", "completed", "failed", "blocked"
4. `DefaultSocketPath()` should check `os.Getenv("XDG_RUNTIME_DIR")` and use that if available

## Backpressure

### Validation Command

```bash
go build ./internal/web/...
```

### Must Pass
| Check | Assertion |
|-------|-----------|
| Package compiles | No build errors |
| Types exported | WireEvent, GraphPayload, NodePayload, EdgePayload, PusherConfig accessible |
| Defaults work | DefaultPusherConfig() returns valid config |

### CI Compatibility
- [x] No external API keys
- [x] No network access
- [x] Runs in <60 seconds

## NOT In Scope
- SocketPusher struct (task #2)
- Connection logic (task #2)
- CLI integration (task #3)
