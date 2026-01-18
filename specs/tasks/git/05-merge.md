---
task: 5
status: pending
backpressure: "go test ./internal/git/..."
depends_on: [1, 3]
---

# Merge Serialization

**Parent spec**: `/specs/GIT.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement MergeManager with mutex-based FCFS serialization for safe concurrent merge operations.

## Dependencies

### External Specs (must be implemented)
- CLAUDE - provides `*claude.Client` for conflict resolution

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `gitExec`)
- Task #3 must be complete (provides: `Branch` type)

### Package Dependencies
- Standard library (`sync`, `context`)
- `internal/claude`

## Deliverables

### Files to Create/Modify

```
internal/git/
└── merge.go    # CREATE: Merge serialization and rebase operations
```

### Types to Implement

```go
// MergeManager handles serialized merging of branches
type MergeManager struct {
    // mutex ensures only one merge at a time
    mutex sync.Mutex

    // RepoRoot is the main repository path
    RepoRoot string

    // Claude client for conflict resolution
    Claude *claude.Client

    // MaxConflictAttempts is the max retries for conflict resolution
    MaxConflictAttempts int

    // PendingDeletes tracks branches to delete after batch completes
    PendingDeletes []string
}

// MergeResult contains the outcome of a merge operation
type MergeResult struct {
    // Success indicates if the merge completed
    Success bool

    // ConflictsResolved is the number of conflicts that were resolved
    ConflictsResolved int

    // Attempts is how many conflict resolution attempts were made
    Attempts int

    // Error is set if the merge failed
    Error error
}
```

### Functions to Implement

```go
// NewMergeManager creates a new merge manager
func NewMergeManager(repoRoot string, claude *claude.Client) *MergeManager

// Merge acquires the merge lock, rebases, and merges a branch
// This is the primary entry point for merge operations
func (m *MergeManager) Merge(ctx context.Context, branch *Branch) (*MergeResult, error)

// ScheduleBranchDelete marks a branch for deletion after batch completes
func (m *MergeManager) ScheduleBranchDelete(branchName string)

// FlushDeletes deletes all scheduled branches
// Called after full batch is merged
func (m *MergeManager) FlushDeletes(ctx context.Context) error

// Rebase rebases the current branch onto the target
// Returns hasConflicts=true if conflicts are detected
func Rebase(ctx context.Context, worktreePath, targetBranch string) (hasConflicts bool, err error)

// ForcePushWithLease pushes with --force-with-lease for safety
func ForcePushWithLease(ctx context.Context, worktreePath string) error

// Fetch fetches the latest from origin for the target branch
func Fetch(ctx context.Context, repoRoot, targetBranch string) error

// deleteBranch deletes a branch locally and/or remotely
func deleteBranch(ctx context.Context, repoRoot, branchName string, remote bool) error

// getConflictedFiles returns list of files with merge conflicts
func getConflictedFiles(ctx context.Context, worktreePath string) ([]string, error)

// continueRebase continues a rebase after conflict resolution
func continueRebase(ctx context.Context, worktreePath string) error

// abortRebase aborts a rebase in progress
func abortRebase(ctx context.Context, worktreePath string) error
```

### Merge Flow

```
1. Acquire mutex (blocks until available)
2. Fetch origin/<target_branch>
3. Rebase onto origin/<target_branch>
4. If conflicts -> delegate to Task #6 (conflict resolution)
5. Push (force-with-lease if rebased)
6. Schedule branch for deletion
7. Release mutex
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestMerge
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestMergeManager_Serialization` | Concurrent merges execute serially |
| `TestMergeManager_AcquireLock` | Mutex blocks concurrent access |
| `TestMergeManager_PendingDeletes` | ScheduleBranchDelete adds to list |
| `TestMergeManager_FlushDeletes` | Clears pending deletes list |
| `TestRebase_NoConflicts` | Returns hasConflicts=false when clean |
| `TestRebase_WithConflicts` | Returns hasConflicts=true on conflicts |
| `TestForcePushWithLease` | Executes push --force-with-lease |
| `TestFetch` | Fetches target branch from origin |
| `TestGetConflictedFiles` | Returns list of conflicted file paths |
| `TestDeleteBranch_Local` | Deletes local branch |
| `TestDeleteBranch_Remote` | Deletes remote branch |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Temp git repo with remote | Created in test | Test merge operations |
| Conflicting branches | Created in test | Test conflict detection |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses local bare repo as "remote")
- [x] Runs in <60 seconds

## Implementation Notes

- Mutex acquisition should be fast (<100ms excluding wait)
- Use `--force-with-lease` not `--force` for safety
- Branch deletion is deferred until batch completes
- Conflict resolution is in Task #6

## NOT In Scope

- Worktree management (Task #2)
- Conflict resolution with Claude (Task #6)
- GitHub PR API integration (separate unit)
