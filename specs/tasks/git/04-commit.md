---
task: 4
status: pending
backpressure: "go test ./internal/git/..."
depends_on: [1]
---

# Commit Operations

**Parent spec**: `/specs/GIT.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement commit operations including staging, committing with `--no-verify`, and checking for uncommitted changes.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `gitExec`)

### Package Dependencies
- Standard library only

## Deliverables

### Files to Create/Modify

```
internal/git/
└── commit.go    # CREATE: Staging and commit operations
```

### Types to Implement

```go
// CommitOptions configures a commit operation
type CommitOptions struct {
    // Message is the commit message
    Message string

    // NoVerify skips pre-commit hooks (default: true during tasks)
    NoVerify bool

    // AllowEmpty permits commits with no changes
    AllowEmpty bool
}
```

### Functions to Implement

```go
// Commit stages and commits changes in a worktree
func Commit(ctx context.Context, worktreePath string, opts CommitOptions) error

// StageAll stages all changes in a worktree (git add -A)
func StageAll(ctx context.Context, worktreePath string) error

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges(ctx context.Context, worktreePath string) (bool, error)

// GetStagedFiles returns list of files staged for commit
func GetStagedFiles(ctx context.Context, worktreePath string) ([]string, error)
```

### Implementation Notes

From the design spec:

```go
func Commit(ctx context.Context, worktreePath string, opts CommitOptions) error {
    args := []string{"commit", "-m", opts.Message}

    if opts.NoVerify {
        args = append(args, "--no-verify")
    }
    if opts.AllowEmpty {
        args = append(args, "--allow-empty")
    }

    _, err := gitExec(ctx, worktreePath, args...)
    return err
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestCommit
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestStageAll` | Stages all changes including untracked files |
| `TestCommit_Basic` | Creates commit with message |
| `TestCommit_NoVerify` | Passes `--no-verify` flag when set |
| `TestCommit_AllowEmpty` | Creates empty commit when allowed |
| `TestCommit_FailsEmpty` | Fails on empty commit when not allowed |
| `TestHasUncommittedChanges_Clean` | Returns false for clean worktree |
| `TestHasUncommittedChanges_Dirty` | Returns true for modified files |
| `TestHasUncommittedChanges_Staged` | Returns true for staged files |
| `TestGetStagedFiles` | Returns list of staged file paths |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Temp git repo | Created in test | Test commit operations |
| Test files | Created in test | Files to stage and commit |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- `--no-verify` is default true during task execution to skip pre-commit hooks
- `StageAll` uses `git add -A` to stage all changes including deletions
- `HasUncommittedChanges` checks both staged and unstaged changes

## NOT In Scope

- Worktree management (Task #2)
- Branch naming (Task #3)
- Merge operations (Task #5, #6)
