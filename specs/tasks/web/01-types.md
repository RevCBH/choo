---
task: 1
status: pending
backpressure: "go build ./internal/web/..."
depends_on: []
---

# Define Shared Types

**Parent spec**: `/specs/WEB.md`
**Task**: #1 of 7 in implementation plan

## Objective

Define all shared types for the web package: Event, GraphData, UnitState, StateSnapshot, and related structures.

## Dependencies

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `encoding/json`
- `time`

## Deliverables

### Files to Create

```
internal/web/
└── types.go    # CREATE: All shared types for web package
```

### Types to Implement

```go
package web

import (
    "encoding/json"
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
```

## Backpressure

### Validation Command

```bash
go build ./internal/web/...
```

### Must Pass
- Code compiles without errors
- All types have proper JSON tags
- Types use appropriate Go idioms (pointers for optional fields, etc.)

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `json.RawMessage` for `Payload` to defer parsing until the event type is known
- Use pointer types (`*int`, `*time.Time`) for optional fields that may be omitted
- Status strings are intentionally not enums - kept as strings for JSON serialization simplicity
- `UnitState.StartedAt` uses `time.Time` (zero value is "not started") while `StateSnapshot.StartedAt` uses `*time.Time` for explicit null in JSON

## NOT In Scope

- Store logic (task #2)
- SSE hub types (task #3)
- Socket handling (task #4)
- HTTP handlers (task #5)
- Server lifecycle (task #6)
