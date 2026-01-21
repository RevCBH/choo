---
task: 3
status: pending
backpressure: "go test ./internal/daemon/db/... -run TestRun"
depends_on: [2]
---

# Run CRUD Operations

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement all CRUD operations for the runs table, including status updates and filtering queries.

## Dependencies

### Task Dependencies (within this unit)
- Task #2 must be complete (DB connection and schema required)

### Package Dependencies
- `database/sql` - Standard database interface
- `time` - For timestamp handling
- `fmt` - For error wrapping

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
├── runs.go     # CREATE: Run CRUD operations
└── db_test.go  # MODIFY: Add run tests
```

### Functions to Implement

```go
// CreateRun inserts a new run into the database.
// Returns an error if a run already exists for the same branch/repo.
func (db *DB) CreateRun(run *Run) error {
    // INSERT with all fields
    // Set started_at if status is Running
    // Handle unique constraint violation with descriptive error
}

// GetRun retrieves a run by its ID.
// Returns nil, nil if the run does not exist.
func (db *DB) GetRun(id string) (*Run, error) {
    // SELECT * FROM runs WHERE id = ?
    // Handle sql.ErrNoRows -> return nil, nil
}

// GetRunByBranch retrieves a run by feature branch and repo path.
// Returns nil, nil if no matching run exists.
func (db *DB) GetRunByBranch(featureBranch, repoPath string) (*Run, error) {
    // SELECT * FROM runs WHERE feature_branch = ? AND repo_path = ?
    // Handle sql.ErrNoRows -> return nil, nil
}

// UpdateRunStatus updates the status of a run.
// Sets started_at when transitioning to Running.
// Sets completed_at when transitioning to Completed/Failed/Cancelled.
func (db *DB) UpdateRunStatus(id string, status RunStatus, err *string) error {
    // UPDATE runs SET status = ?, error = ?, started_at/completed_at = ? WHERE id = ?
    // Check rows affected, return error if run not found
}

// ListRunsByStatus returns all runs with the given status.
func (db *DB) ListRunsByStatus(status RunStatus) ([]*Run, error) {
    // SELECT * FROM runs WHERE status = ? ORDER BY id
}

// ListIncompleteRuns returns all runs that are not completed/failed/cancelled.
// Used for resuming interrupted workflows after daemon restart.
func (db *DB) ListIncompleteRuns() ([]*Run, error) {
    // SELECT * FROM runs WHERE status IN ('pending', 'running') ORDER BY id
}

// DeleteRun removes a run and all associated units/events (cascade).
func (db *DB) DeleteRun(id string) error {
    // DELETE FROM runs WHERE id = ?
    // Cascade handles units and events automatically
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/db/... -run TestRun
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunCreate` | CreateRun inserts and GetRun retrieves with matching fields |
| `TestRunCreateDuplicate` | CreateRun returns error for same branch/repo |
| `TestRunGetByBranch` | GetRunByBranch finds run by branch and repo path |
| `TestRunGetNotFound` | GetRun returns nil, nil for non-existent ID |
| `TestRunUpdateStatus` | UpdateRunStatus changes status and sets timestamps |
| `TestRunUpdateStatusWithError` | UpdateRunStatus stores error message |
| `TestRunListByStatus` | ListRunsByStatus returns only matching runs |
| `TestRunListIncomplete` | ListIncompleteRuns returns pending and running runs |
| `TestRunDelete` | DeleteRun removes run from database |

## NOT In Scope

- Unit CRUD operations (Task #4)
- Event operations (Task #5)
- Cascade delete verification (Task #6)
- Transaction support for multi-table updates (Task #6)
