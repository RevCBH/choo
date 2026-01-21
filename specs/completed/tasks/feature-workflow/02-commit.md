---
task: 2
status: complete
backpressure: "go test ./internal/feature/... -run TestCommit"
depends_on: [1]
---

# Spec Commit Operations

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement spec commit operations that stage, commit, and push generated specs to the feature branch.

## Dependencies

### External Specs (must be implemented)
- GIT - provides `git.Client` for git operations

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`)

### Package Dependencies
- `internal/git` - git operations

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── commit.go    # CREATE: Commit operations
```

### Types to Implement

```go
// CommitResult holds the result of the commit operation
type CommitResult struct {
    CommitHash string
    FileCount  int
    Pushed     bool
}

// CommitOptions configures the commit operation
type CommitOptions struct {
    PushRetries int  // default 1 (one retry)
    DryRun      bool
}
```

### Functions to Implement

```go
// CommitSpecs stages and commits generated specs to the feature branch
func CommitSpecs(ctx context.Context, gitClient *git.Client, prdID string) (*CommitResult, error)

// CommitSpecsWithOptions commits with custom options
func CommitSpecsWithOptions(ctx context.Context, gitClient *git.Client, prdID string, opts CommitOptions) (*CommitResult, error)

// generateCommitMessage returns the standardized commit message
// Format: "chore(feature): add specs for <prd-id>"
func generateCommitMessage(prdID string) string
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestCommit
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCommitSpecs_Success` | Stages files, commits, pushes, returns result |
| `TestCommitSpecs_StagingFailure` | Returns error on stage failure |
| `TestCommitSpecs_CommitFailure` | Returns error on commit failure |
| `TestCommitSpecs_PushRetry` | Retries push once on failure |
| `TestCommitSpecs_PushFailsAfterRetry` | Returns error after retry exhausted |
| `TestCommitSpecs_DryRun` | Does not execute git commands |
| `TestGenerateCommitMessage` | Returns `"chore(feature): add specs for test-prd"` |

### Test Fixtures

None required - uses mocked git client.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Specs directory path: `specs/tasks/<prd-id>/`
- Commit message format is deterministic (no Claude invocation)
- Push retry logic: one automatic retry before returning error
- Dry-run mode should log operations without executing

```go
func (w *Workflow) commitSpecs(ctx context.Context) error {
    specsDir := filepath.Join("specs/tasks", w.prd.ID)

    // Stage all generated spec files
    if err := w.git.Add(specsDir); err != nil {
        return fmt.Errorf("failed to stage specs: %w", err)
    }

    // Commit with standardized message (no Claude invocation)
    commitMsg := generateCommitMessage(w.prd.ID)
    if err := w.git.Commit(commitMsg); err != nil {
        return fmt.Errorf("failed to commit specs: %w", err)
    }

    // Push to remote feature branch with retry
    var pushErr error
    for i := 0; i <= 1; i++ { // One retry
        if pushErr = w.git.Push(); pushErr == nil {
            return nil
        }
    }
    return fmt.Errorf("failed to push specs after retry: %w", pushErr)
}
```

## NOT In Scope

- State definitions (Task #1)
- Drift detection (Task #3)
- Completion logic (Task #4)
- Review cycle management (Task #5)
- Workflow orchestration (Task #6)
