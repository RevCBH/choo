package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection with daemon-specific operations
type DB struct {
	conn *sql.DB
}

// Open creates or opens a SQLite database at the given path.
// It enables WAL mode, foreign keys, and runs migrations.
func Open(path string) (*DB, error) {
	// 1. Open connection with modernc.org/sqlite driver
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 2. Enable WAL mode: PRAGMA journal_mode=WAL
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// 3. Enable foreign keys: PRAGMA foreign_keys=ON
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn}

	// 4. Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// 5. Return wrapped DB
	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate creates or updates the database schema
func (db *DB) migrate() error {
	schema := `
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
`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}
