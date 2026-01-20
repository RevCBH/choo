package web

import (
	"encoding/json"
	"os"
	"time"
)

// Event represents a message received from the orchestrator via Unix socket.
// Events are sent as newline-delimited JSON.
type Event struct {
	Type    string          `json:"type"`
	Time    time.Time       `json:"time"`
	Unit    string          `json:"unit,omitempty"`
	Task    *int            `json:"task,omitempty"`
	PR      *int            `json:"pr,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

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

// OrchestratorPayload is the payload for orch.started events.
// Contains the dependency graph and orchestrator configuration.
type OrchestratorPayload struct {
	UnitCount   int        `json:"unit_count"`
	Parallelism int        `json:"parallelism"`
	Graph       *GraphData `json:"graph"`
}

// GraphData represents the dependency graph structure.
// Used for visualization in the web UI.
type GraphData struct {
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
	Levels [][]string  `json:"levels"`
}

// GraphNode represents a unit in the dependency graph.
type GraphNode struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

// GraphEdge represents a dependency between two units.
// From depends on To (From -> To means To must complete before From).
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// GraphPayload represents the dependency graph for visualization (pusher format)
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

// UnitState tracks the status of a single unit during orchestration.
type UnitState struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // "pending", "ready", "in_progress", "complete", "failed", "blocked"
	CurrentTask int       `json:"currentTask"`
	TotalTasks  int       `json:"totalTasks"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
}

// StateSnapshot is the response for GET /api/state.
// Provides the complete current state of the orchestration.
type StateSnapshot struct {
	Connected   bool         `json:"connected"`
	Status      string       `json:"status"` // "waiting", "running", "completed", "failed"
	StartedAt   *time.Time   `json:"startedAt,omitempty"`
	Parallelism int          `json:"parallelism,omitempty"`
	Units       []*UnitState `json:"units"`
	Summary     StateSummary `json:"summary"`
}

// StateSummary provides aggregate counts of unit statuses.
type StateSummary struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	InProgress int `json:"inProgress"`
	Complete   int `json:"complete"`
	Failed     int `json:"failed"`
	Blocked    int `json:"blocked"`
}

// Config holds server configuration.
type Config struct {
	// Addr is the HTTP listen address (default ":8080")
	Addr string

	// SocketPath is the Unix socket path (default ~/.choo/web.sock)
	SocketPath string
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
// Uses $XDG_RUNTIME_DIR/choo/web.sock if set, otherwise ~/.choo/web.sock
// This matches the default path used by the web server
func DefaultSocketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return xdg + "/choo/web.sock"
	}
	home, _ := os.UserHomeDir()
	return home + "/.choo/web.sock"
}
