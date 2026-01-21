# DAEMON-DB — SQLite Database Layer for Persistent State Storage

## Overview

DAEMON-DB provides the persistent storage layer for the daemon architecture, enabling full job resumability across process restarts. The database tracks workflow runs, individual work units, and a complete event log for debugging and replay.

The system uses SQLite with WAL mode for concurrent access, storing all state needed to resume interrupted workflows. When a daemon restarts, it queries the database for incomplete runs and units, then resumes execution from where it left off. The event log captures every state transition, enabling both debugging and potential event replay scenarios.

This component sits between the orchestration layer and the filesystem, providing ACID guarantees for state mutations while keeping the storage footprint minimal and portable.

```
┌─────────────────────────────────────────────────────────┐
│                    Orchestrator                         │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                      DAEMON-DB                          │
│  ┌───────────┐  ┌───────────┐  ┌───────────────────┐   │
│  │   runs    │  │   units   │  │      events       │   │
│  └───────────┘  └───────────┘  └───────────────────┘   │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
                 ┌────────────────┐
                 │  SQLite (WAL)  │
                 └────────────────┘
```

## Requirements

### Functional Requirements

1. Store and retrieve workflow runs with all associated metadata
2. Track individual work units within runs, including their status and branch information
3. Record events with per-run sequence numbers for ordered replay
4. Enforce unique constraint: one active run per branch/repo combination
5. Support cascading deletes when runs are removed
6. Provide indexed queries for status-based filtering
7. Enable schema migrations for forward compatibility
8. Support full state reconstruction after process restart

### Performance Requirements

| Metric | Target |
|--------|--------|
| Run insert latency | < 5ms |
| Unit status update | < 2ms |
| Event append | < 1ms |
| Query runs by status | < 10ms for 1000 runs |
| Database file size | < 100MB for 10,000 events |
| Concurrent readers | Unlimited (WAL mode) |

### Constraints

- Pure Go implementation required (no CGO dependencies)
- Single database file for portability
- Must handle unexpected process termination gracefully
- Foreign key enforcement required for referential integrity

## Design

### Module Structure

```
internal/daemon/db/
├── db.go           # Connection management and migrations
├── runs.go         # Run CRUD operations
├── units.go        # Unit CRUD operations
├── events.go       # Event logging and queries
├── types.go        # Status constants and record types
└── db_test.go      # Database tests
```

### Core Types

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

### Database Schema

```sql
-- Runs table: Top-level workflow executions
CREATE TABLE runs (
    id              TEXT PRIMARY KEY,
    feature_branch  TEXT NOT NULL,
    repo_path       TEXT NOT NULL,
    target_branch   TEXT NOT NULL,
    tasks_dir       TEXT NOT NULL,
    parallelism     INTEGER NOT NULL,
    status          TEXT NOT NULL,
    daemon_version  TEXT NOT NULL,
    started_at      DATETIME,
    completed_at    DATETIME,
    error           TEXT,
    config_json     TEXT,
    UNIQUE(feature_branch, repo_path)
);

-- Units table: Individual work units within a run
CREATE TABLE units (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    unit_id         TEXT NOT NULL,
    status          TEXT NOT NULL,
    branch          TEXT,
    worktree_path   TEXT,
    started_at      DATETIME,
    completed_at    DATETIME,
    error           TEXT,
    UNIQUE(run_id, unit_id)
);

-- Events table: Event log for replay and debugging
CREATE TABLE events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence        INTEGER NOT NULL,
    event_type      TEXT NOT NULL,
    unit_id         TEXT,
    payload_json    TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(run_id, sequence)
);

-- Indexes for common queries
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_units_run_id ON units(run_id);
CREATE INDEX idx_units_status ON units(status);
CREATE INDEX idx_events_run_id ON events(run_id);
CREATE INDEX idx_events_sequence ON events(run_id, sequence);
```

### API Surface

```go
// Connection management
func Open(path string) (*DB, error)
func (db *DB) Close() error

// Run operations
func (db *DB) CreateRun(run *Run) error
func (db *DB) GetRun(id string) (*Run, error)
func (db *DB) GetRunByBranch(featureBranch, repoPath string) (*Run, error)
func (db *DB) UpdateRunStatus(id string, status RunStatus, err *string) error
func (db *DB) ListRunsByStatus(status RunStatus) ([]*Run, error)
func (db *DB) ListIncompleteRuns() ([]*Run, error)
func (db *DB) DeleteRun(id string) error

// Unit operations
func (db *DB) CreateUnit(unit *UnitRecord) error
func (db *DB) GetUnit(id string) (*UnitRecord, error)
func (db *DB) UpdateUnitStatus(id string, status UnitStatus, err *string) error
func (db *DB) UpdateUnitBranch(id string, branch, worktreePath string) error
func (db *DB) ListUnitsByRun(runID string) ([]*UnitRecord, error)
func (db *DB) ListUnitsByStatus(runID string, status UnitStatus) ([]*UnitRecord, error)

// Event operations
func (db *DB) AppendEvent(runID string, eventType string, unitID *string, payload interface{}) error
func (db *DB) GetNextSequence(runID string) (int, error)
func (db *DB) ListEvents(runID string) ([]*EventRecord, error)
func (db *DB) ListEventsSince(runID string, sequence int) ([]*EventRecord, error)
```

## Implementation Notes

### Connection Management

The database uses `modernc.org/sqlite`, a pure Go SQLite implementation that avoids CGO dependencies. This simplifies cross-compilation and deployment.

```go
type DB struct {
    conn *sql.DB
}

func Open(path string) (*DB, error) {
    conn, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // Enable WAL mode for better concurrency
    if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
        return nil, fmt.Errorf("failed to enable WAL: %w", err)
    }

    // Enable foreign keys
    if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
        return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }

    db := &DB{conn: conn}

    if err := db.migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    return db, nil
}
```

### WAL Mode Considerations

WAL (Write-Ahead Logging) mode provides several benefits:
- Readers do not block writers
- Writers do not block readers
- Improved crash recovery
- Better performance for read-heavy workloads

However, WAL creates additional files (`-wal` and `-shm`) alongside the main database file. These must be preserved together for database integrity.

### ULID Generation

Run IDs use ULIDs (Universally Unique Lexicographically Sortable Identifiers) for:
- Time-sortable ordering without additional timestamp queries
- 128-bit uniqueness across distributed systems
- URL-safe string representation

```go
func NewRunID() string {
    return ulid.Make().String()
}
```

### Transaction Handling

Status updates that involve multiple tables should use transactions:

```go
func (db *DB) CompleteRun(id string, finalStatus RunStatus) error {
    tx, err := db.conn.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Update run status
    _, err = tx.Exec(
        "UPDATE runs SET status = ?, completed_at = ? WHERE id = ?",
        finalStatus, time.Now(), id,
    )
    if err != nil {
        return err
    }

    // Append completion event
    seq, err := db.getNextSequenceInTx(tx, id)
    if err != nil {
        return err
    }

    _, err = tx.Exec(
        "INSERT INTO events (run_id, sequence, event_type) VALUES (?, ?, ?)",
        id, seq, "run_completed",
    )
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### Error Handling

Database errors should be wrapped with context to aid debugging:

```go
if err != nil {
    return fmt.Errorf("failed to update run %s status to %s: %w", id, status, err)
}
```

### Nullable Fields

Fields that may be null use pointer types (`*string`, `*time.Time`). The `database/sql` package's `NullString` and `NullTime` types can also be used, but pointer types provide cleaner Go APIs.

## Testing Strategy

### Unit Tests

```go
func TestRunLifecycle(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    run := &Run{
        ID:            ulid.Make().String(),
        FeatureBranch: "feature/test",
        RepoPath:      "/tmp/repo",
        TargetBranch:  "main",
        TasksDir:      "/tmp/tasks",
        Parallelism:   4,
        Status:        RunStatusPending,
        ConfigJSON:    "{}",
    }

    // Create
    err := db.CreateRun(run)
    require.NoError(t, err)

    // Read
    retrieved, err := db.GetRun(run.ID)
    require.NoError(t, err)
    assert.Equal(t, run.FeatureBranch, retrieved.FeatureBranch)
    assert.Equal(t, RunStatusPending, retrieved.Status)

    // Update status
    err = db.UpdateRunStatus(run.ID, RunStatusRunning, nil)
    require.NoError(t, err)

    retrieved, err = db.GetRun(run.ID)
    require.NoError(t, err)
    assert.Equal(t, RunStatusRunning, retrieved.Status)

    // Complete with error
    errMsg := "something went wrong"
    err = db.UpdateRunStatus(run.ID, RunStatusFailed, &errMsg)
    require.NoError(t, err)

    retrieved, err = db.GetRun(run.ID)
    require.NoError(t, err)
    assert.Equal(t, RunStatusFailed, retrieved.Status)
    assert.Equal(t, &errMsg, retrieved.Error)
}

func TestUniqueConstraintBranchRepo(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    run1 := &Run{
        ID:            ulid.Make().String(),
        FeatureBranch: "feature/same",
        RepoPath:      "/same/repo",
        TargetBranch:  "main",
        TasksDir:      "/tmp/tasks",
        Parallelism:   4,
        Status:        RunStatusPending,
        ConfigJSON:    "{}",
    }

    run2 := &Run{
        ID:            ulid.Make().String(),
        FeatureBranch: "feature/same",
        RepoPath:      "/same/repo",
        TargetBranch:  "main",
        TasksDir:      "/tmp/tasks",
        Parallelism:   4,
        Status:        RunStatusPending,
        ConfigJSON:    "{}",
    }

    err := db.CreateRun(run1)
    require.NoError(t, err)

    err = db.CreateRun(run2)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

func TestCascadeDelete(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    runID := ulid.Make().String()
    run := &Run{
        ID:            runID,
        FeatureBranch: "feature/cascade",
        RepoPath:      "/tmp/repo",
        TargetBranch:  "main",
        TasksDir:      "/tmp/tasks",
        Parallelism:   4,
        Status:        RunStatusPending,
        ConfigJSON:    "{}",
    }
    require.NoError(t, db.CreateRun(run))

    unit := &UnitRecord{
        ID:     runID + "_unit1",
        RunID:  runID,
        UnitID: "unit1",
        Status: string(UnitStatusPending),
    }
    require.NoError(t, db.CreateUnit(unit))

    require.NoError(t, db.AppendEvent(runID, "test_event", nil, nil))

    // Delete run
    require.NoError(t, db.DeleteRun(runID))

    // Verify cascade
    units, err := db.ListUnitsByRun(runID)
    require.NoError(t, err)
    assert.Empty(t, units)

    events, err := db.ListEvents(runID)
    require.NoError(t, err)
    assert.Empty(t, events)
}

func TestEventSequencing(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    runID := createTestRun(t, db)

    // Append multiple events
    for i := 0; i < 5; i++ {
        err := db.AppendEvent(runID, fmt.Sprintf("event_%d", i), nil, nil)
        require.NoError(t, err)
    }

    events, err := db.ListEvents(runID)
    require.NoError(t, err)
    assert.Len(t, events, 5)

    // Verify sequence ordering
    for i, event := range events {
        assert.Equal(t, i+1, event.Sequence)
    }

    // Test ListEventsSince
    since, err := db.ListEventsSince(runID, 3)
    require.NoError(t, err)
    assert.Len(t, since, 2)
    assert.Equal(t, 4, since[0].Sequence)
    assert.Equal(t, 5, since[1].Sequence)
}

func setupTestDB(t *testing.T) *DB {
    t.Helper()
    db, err := Open(":memory:")
    require.NoError(t, err)
    return db
}

func createTestRun(t *testing.T, db *DB) string {
    t.Helper()
    id := ulid.Make().String()
    run := &Run{
        ID:            id,
        FeatureBranch: "feature/test-" + id[:8],
        RepoPath:      "/tmp/repo",
        TargetBranch:  "main",
        TasksDir:      "/tmp/tasks",
        Parallelism:   4,
        Status:        RunStatusPending,
        ConfigJSON:    "{}",
    }
    require.NoError(t, db.CreateRun(run))
    return id
}
```

### Integration Tests

- Resume interrupted run: Start a run, kill the process, restart, verify run resumes
- Concurrent access: Multiple goroutines reading/writing simultaneously
- Large event logs: Append 100,000 events, verify query performance
- Schema migration: Open database with older schema version, verify migration

### Manual Testing

- [ ] Create database in new location, verify tables created
- [ ] Create run, verify queryable immediately
- [ ] Add units to run, verify foreign key constraint
- [ ] Delete run, verify cascade removes units and events
- [ ] Open existing database, verify data persists
- [ ] Corrupt WAL file, verify recovery behavior

## Design Decisions

### Why SQLite over embedded key-value stores?

SQLite provides:
- Relational integrity with foreign keys
- Rich query capabilities for filtering and aggregation
- Built-in schema migrations
- Well-tested ACID guarantees
- Human-readable with standard tools

Key-value stores like BoltDB or BadgerDB would require manual index management and lack relational constraints.

### Why pure Go SQLite (modernc.org/sqlite)?

The `modernc.org/sqlite` package is a pure Go port of SQLite, avoiding CGO:
- Simpler cross-compilation
- Easier deployment (no shared library dependencies)
- Better compatibility with Go tooling
- Slightly slower than CGO SQLite, but sufficient for this workload

### Why ULIDs for Run IDs?

ULIDs provide:
- Time-based sorting without separate timestamp queries
- Collision resistance comparable to UUIDs
- URL-safe string representation
- Monotonic ordering when generated in the same millisecond

UUIDs would require additional timestamp columns for chronological ordering.

### Why composite key for Unit IDs?

The unit ID format `{run_id}_{unit_id}` provides:
- Globally unique primary key
- Easy extraction of parent run
- Natural grouping in sorted output
- Simpler joins in queries

### Why event sequence numbers per run?

Per-run sequence numbers (rather than global auto-increment):
- Enable ordered replay of a specific run
- Support partial event queries ("events since sequence N")
- Avoid gaps in sequence when runs are deleted
- Simplify distributed event processing

## Future Enhancements

1. **Event compaction**: Archive old events to separate storage after run completion
2. **Read replicas**: Support read-only database copies for monitoring
3. **Metrics export**: Prometheus metrics for run/unit status counts
4. **Full-text search**: Index error messages for debugging queries
5. **Event replay**: Reconstruct state by replaying events from a checkpoint
6. **Database encryption**: Support SQLCipher for encrypted storage
7. **Connection pooling**: Optimize for high-concurrency scenarios

## References

- SQLite WAL mode: https://www.sqlite.org/wal.html
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- ULID specification: https://github.com/ulid/spec
- Related specs: DAEMON-ORCHESTRATOR (uses DB for state), DAEMON-API (exposes run queries)
