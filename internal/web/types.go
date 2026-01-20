package web

import (
	"os"
	"time"
)

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
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Status    string   `json:"status"`
	Tasks     int      `json:"tasks"`
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
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		return xdgRuntime + "/choo.sock"
	}
	return "/tmp/choo.sock"
}
