# HISTORY-TYPES - Canonical Type Definitions for History System

## Overview

This spec defines the **canonical types** shared across the history system components: Daemon, Store, API, and UI. All other specs reference these definitions rather than redefining them, ensuring consistency across the implementation.

## Core Types

### Run

Represents a single orchestration run.

```go
// Run represents an orchestration run
type Run struct {
    // ID is the unique run identifier (e.g., "run_20250120_143052_a1b2")
    ID string `json:"id"`

    // RepoPath is the canonical repository path for scoping
    RepoPath string `json:"repo_path"`

    // StartedAt is when the run began
    StartedAt time.Time `json:"started_at"`

    // CompletedAt is when the run finished (nil if still running)
    CompletedAt *time.Time `json:"completed_at,omitempty"`

    // Status is the run state
    Status RunStatus `json:"status"`

    // Parallelism is the max concurrent units configured
    Parallelism int `json:"parallelism"`

    // TotalUnits is the count of units in the run
    TotalUnits int `json:"total_units"`

    // CompletedUnits is the count of successfully finished units
    CompletedUnits int `json:"completed_units"`

    // FailedUnits is the count of units that failed
    FailedUnits int `json:"failed_units"`

    // BlockedUnits is the count of units blocked by dependencies
    BlockedUnits int `json:"blocked_units"`

    // Error contains the error message if status is failed
    Error string `json:"error,omitempty"`

    // TasksDir is the path to the tasks directory
    TasksDir string `json:"tasks_dir,omitempty"`

    // DryRun indicates this was a dry-run (no actual execution)
    DryRun bool `json:"dry_run"`
}
```

### RunStatus

```go
type RunStatus string

const (
    RunStatusRunning   RunStatus = "running"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
    RunStatusStopped   RunStatus = "stopped"
)
```

### RunConfig

Input for creating a new run.

```go
type RunConfig struct {
    // ID is the unique run identifier
    ID string `json:"id"`

    // RepoPath is the canonical repository path
    RepoPath string `json:"repo_path"`

    // Parallelism is the max concurrent units
    Parallelism int `json:"parallelism"`

    // TotalUnits is the count of units
    TotalUnits int `json:"total_units"`

    // TasksDir is the path to tasks
    TasksDir string `json:"tasks_dir"`

    // DryRun indicates this is a dry-run
    DryRun bool `json:"dry_run"`

    // Graph is the optional dependency graph
    Graph *GraphData `json:"graph,omitempty"`
}
```

### RunResult

Input for completing a run.

```go
type RunResult struct {
    // Status is the final run status
    Status RunStatus `json:"status"`

    // CompletedUnits is the final count
    CompletedUnits int `json:"completed_units"`

    // FailedUnits is the final count
    FailedUnits int `json:"failed_units"`

    // BlockedUnits is the final count
    BlockedUnits int `json:"blocked_units"`

    // Error contains failure reason if status is failed
    Error string `json:"error,omitempty"`
}
```

## Event Types

### EventRecord

Input format for inserting events (used by CLI when sending to daemon).

```go
type EventRecord struct {
    // Time is when the event occurred
    Time time.Time `json:"time"`

    // Type is the event type (see Event Type Constants)
    Type string `json:"type"`

    // Unit is the unit ID this event relates to (empty for run-level events)
    Unit string `json:"unit,omitempty"`

    // Task is the task number within the unit (nil if not task-related)
    Task *int `json:"task,omitempty"`

    // PR is the pull request number (nil if not PR-related)
    PR *int `json:"pr,omitempty"`

    // Payload contains event-specific data (JSON-encoded)
    Payload json.RawMessage `json:"payload,omitempty"`

    // Error contains error message if this is a failure event
    Error string `json:"error,omitempty"`
}
```

### StoredEvent

Output format when querying events from the database.

```go
type StoredEvent struct {
    // ID is the auto-incremented database ID
    ID int64 `json:"id"`

    // RunID is the run this event belongs to
    RunID string `json:"run_id"`

    // Seq is the sequence number for ordering within the run
    Seq int64 `json:"seq"`

    // Time is when the event occurred
    Time time.Time `json:"time"`

    // Type is the event type
    Type string `json:"type"`

    // Unit is the unit ID
    Unit string `json:"unit,omitempty"`

    // Task is the task number
    Task *int `json:"task,omitempty"`

    // PR is the PR number
    PR *int `json:"pr,omitempty"`

    // Payload is the JSON-encoded event data
    Payload json.RawMessage `json:"payload,omitempty"`

    // Error contains error message
    Error string `json:"error,omitempty"`
}
```

### Event Type Constants

Canonical event type strings used throughout the system.

```go
const (
    // Run lifecycle events
    EventRunStarted   = "run.started"
    EventRunStopped   = "run.stopped"   // Payload: {"reason": "user_interrupt"|"error"|...}
    EventRunResumed   = "run.resumed"   // Payload: {"resumed_from_seq": N}
    EventRunCompleted = "run.completed"
    EventRunFailed    = "run.failed"

    // Unit lifecycle events
    EventUnitQueued    = "unit.queued"
    EventUnitStarted   = "unit.started"
    EventUnitCompleted = "unit.completed"
    EventUnitFailed    = "unit.failed"
    EventUnitBlocked   = "unit.blocked"
    EventUnitSkipped   = "unit.skipped"

    // Task events (within units)
    EventTaskStarted   = "task.started"
    EventTaskCompleted = "task.completed"
    EventTaskFailed    = "task.failed"

    // PR events
    EventPRCreated = "pr.created"
    EventPRMerged  = "pr.merged"
    EventPRFailed  = "pr.failed"
)
```

## Graph Types

### GraphData

Dependency graph for a run, used for visualization.

```go
type GraphData struct {
    // Nodes is the list of graph nodes
    Nodes []GraphNode `json:"nodes"`

    // Edges is the list of graph edges
    Edges []GraphEdge `json:"edges"`

    // Levels is the level assignments for layout (array of arrays of node IDs)
    Levels [][]string `json:"levels"`
}
```

### GraphNode

```go
type GraphNode struct {
    // ID is the unique node identifier (unit name)
    ID string `json:"id"`

    // Level is the dependency level (0 = no dependencies)
    Level int `json:"level"`

    // Status is the current node status (for visualization)
    Status string `json:"status,omitempty"`
}
```

### GraphEdge

```go
type GraphEdge struct {
    // From is the source node ID (dependency)
    From string `json:"from"`

    // To is the target node ID (dependent)
    To string `json:"to"`
}
```

## Pagination Types

All list endpoints use **limit/offset** pagination (not page/pageSize).

### ListOptions

Controls pagination for run listings.

```go
type ListOptions struct {
    // Limit is the max number of runs to return (default: 50, max: 100)
    Limit int `json:"limit"`

    // Offset is the number of runs to skip (default: 0)
    Offset int `json:"offset"`

    // Status filters by run status (empty for all)
    Status RunStatus `json:"status,omitempty"`
}
```

### EventListOptions

Controls pagination and filtering for event listings.

```go
type EventListOptions struct {
    // Limit is the max number of events to return (default: 100, max: 1000)
    Limit int `json:"limit"`

    // Offset is the number of events to skip (default: 0)
    Offset int `json:"offset"`

    // AfterSeq returns only events with seq > this value (for polling)
    AfterSeq int64 `json:"after_seq,omitempty"`

    // Type filters by event type prefix (e.g., "unit" matches all unit.* events)
    Type string `json:"type,omitempty"`

    // Unit filters by specific unit ID
    Unit string `json:"unit,omitempty"`
}
```

## Response Types

### RunList

Paginated response for listing runs.

```go
type RunList struct {
    // Runs is the list of runs
    Runs []Run `json:"runs"`

    // Total is the total count matching the query (for pagination UI)
    Total int `json:"total"`

    // HasMore indicates more results exist beyond this page
    HasMore bool `json:"has_more"`
}
```

### EventList

Paginated response for listing events.

```go
type EventList struct {
    // Events is the list of events
    Events []StoredEvent `json:"events"`

    // Total is the total count matching the query
    Total int `json:"total"`

    // HasMore indicates more results exist
    HasMore bool `json:"has_more"`
}
```

## API Error Response

Standard error response format for all API endpoints.

```go
type APIError struct {
    // Error is the human-readable error message
    Error string `json:"error"`

    // Code is the machine-readable error code
    Code string `json:"code,omitempty"`

    // Details provides additional context
    Details string `json:"details,omitempty"`
}
```

### Error Codes

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `MISSING_PARAM` | 400 | Required parameter missing |
| `INVALID_JSON` | 400 | Request body not valid JSON |
| `INVALID_PARAM` | 400 | Parameter has invalid value |
| `NOT_FOUND` | 404 | Resource does not exist |
| `ALREADY_EXISTS` | 409 | Resource already exists |
| `INTERNAL` | 500 | Internal server error |

## Run ID Format

Run IDs follow a predictable format for sortability and debugging:

```
run_YYYYMMDD_HHMMSS_XXXX
```

Where:
- `YYYYMMDD_HHMMSS` is the UTC timestamp when the run started
- `XXXX` is 4 random hex characters for uniqueness

Example: `run_20250120_143052_a1b2`

```go
func GenerateRunID() string {
    now := time.Now().UTC()
    suffix := make([]byte, 2)
    rand.Read(suffix)
    return fmt.Sprintf("run_%s_%s",
        now.Format("20060102_150405"),
        hex.EncodeToString(suffix),
    )
}
```

## Sensitive Data Redaction

Event payloads are redacted before storage using an **allow-list** approach. Only explicitly safe fields are preserved; all others are replaced with `[REDACTED]`.

### Safe Payload Fields

```go
var safePayloadFields = map[string]bool{
    // File/path information
    "file":     true,
    "path":     true,
    "worktree": true,

    // Git information
    "branch":    true,
    "commit":    true,
    "pr_number": true,

    // Status/metrics
    "status":      true,
    "duration":    true,
    "duration_ms": true,
    "exit_code":   true,

    // Resume tracking
    "reason":          true,
    "resumed_from_seq": true,
}
```

Fields NOT in this list (e.g., `token`, `api_key`, `secret`, `password`, `credentials`) are automatically redacted.

## References

This spec is referenced by:
- [DAEMON.md](./DAEMON.md) - Daemon process lifecycle
- [HISTORY-STORE.md](./HISTORY-STORE.md) - SQLite storage layer
- [HISTORY-API.md](./HISTORY-API.md) - HTTP API handlers
- [HISTORY-UI.md](./HISTORY-UI.md) - Frontend visualization
