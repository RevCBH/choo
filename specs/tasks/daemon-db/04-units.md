---
task: 4
status: pending
backpressure: "go test ./internal/daemon/db/... -run TestUnit"
depends_on: [3]
---

# Unit CRUD Operations

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement all CRUD operations for the units table, including status updates, branch assignment, and filtering queries.

## Dependencies

### Task Dependencies (within this unit)
- Task #3 must be complete (runs must exist before units can reference them)

### Package Dependencies
- `database/sql` - Standard database interface
- `time` - For timestamp handling
- `fmt` - For error wrapping

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
├── units.go    # CREATE: Unit CRUD operations
└── db_test.go  # MODIFY: Add unit tests
```

### Functions to Implement

```go
// CreateUnit inserts a new unit into the database.
// The unit ID should be created using MakeUnitRecordID(runID, unitID).
// Returns an error if the parent run does not exist.
func (db *DB) CreateUnit(unit *UnitRecord) error {
    // INSERT with all fields
    // Foreign key constraint ensures run exists
}

// GetUnit retrieves a unit by its composite ID.
// Returns nil, nil if the unit does not exist.
func (db *DB) GetUnit(id string) (*UnitRecord, error) {
    // SELECT * FROM units WHERE id = ?
    // Handle sql.ErrNoRows -> return nil, nil
}

// UpdateUnitStatus updates the status of a unit.
// Sets started_at when transitioning to InProgress.
// Sets completed_at when transitioning to Complete/Failed.
func (db *DB) UpdateUnitStatus(id string, status UnitStatus, err *string) error {
    // UPDATE units SET status = ?, error = ?, started_at/completed_at = ? WHERE id = ?
    // Check rows affected, return error if unit not found
}

// UpdateUnitBranch sets the git branch and worktree path for a unit.
// Called when a worktree is created for the unit's work.
func (db *DB) UpdateUnitBranch(id string, branch, worktreePath string) error {
    // UPDATE units SET branch = ?, worktree_path = ? WHERE id = ?
    // Check rows affected, return error if unit not found
}

// ListUnitsByRun returns all units belonging to a run.
func (db *DB) ListUnitsByRun(runID string) ([]*UnitRecord, error) {
    // SELECT * FROM units WHERE run_id = ? ORDER BY unit_id
}

// ListUnitsByStatus returns all units with the given status within a run.
func (db *DB) ListUnitsByStatus(runID string, status UnitStatus) ([]*UnitRecord, error) {
    // SELECT * FROM units WHERE run_id = ? AND status = ? ORDER BY unit_id
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/db/... -run TestUnit
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestUnitCreate` | CreateUnit inserts and GetUnit retrieves with matching fields |
| `TestUnitCreateWithoutRun` | CreateUnit returns foreign key error for non-existent run |
| `TestUnitGetNotFound` | GetUnit returns nil, nil for non-existent ID |
| `TestUnitUpdateStatus` | UpdateUnitStatus changes status and sets timestamps |
| `TestUnitUpdateStatusWithError` | UpdateUnitStatus stores error message |
| `TestUnitUpdateBranch` | UpdateUnitBranch sets branch and worktree path |
| `TestUnitListByRun` | ListUnitsByRun returns all units for a run |
| `TestUnitListByStatus` | ListUnitsByStatus returns only units with matching status |

## NOT In Scope

- Event operations (Task #5)
- Cascade delete verification (Task #6)
- Bulk unit creation (not in API)
