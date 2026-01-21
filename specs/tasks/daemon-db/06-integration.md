---
task: 6
status: complete
backpressure: "go test ./internal/daemon/db/... -v"
depends_on: [4, 5]
---

# Integration Tests

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement integration tests verifying cascade deletes, resumability, and cross-entity operations.

## Dependencies

### Task Dependencies (within this unit)
- Task #4 must be complete (unit operations)
- Task #5 must be complete (event operations)

### Package Dependencies
- `testing` - Standard testing package
- `github.com/stretchr/testify/require` - Test assertions
- `github.com/stretchr/testify/assert` - Test assertions

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
└── db_test.go  # MODIFY: Add integration tests
```

### Test Functions to Implement

```go
// TestCascadeDelete verifies that deleting a run removes all associated
// units and events automatically via foreign key cascade.
func TestCascadeDelete(t *testing.T) {
    // 1. Create run
    // 2. Create multiple units for run
    // 3. Append multiple events for run
    // 4. Delete run
    // 5. Assert ListUnitsByRun returns empty
    // 6. Assert ListEvents returns empty
}

// TestUniqueConstraintBranchRepo verifies that only one run can exist
// for a given branch/repo combination.
func TestUniqueConstraintBranchRepo(t *testing.T) {
    // 1. Create run with branch "feature/test" and repo "/repo"
    // 2. Attempt to create second run with same branch/repo
    // 3. Assert error contains "UNIQUE constraint"
    // 4. Create run with different branch, same repo -> succeeds
    // 5. Create run with same branch, different repo -> succeeds
}

// TestRunLifecycle verifies a complete run through all status transitions.
func TestRunLifecycle(t *testing.T) {
    // 1. Create run with Pending status
    // 2. Update to Running -> assert started_at is set
    // 3. Update to Completed -> assert completed_at is set
    // 4. Retrieve and verify all fields
}

// TestUnitLifecycle verifies a complete unit through all status transitions.
func TestUnitLifecycle(t *testing.T) {
    // 1. Create unit with Pending status
    // 2. Update to InProgress -> assert started_at is set
    // 3. Update branch and worktree path
    // 4. Update to Complete -> assert completed_at is set
    // 5. Retrieve and verify all fields
}

// TestEventSequencingConcurrent verifies that concurrent event appends
// receive unique sequence numbers.
func TestEventSequencingConcurrent(t *testing.T) {
    // 1. Create run
    // 2. Launch multiple goroutines appending events
    // 3. Wait for completion
    // 4. Assert all sequence numbers are unique and consecutive
}

// TestResumability verifies that incomplete runs can be found after restart.
func TestResumability(t *testing.T) {
    // 1. Create runs with various statuses: Pending, Running, Completed, Failed
    // 2. Call ListIncompleteRuns
    // 3. Assert only Pending and Running runs are returned
    // 4. Assert Completed and Failed runs are not returned
}

// TestGetRunByBranchPartialMatch ensures exact matching on branch/repo.
func TestGetRunByBranchPartialMatch(t *testing.T) {
    // 1. Create run with branch "feature/test"
    // 2. GetRunByBranch with "feature" -> returns nil
    // 3. GetRunByBranch with "feature/test" -> returns run
}

// setupTestDB creates an in-memory database for testing.
func setupTestDB(t *testing.T) *DB {
    // Open(":memory:")
    // t.Cleanup to close
}

// createTestRun creates a run with unique branch for testing.
func createTestRun(t *testing.T, db *DB) string {
    // Generate unique branch name using ULID prefix
    // Create and return run ID
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/db/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCascadeDelete` | Deleting run removes all units and events |
| `TestUniqueConstraintBranchRepo` | Duplicate branch/repo causes unique constraint error |
| `TestRunLifecycle` | Run transitions through all statuses correctly |
| `TestUnitLifecycle` | Unit transitions through all statuses correctly |
| `TestEventSequencingConcurrent` | Concurrent appends produce unique sequences |
| `TestResumability` | ListIncompleteRuns returns only pending/running |
| `TestGetRunByBranchPartialMatch` | GetRunByBranch requires exact match |
| All previous tests | All tests from tasks 2-5 continue to pass |

## NOT In Scope

- Performance benchmarks (future enhancement)
- Schema migration version tests (future enhancement)
- WAL file corruption recovery (manual testing only)
