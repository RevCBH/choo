# Task 02: SQLite Schema and Migrations

```yaml
task: 02-schema
unit: history-store
depends_on: [01-types]
backpressure: "go test ./internal/history/... -run TestMigrate -v"
```

## Objective

Create the SQLite schema definition and migration system in `internal/history/migrate.go`.

## Requirements

1. Create `internal/history/migrate.go` with:
   - `Migrate(db *sql.DB) error` function
   - Schema version tracking via `schema_version` table
   - Initial schema (version 1) with tables:
     - `runs` - orchestration runs
     - `events` - run events with sequence numbers
     - `graphs` - dependency graph data per run

2. Schema for `runs` table:
   ```sql
   CREATE TABLE runs (
       id TEXT PRIMARY KEY,
       repo_path TEXT NOT NULL,
       started_at DATETIME NOT NULL,
       completed_at DATETIME,
       status TEXT NOT NULL DEFAULT 'running',
       parallelism INTEGER NOT NULL DEFAULT 1,
       total_units INTEGER NOT NULL DEFAULT 0,
       completed_units INTEGER NOT NULL DEFAULT 0,
       failed_units INTEGER NOT NULL DEFAULT 0,
       blocked_units INTEGER NOT NULL DEFAULT 0,
       error TEXT,
       tasks_dir TEXT,
       dry_run INTEGER NOT NULL DEFAULT 0
   );
   CREATE INDEX idx_runs_repo ON runs(repo_path);
   CREATE INDEX idx_runs_status ON runs(status);
   CREATE INDEX idx_runs_started ON runs(started_at DESC);
   ```

3. Schema for `events` table:
   ```sql
   CREATE TABLE events (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       run_id TEXT NOT NULL REFERENCES runs(id),
       seq INTEGER NOT NULL,
       time DATETIME NOT NULL,
       type TEXT NOT NULL,
       unit TEXT,
       task INTEGER,
       pr INTEGER,
       payload TEXT,
       error TEXT,
       UNIQUE(run_id, seq)
   );
   CREATE INDEX idx_events_run ON events(run_id);
   CREATE INDEX idx_events_type ON events(type);
   CREATE INDEX idx_events_unit ON events(unit);
   ```

4. Schema for `graphs` table:
   ```sql
   CREATE TABLE graphs (
       run_id TEXT PRIMARY KEY REFERENCES runs(id),
       nodes TEXT NOT NULL,
       edges TEXT NOT NULL,
       levels TEXT NOT NULL
   );
   ```

5. Enable WAL mode on database open

## Acceptance Criteria

- [ ] `Migrate` function creates all tables idempotently
- [ ] Schema version tracking works
- [ ] WAL mode is enabled
- [ ] All indexes are created
- [ ] Test covers migration from empty database

## Files to Create/Modify

- `internal/history/migrate.go` (create)
- `internal/history/migrate_test.go` (create)
