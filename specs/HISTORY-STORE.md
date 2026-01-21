# HISTORY-STORE - SQLite Storage Layer for Run History

## Overview

The History Store provides SQLite-based persistence for orchestration runs and their events. It enables querying past runs, viewing execution history, and analyzing patterns across multiple orchestration sessions. The store uses WAL (Write-Ahead Logging) mode for concurrent read performance while the daemon writes.

The database is located at `~/.choo/history.db` and stores runs, events, and graph data scoped per repository. The handler component sends events to the daemon rather than writing directly to the database, enabling a clean separation between event production (CLI) and persistence (daemon).

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           History System                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌──────────────┐          ┌──────────────┐          ┌──────────────┐ │
│   │    Events    │          │   Handler    │          │    Daemon    │ │
│   │     Bus      │─────────▶│  (in CLI)    │─────────▶│   Client     │ │
│   └──────────────┘          └──────────────┘          └──────────────┘ │
│                                                               │         │
│                                                               ▼         │
│                                                       ┌──────────────┐ │
│                                                       │    Store     │ │
│                                                       │  (SQLite)    │ │
│                                                       └──────────────┘ │
│                                                               │         │
│                                                               ▼         │
│                                                       ┌──────────────┐ │
│                                                       │ ~/.choo/     │ │
│                                                       │ history.db   │ │
│                                                       └──────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Store run metadata including ID, repo path, timestamps, status, and configuration
2. Store events with sequence numbers for ordering, preserving full execution timeline
3. Store dependency graph data (nodes, edges, levels) for visualization
4. Support repository scoping - queries filter by canonical repo path
5. Enable WAL mode on connection for concurrent read performance
6. Track run lifecycle states: running, completed, failed, stopped
7. Support resume by appending events to existing runs with `run.stopped` and `run.resumed` markers
8. Auto-cleanup runs older than configurable retention period (default: 90 days)
9. Redact sensitive data (API keys, tokens) before storage
10. Provide pagination for listing runs and events

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event insert latency | <5ms |
| Run list query | <50ms for 100 runs |
| Event list query | <100ms for 1000 events |
| Database file growth | <10MB per 1000 runs |
| Concurrent readers | Unlimited (WAL mode) |

### Constraints

- Single database file at `~/.choo/history.db` shared across all repos
- SQLite with WAL mode (requires filesystem that supports shared locks)
- Handler sends to daemon; daemon owns database writes
- No external database dependencies - pure SQLite
- Events are append-only (no updates or deletes during a run)

## Design

### Shared Types

All shared types (Run, RunStatus, RunConfig, RunResult, EventRecord, StoredEvent, GraphData, ListOptions, EventListOptions, RunList, EventList) are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). This spec references those canonical definitions.

### Module Structure

```
internal/history/
├── store.go      # SQLite operations with WAL mode
├── types.go      # Data types (implements HISTORY-TYPES.md definitions)
├── migrate.go    # Schema creation with version management
└── handler.go    # Event handler that sends to daemon (not direct DB writes)
```

### Store-Specific Types

All shared types are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). The store package implements those types.

```go
// internal/history/store.go

// Store provides SQLite storage for run history
type Store struct {
    // db is the SQLite database connection
    db *sql.DB

    // dbPath is the path to the database file
    dbPath string
}
```

```go
// internal/history/handler.go

// Handler sends events to the daemon for storage
type Handler struct {
    // client is the daemon client for sending events
    client *daemon.Client

    // runID is the current run's ID
    runID string

    // seq is the atomic sequence counter
    seq atomic.Int64
}
```

### API Surface

```go
// internal/history/store.go

// NewStore creates a new history store at the given path
// Enables WAL mode and creates schema if needed
func NewStore(dbPath string) (*Store, error)

// Close closes the database connection
func (s *Store) Close() error

// CreateRun inserts a new run record and returns the run ID
func (s *Store) CreateRun(cfg RunConfig) (string, error)

// InsertEvent appends an event to a run
func (s *Store) InsertEvent(runID string, event EventRecord) error

// AppendResumeMarker adds a run.resumed event for resume tracking
func (s *Store) AppendResumeMarker(runID string) error

// CompleteRun updates a run with its final result
func (s *Store) CompleteRun(runID string, result RunResult) error

// GetRun retrieves a single run by ID
func (s *Store) GetRun(runID string) (*Run, error)

// ListRuns returns runs for a repository with pagination
func (s *Store) ListRuns(repoPath string, opts ListOptions) (*RunList, error)

// GetRunEvents returns events for a run with pagination
func (s *Store) GetRunEvents(runID string, opts EventListOptions) (*EventList, error)

// GetGraph retrieves the dependency graph for a run
func (s *Store) GetGraph(runID string) (*GraphData, error)

// SaveGraph stores the dependency graph for a run
func (s *Store) SaveGraph(runID string, nodes, edges, levels string) error

// DeleteOldRuns removes runs older than the given duration
func (s *Store) DeleteOldRuns(olderThan time.Duration) (int, error)
```

```go
// internal/history/migrate.go

// Migrate creates or updates the database schema
func Migrate(db *sql.DB) error

// SchemaVersion returns the current schema version
func SchemaVersion(db *sql.DB) (int, error)
```

```go
// internal/history/handler.go

// NewHandler creates a handler that sends events to the daemon
func NewHandler(client *daemon.Client, runID string) *Handler

// Handle implements events.Handler interface
func (h *Handler) Handle(e events.Event)

// Seq returns the current sequence number
func (h *Handler) Seq() int64
```

### Database Schema

```sql
-- Enable WAL mode on connection
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

-- Runs table stores orchestration run metadata
CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,              -- "run_20250120_143052_a1b2"
    repo_path TEXT NOT NULL,          -- canonical repo path for scoping
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    status TEXT NOT NULL DEFAULT 'running',  -- running/completed/failed/stopped
    parallelism INTEGER NOT NULL,
    total_units INTEGER NOT NULL,
    completed_units INTEGER DEFAULT 0,
    failed_units INTEGER DEFAULT 0,
    blocked_units INTEGER DEFAULT 0,
    error TEXT,
    tasks_dir TEXT,
    dry_run BOOLEAN DEFAULT FALSE
);

-- Events table stores individual run events
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,             -- sequence for ordering
    time DATETIME NOT NULL,
    type TEXT NOT NULL,               -- "unit.started", "run.stopped", etc.
    unit TEXT,
    task INTEGER,
    pr INTEGER,
    payload TEXT,                     -- JSON (redacted for sensitive data)
    error TEXT
);

-- Graphs table stores dependency graph data
CREATE TABLE IF NOT EXISTS graphs (
    run_id TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    nodes TEXT NOT NULL,              -- JSON array
    edges TEXT NOT NULL,
    levels TEXT NOT NULL
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_events_run_seq ON events(run_id, seq);
CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_repo ON runs(repo_path, started_at DESC);
```

### Resume Flow

When a run is resumed, the same `run_id` is preserved to maintain a complete history:

```
┌─────────────────┐
│ Original Run    │
│ seq 1-50        │
└────────┬────────┘
         │
         │ User stops (Ctrl+C)
         ▼
┌─────────────────┐
│ run.stopped     │
│ seq 51          │
└────────┬────────┘
         │
         │ User runs `choo resume`
         ▼
┌─────────────────┐
│ run.resumed     │
│ seq 52          │
└────────┬────────┘
         │
         │ Execution continues
         ▼
┌─────────────────┐
│ Continued       │
│ seq 53+         │
└─────────────────┘
```

Example event sequence for a resumed run:

```sql
-- Events for run_20250120_143052_a1b2
SELECT seq, type, unit, time FROM events WHERE run_id = 'run_20250120_143052_a1b2' ORDER BY seq;

-- seq 1-50: original run events
-- seq 1:  orch.started
-- seq 2:  unit.started     app-shell
-- ...
-- seq 50: task.completed   app-shell

-- seq 51: run stopped
-- seq 51: run.stopped      (timestamp: 2025-01-20 14:45:00)

-- seq 52: run resumed
-- seq 52: run.resumed      (timestamp: 2025-01-20 15:00:00)

-- seq 53+: continued execution
-- seq 53: unit.started     config-layer
-- ...
```

## Implementation Notes

### WAL Mode Initialization

WAL mode must be enabled immediately after opening the connection:

```go
func NewStore(dbPath string) (*Store, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }

    // Enable WAL mode for concurrent reads
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        db.Close()
        return nil, fmt.Errorf("enable WAL mode: %w", err)
    }

    // Set busy timeout to handle concurrent access
    if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
        db.Close()
        return nil, fmt.Errorf("set busy timeout: %w", err)
    }

    // Run migrations
    if err := Migrate(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("migrate schema: %w", err)
    }

    return &Store{db: db, dbPath: dbPath}, nil
}
```

### Payload Redaction

Sensitive data must be redacted before storage. The handler uses an allow-list approach:

```go
// safePayloadFields defines fields that are safe to store
var safePayloadFields = map[string]bool{
    "file":      true,
    "branch":    true,
    "commit":    true,
    "pr_number": true,
    "status":    true,
    "duration":  true,
    "exit_code": true,
}

func redactPayload(payload any) any {
    m, ok := payload.(map[string]any)
    if !ok {
        return payload
    }

    safe := make(map[string]any)
    for k, v := range m {
        if safePayloadFields[k] {
            safe[k] = v
        } else {
            safe[k] = "[redacted]"
        }
    }
    return safe
}
```

### Event Handler Implementation

The handler converts events.Event to EventRecord and sends to the daemon:

```go
func (h *Handler) Handle(e events.Event) {
    seq := h.seq.Add(1)

    record := EventRecord{
        Type:    string(e.Type),
        Unit:    e.Unit,
        Task:    e.Task,
        PR:      e.PR,
        Payload: redactPayload(e.Payload),
        Error:   e.Error,
    }

    // Send to daemon (non-blocking, daemon handles persistence)
    if err := h.client.SendEvent(h.runID, seq, record); err != nil {
        log.Printf("WARN: failed to send event to daemon: %v", err)
    }
}
```

### Concurrent Read Access

WAL mode allows unlimited concurrent readers while a single writer operates:

```go
func (s *Store) GetRunEvents(runID string, opts EventListOptions) (*EventList, error) {
    // Safe to call concurrently - WAL mode handles isolation
    query := `
        SELECT id, run_id, seq, time, type, unit, task, pr, payload, error
        FROM events
        WHERE run_id = ?
    `
    args := []any{runID}

    if opts.AfterSeq > 0 {
        query += " AND seq > ?"
        args = append(args, opts.AfterSeq)
    }

    if len(opts.Types) > 0 {
        placeholders := strings.Repeat("?,", len(opts.Types)-1) + "?"
        query += " AND type IN (" + placeholders + ")"
        for _, t := range opts.Types {
            args = append(args, t)
        }
    }

    query += " ORDER BY seq ASC"

    limit := opts.Limit
    if limit == 0 {
        limit = 1000
    }
    query += " LIMIT ?"
    args = append(args, limit+1) // +1 to detect HasMore

    rows, err := s.db.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("query events: %w", err)
    }
    defer rows.Close()

    var events []StoredEvent
    for rows.Next() {
        var e StoredEvent
        var unit, payload, errStr sql.NullString
        var task, pr sql.NullInt64

        err := rows.Scan(&e.ID, &e.RunID, &e.Seq, &e.Time, &e.Type,
            &unit, &task, &pr, &payload, &errStr)
        if err != nil {
            return nil, fmt.Errorf("scan event: %w", err)
        }

        e.Unit = unit.String
        e.Payload = payload.String
        e.Error = errStr.String
        if task.Valid {
            t := int(task.Int64)
            e.Task = &t
        }
        if pr.Valid {
            p := int(pr.Int64)
            e.PR = &p
        }

        events = append(events, e)
    }

    hasMore := len(events) > limit
    if hasMore {
        events = events[:limit]
    }

    return &EventList{
        Events:  events,
        HasMore: hasMore,
    }, nil
}
```

### Retention Cleanup

Automatic cleanup runs periodically to remove old runs:

```go
func (s *Store) DeleteOldRuns(olderThan time.Duration) (int, error) {
    cutoff := time.Now().Add(-olderThan)

    // CASCADE delete removes associated events and graphs
    result, err := s.db.Exec(`
        DELETE FROM runs
        WHERE completed_at IS NOT NULL
        AND completed_at < ?
    `, cutoff)
    if err != nil {
        return 0, fmt.Errorf("delete old runs: %w", err)
    }

    count, err := result.RowsAffected()
    if err != nil {
        return 0, fmt.Errorf("rows affected: %w", err)
    }

    return int(count), nil
}
```

### Run ID Generation

Run IDs follow a predictable format for sortability and debugging:

```go
func GenerateRunID() string {
    now := time.Now()
    suffix := randomHex(4) // 4 hex chars for uniqueness
    return fmt.Sprintf("run_%s_%s",
        now.Format("20060102_150405"),
        suffix,
    )
    // Example: "run_20250120_143052_a1b2"
}
```

## Testing Strategy

### Unit Tests

```go
// internal/history/store_test.go

func TestStore_CreateAndGetRun(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    cfg := RunConfig{
        ID:          "run_test_001",
        RepoPath:    "/home/user/myrepo",
        Parallelism: 4,
        TotalUnits:  3,
        TasksDir:    "./specs/tasks",
        DryRun:      false,
    }

    id, err := store.CreateRun(cfg)
    if err != nil {
        t.Fatalf("CreateRun failed: %v", err)
    }
    if id != cfg.ID {
        t.Errorf("expected ID %s, got %s", cfg.ID, id)
    }

    run, err := store.GetRun(id)
    if err != nil {
        t.Fatalf("GetRun failed: %v", err)
    }

    if run.RepoPath != cfg.RepoPath {
        t.Errorf("expected RepoPath %s, got %s", cfg.RepoPath, run.RepoPath)
    }
    if run.Status != RunStatusRunning {
        t.Errorf("expected status running, got %s", run.Status)
    }
    if run.Parallelism != cfg.Parallelism {
        t.Errorf("expected parallelism %d, got %d", cfg.Parallelism, run.Parallelism)
    }
}

func TestStore_InsertAndGetEvents(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    // Create run first
    _, err := store.CreateRun(RunConfig{
        ID:          "run_test_002",
        RepoPath:    "/test/repo",
        Parallelism: 2,
        TotalUnits:  1,
    })
    if err != nil {
        t.Fatalf("CreateRun failed: %v", err)
    }

    // Insert events
    events := []EventRecord{
        {Type: "unit.started", Unit: "app-shell"},
        {Type: "task.started", Unit: "app-shell", Task: intPtr(1)},
        {Type: "task.completed", Unit: "app-shell", Task: intPtr(1)},
    }

    for _, e := range events {
        if err := store.InsertEvent("run_test_002", e); err != nil {
            t.Fatalf("InsertEvent failed: %v", err)
        }
    }

    // Query events
    list, err := store.GetRunEvents("run_test_002", EventListOptions{})
    if err != nil {
        t.Fatalf("GetRunEvents failed: %v", err)
    }

    if len(list.Events) != 3 {
        t.Errorf("expected 3 events, got %d", len(list.Events))
    }

    // Verify ordering
    for i, e := range list.Events {
        if e.Seq != int64(i+1) {
            t.Errorf("event %d: expected seq %d, got %d", i, i+1, e.Seq)
        }
    }
}

func TestStore_ListRunsByRepo(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    // Create runs for different repos
    repos := []string{"/repo/a", "/repo/a", "/repo/b"}
    for i, repo := range repos {
        _, err := store.CreateRun(RunConfig{
            ID:          fmt.Sprintf("run_%d", i),
            RepoPath:    repo,
            Parallelism: 1,
            TotalUnits:  1,
        })
        if err != nil {
            t.Fatalf("CreateRun failed: %v", err)
        }
    }

    // List runs for repo/a
    list, err := store.ListRuns("/repo/a", ListOptions{})
    if err != nil {
        t.Fatalf("ListRuns failed: %v", err)
    }

    if len(list.Runs) != 2 {
        t.Errorf("expected 2 runs for /repo/a, got %d", len(list.Runs))
    }
}

func TestStore_CompleteRun(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    _, err := store.CreateRun(RunConfig{
        ID:          "run_complete_test",
        RepoPath:    "/test/repo",
        Parallelism: 2,
        TotalUnits:  3,
    })
    if err != nil {
        t.Fatalf("CreateRun failed: %v", err)
    }

    result := RunResult{
        Status:         RunStatusCompleted,
        CompletedUnits: 3,
        FailedUnits:    0,
        BlockedUnits:   0,
    }

    if err := store.CompleteRun("run_complete_test", result); err != nil {
        t.Fatalf("CompleteRun failed: %v", err)
    }

    run, err := store.GetRun("run_complete_test")
    if err != nil {
        t.Fatalf("GetRun failed: %v", err)
    }

    if run.Status != RunStatusCompleted {
        t.Errorf("expected status completed, got %s", run.Status)
    }
    if run.CompletedAt == nil {
        t.Error("expected CompletedAt to be set")
    }
    if run.CompletedUnits != 3 {
        t.Errorf("expected 3 completed units, got %d", run.CompletedUnits)
    }
}

func TestStore_ResumeMarker(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    _, err := store.CreateRun(RunConfig{
        ID:          "run_resume_test",
        RepoPath:    "/test/repo",
        Parallelism: 2,
        TotalUnits:  2,
    })
    if err != nil {
        t.Fatalf("CreateRun failed: %v", err)
    }

    // Insert some events
    store.InsertEvent("run_resume_test", EventRecord{Type: "unit.started", Unit: "a"})
    store.InsertEvent("run_resume_test", EventRecord{Type: "run.stopped"})

    // Append resume marker
    if err := store.AppendResumeMarker("run_resume_test"); err != nil {
        t.Fatalf("AppendResumeMarker failed: %v", err)
    }

    // Verify resume event was added
    list, err := store.GetRunEvents("run_resume_test", EventListOptions{})
    if err != nil {
        t.Fatalf("GetRunEvents failed: %v", err)
    }

    lastEvent := list.Events[len(list.Events)-1]
    if lastEvent.Type != "run.resumed" {
        t.Errorf("expected last event to be run.resumed, got %s", lastEvent.Type)
    }
}

func TestStore_DeleteOldRuns(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()

    // Create and complete an old run
    _, _ = store.CreateRun(RunConfig{
        ID:          "run_old",
        RepoPath:    "/test/repo",
        Parallelism: 1,
        TotalUnits:  1,
    })
    store.CompleteRun("run_old", RunResult{Status: RunStatusCompleted})

    // Manually set completed_at to 100 days ago
    _, err := store.db.Exec(`
        UPDATE runs SET completed_at = datetime('now', '-100 days')
        WHERE id = 'run_old'
    `)
    if err != nil {
        t.Fatalf("update completed_at failed: %v", err)
    }

    // Delete runs older than 90 days
    count, err := store.DeleteOldRuns(90 * 24 * time.Hour)
    if err != nil {
        t.Fatalf("DeleteOldRuns failed: %v", err)
    }

    if count != 1 {
        t.Errorf("expected 1 deleted run, got %d", count)
    }

    // Verify run is gone
    _, err = store.GetRun("run_old")
    if err == nil {
        t.Error("expected error for deleted run")
    }
}

// Helper to create test store with in-memory database
func newTestStore(t *testing.T) *Store {
    t.Helper()
    store, err := NewStore(":memory:")
    if err != nil {
        t.Fatalf("failed to create test store: %v", err)
    }
    return store
}

func intPtr(i int) *int {
    return &i
}
```

```go
// internal/history/handler_test.go

func TestHandler_SequenceNumbers(t *testing.T) {
    // Mock daemon client
    var received []struct {
        runID string
        seq   int64
    }

    client := &mockDaemonClient{
        sendEvent: func(runID string, seq int64, event EventRecord) error {
            received = append(received, struct {
                runID string
                seq   int64
            }{runID, seq})
            return nil
        },
    }

    handler := NewHandler(client, "run_seq_test")

    // Send multiple events
    for i := 0; i < 5; i++ {
        handler.Handle(events.Event{
            Type: events.TaskStarted,
            Unit: "test",
        })
    }

    // Verify sequence numbers are incrementing
    for i, r := range received {
        expected := int64(i + 1)
        if r.seq != expected {
            t.Errorf("event %d: expected seq %d, got %d", i, expected, r.seq)
        }
    }
}

func TestHandler_PayloadRedaction(t *testing.T) {
    var capturedPayload any

    client := &mockDaemonClient{
        sendEvent: func(runID string, seq int64, event EventRecord) error {
            capturedPayload = event.Payload
            return nil
        },
    }

    handler := NewHandler(client, "run_redact_test")

    // Send event with sensitive payload
    handler.Handle(events.Event{
        Type: events.TaskCompleted,
        Unit: "test",
        Payload: map[string]any{
            "file":    "01-types.md",  // safe
            "api_key": "secret123",    // should be redacted
            "status":  "success",      // safe
        },
    })

    payload := capturedPayload.(map[string]any)

    if payload["file"] != "01-types.md" {
        t.Errorf("expected file to be preserved, got %v", payload["file"])
    }
    if payload["api_key"] != "[redacted]" {
        t.Errorf("expected api_key to be redacted, got %v", payload["api_key"])
    }
    if payload["status"] != "success" {
        t.Errorf("expected status to be preserved, got %v", payload["status"])
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Concurrent reads during write | Writer inserts events, multiple readers query simultaneously |
| Resume flow | Create run, insert events, stop, resume, verify event sequence |
| Large event volume | Insert 10000 events, verify query performance |
| Retention cleanup | Create runs with old timestamps, run cleanup, verify deletion |
| Schema migration | Start with empty database, verify schema creation |

### Manual Testing

- [ ] `choo run` creates a new run record in history.db
- [ ] Events appear in database as tasks execute
- [ ] `choo resume` appends run.resumed marker and continues
- [ ] Concurrent `choo status` queries work during run
- [ ] Run listing filters correctly by repository
- [ ] Event pagination returns correct subsets
- [ ] Sensitive data is redacted in stored payloads
- [ ] Old runs are cleaned up after retention period

## Design Decisions

### Why WAL Mode?

WAL (Write-Ahead Logging) mode provides concurrent read access while a single writer operates. This is critical for the history system because:
- The daemon writes events during execution
- The CLI or web UI may query history simultaneously
- Standard SQLite journal mode would block readers during writes

Trade-off: WAL requires filesystem support for shared locks (works on most systems except some network filesystems).

### Why Single Database File?

A single `~/.choo/history.db` file simplifies:
- Backup and restore (copy one file)
- Retention management (single cleanup query)
- Cross-repo queries in future features

Alternative considered: Per-repo databases would provide isolation but complicate management and prevent cross-repo analytics.

### Why Handler Sends to Daemon Instead of Direct Writes?

The handler sends events to the daemon rather than writing directly because:
- Single writer avoids SQLite lock contention
- Daemon manages database lifecycle (open, migrate, close)
- CLI process can exit without waiting for writes
- Future: daemon can batch writes for performance

### Why Sequence Numbers Instead of Timestamps for Ordering?

Sequence numbers provide:
- Guaranteed unique ordering within a run
- No clock skew issues across resumed runs
- Efficient indexing and range queries
- Clear causality for debugging

Timestamps are still stored for display but not used for ordering.

### Why Allow-list for Payload Fields?

Allow-listing safe fields (rather than deny-listing sensitive ones) is more secure:
- New sensitive fields are blocked by default
- Explicit approval required for each stored field
- Reduces risk of accidentally storing secrets

## Future Enhancements

1. Export to JSON/CSV for analysis and reporting
2. Event streaming via WebSocket for live dashboards
3. Cross-repo analytics (aggregate stats across all projects)
4. Full-text search in event payloads
5. Event replay for debugging failed runs
6. Compression for old event payloads to reduce storage

## References

- [HISTORY-TYPES.md](./HISTORY-TYPES.md) - Canonical shared type definitions
- [Historical Runs PRD](/docs/HISTORICAL-RUNS-PRD.md) - Product requirements
- [SQLite WAL Mode](https://www.sqlite.org/wal.html) - WAL documentation
- [EVENTS Spec](./completed/EVENTS.md) - Event types and bus architecture
