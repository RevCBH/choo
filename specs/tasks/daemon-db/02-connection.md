---
task: 2
status: pending
backpressure: "go test ./internal/daemon/db/... -run TestOpen"
depends_on: [1]
---

# Connection and Migrations

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement database connection management with WAL mode, foreign key enforcement, and schema migrations.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete (types are referenced in schema)

### Package Dependencies
- `modernc.org/sqlite` - Pure Go SQLite implementation
- `database/sql` - Standard database interface

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
├── db.go       # CREATE: Connection management and migrations
└── db_test.go  # CREATE: Basic connection tests
```

### Types to Implement

```go
// DB wraps the SQLite connection with daemon-specific operations
type DB struct {
    conn *sql.DB
}
```

### Functions to Implement

```go
// Open creates or opens a SQLite database at the given path.
// It enables WAL mode, foreign keys, and runs migrations.
func Open(path string) (*DB, error) {
    // 1. Open connection with modernc.org/sqlite driver
    // 2. Enable WAL mode: PRAGMA journal_mode=WAL
    // 3. Enable foreign keys: PRAGMA foreign_keys=ON
    // 4. Run migrations
    // 5. Return wrapped DB
}

// Close closes the database connection
func (db *DB) Close() error {
    // Close underlying sql.DB connection
}

// migrate creates or updates the database schema
func (db *DB) migrate() error {
    // Execute CREATE TABLE IF NOT EXISTS for:
    // - runs table with unique constraint on (feature_branch, repo_path)
    // - units table with foreign key to runs, cascade delete
    // - events table with foreign key to runs, cascade delete
    // - All required indexes
}
```

### Schema SQL

```sql
-- Runs table: Top-level workflow executions
CREATE TABLE IF NOT EXISTS runs (
    id              TEXT PRIMARY KEY,
    feature_branch  TEXT NOT NULL,
    repo_path       TEXT NOT NULL,
    target_branch   TEXT NOT NULL,
    tasks_dir       TEXT NOT NULL,
    parallelism     INTEGER NOT NULL,
    status          TEXT NOT NULL,
    started_at      DATETIME,
    completed_at    DATETIME,
    error           TEXT,
    config_json     TEXT,
    UNIQUE(feature_branch, repo_path)
);

-- Units table: Individual work units within a run
CREATE TABLE IF NOT EXISTS units (
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
CREATE TABLE IF NOT EXISTS events (
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
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE INDEX IF NOT EXISTS idx_units_run_id ON units(run_id);
CREATE INDEX IF NOT EXISTS idx_units_status ON units(status);
CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_events_sequence ON events(run_id, sequence);
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/db/... -run TestOpen
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestOpen` | Opens in-memory database without error |
| `TestOpenWALMode` | WAL mode is enabled after open |
| `TestOpenForeignKeys` | Foreign keys are enabled after open |
| `TestOpenMigration` | All tables exist after open |
| `TestClose` | Close returns no error |

## NOT In Scope

- Run CRUD operations (Task #3)
- Unit CRUD operations (Task #4)
- Event operations (Task #5)
- Integration tests (Task #6)
