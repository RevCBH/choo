---
task: 1
status: complete
backpressure: "go build ./internal/daemon/db/..."
depends_on: []
---

# Types and Constants

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define all status constants and record types for the database layer, establishing the data model for runs, units, and events.

## Dependencies

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
└── types.go    # CREATE: Status constants and record types
```

### Types to Implement

```go
// RunStatus represents the lifecycle state of a workflow run
type RunStatus string

const (
    RunStatusPending   RunStatus = "pending"
    RunStatusRunning   RunStatus = "running"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
    RunStatusCancelled RunStatus = "cancelled"
)

// Run represents a top-level workflow execution
type Run struct {
    ID            string     `db:"id"`             // ULID for sortable unique IDs
    FeatureBranch string     `db:"feature_branch"` // Branch being worked on
    RepoPath      string     `db:"repo_path"`      // Absolute path to repository
    TargetBranch  string     `db:"target_branch"`  // Branch to merge into (e.g., main)
    TasksDir      string     `db:"tasks_dir"`      // Directory containing task definitions
    Parallelism   int        `db:"parallelism"`    // Max concurrent units
    Status        RunStatus  `db:"status"`         // Current run status
    DaemonVersion string     `db:"daemon_version"` // Daemon version that created this run
    StartedAt     *time.Time `db:"started_at"`     // When execution began
    CompletedAt   *time.Time `db:"completed_at"`   // When execution finished
    Error         *string    `db:"error"`          // Error message if failed
    ConfigJSON    string     `db:"config_json"`    // Serialized Config struct
}

// UnitStatus represents the lifecycle state of a work unit
type UnitStatus string

const (
    UnitStatusPending   UnitStatus = "pending"
    UnitStatusRunning   UnitStatus = "running"
    UnitStatusCompleted UnitStatus = "completed"
    UnitStatusFailed    UnitStatus = "failed"
)

// UnitRecord represents an individual work unit within a run
type UnitRecord struct {
    ID           string     `db:"id"`            // Composite: run_id + "_" + unit_id
    RunID        string     `db:"run_id"`        // Parent run reference
    UnitID       string     `db:"unit_id"`       // Unit identifier within run
    Status       string     `db:"status"`        // Current unit status
    Branch       *string    `db:"branch"`        // Git branch for this unit's work
    WorktreePath *string    `db:"worktree_path"` // Path to git worktree
    StartedAt    *time.Time `db:"started_at"`    // When unit execution began
    CompletedAt  *time.Time `db:"completed_at"`  // When unit execution finished
    Error        *string    `db:"error"`         // Error message if failed
}

// EventRecord represents a logged event for replay and debugging
type EventRecord struct {
    ID          int64     `db:"id"`           // Auto-increment primary key
    RunID       string    `db:"run_id"`       // Parent run reference
    Sequence    int       `db:"sequence"`     // Per-run sequence number
    EventType   string    `db:"event_type"`   // Event classification
    UnitID      *string   `db:"unit_id"`      // Associated unit (optional)
    PayloadJSON *string   `db:"payload_json"` // Event-specific data
    CreatedAt   time.Time `db:"created_at"`   // When event was recorded
}
```

### Functions to Implement

```go
// NewRunID generates a new ULID-based run ID
func NewRunID() string {
    // Uses oklog/ulid for time-sortable unique IDs
}

// MakeUnitRecordID creates the composite ID for a unit record
func MakeUnitRecordID(runID, unitID string) string {
    // Returns runID + "_" + unitID
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/daemon/db/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |

## NOT In Scope

- Database connection (Task #2)
- CRUD operations (Tasks #3-5)
- Tests beyond compilation (Task #6)
