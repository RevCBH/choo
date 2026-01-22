---
task: 4
status: complete
backpressure: "go test ./internal/worker/... -run 'TestCommitReviewFixes|TestHasUncommittedChanges' -v"
depends_on: [2]
---

# commitReviewFixes and hasUncommittedChanges Migration

**Parent spec**: `/specs/GITOPS-WORKER.md`
**Task**: #4 of 5 in implementation plan

## Objective

Migrate commitReviewFixes() and hasUncommittedChanges() to use GitOps methods (Status, AddAll, Commit).

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides GitOps.Status, GitOps.AddAll, GitOps.Commit, StatusResult
- GITOPS-MOCK — provides MockGitOps for testing

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: Worker.gitOps initialized)

### Package Dependencies
- Standard library (`context`, `errors`, `fmt`)
- Internal: `internal/git` (for CommitOpts, StatusResult, error types)

## Deliverables

### Files to Modify

```
internal/worker/
├── review.go      # MODIFY: Update commitReviewFixes and hasUncommittedChanges
└── review_test.go # MODIFY: Add tests with MockGitOps
```

### Functions to Modify

```go
// commitReviewFixes commits any changes made during the fix attempt.
// Returns (true, nil) if changes were committed, (false, nil) if no changes.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error) {
    // Phase 1-2: Check if GitOps is available
    if w.gitOps == nil {
        return w.commitReviewFixesLegacy(ctx)
    }

    // 1. Check for staged/unstaged changes via Status
    status, err := w.gitOps.Status(ctx)
    if err != nil {
        return false, fmt.Errorf("checking for changes: %w", err)
    }
    if status.Clean {
        return false, nil
    }

    // 2. Stage all changes
    if err := w.gitOps.AddAll(ctx); err != nil {
        return false, fmt.Errorf("staging changes: %w", err)
    }

    // 3. Commit with standardized message
    commitMsg := "fix: address code review feedback"
    if err := w.gitOps.Commit(ctx, commitMsg, git.CommitOpts{NoVerify: true}); err != nil {
        // Handle branch guard errors
        if errors.Is(err, git.ErrProtectedBranch) {
            return false, fmt.Errorf("cannot commit to protected branch: %w", err)
        }
        return false, fmt.Errorf("committing changes: %w", err)
    }

    return true, nil
}

// commitReviewFixesLegacy is the old implementation using raw Runner.
// Retained for Phase 1-2 backward compatibility.
func (w *Worker) commitReviewFixesLegacy(ctx context.Context) (bool, error) {
    if w.worktreePath == "" {
        return false, nil
    }

    // Check for changes
    out, _ := w.runner().Exec(ctx, w.worktreePath, "status", "--porcelain")
    if strings.TrimSpace(out) == "" {
        return false, nil
    }

    // Stage and commit
    w.runner().Exec(ctx, w.worktreePath, "add", "-A")
    _, err := w.runner().Exec(ctx, w.worktreePath, "commit", "-m", "fix: address code review feedback", "--no-verify")
    if err != nil {
        return false, err
    }

    return true, nil
}

// hasUncommittedChanges returns true if there are staged or unstaged changes.
func (w *Worker) hasUncommittedChanges(ctx context.Context) (bool, error) {
    // Phase 1-2: Check if GitOps is available
    if w.gitOps == nil {
        return w.hasUncommittedChangesLegacy(ctx)
    }

    status, err := w.gitOps.Status(ctx)
    if err != nil {
        return false, err
    }
    return !status.Clean, nil
}

// hasUncommittedChangesLegacy is the old implementation using raw Runner.
func (w *Worker) hasUncommittedChangesLegacy(ctx context.Context) (bool, error) {
    if w.worktreePath == "" {
        return false, nil
    }

    out, err := w.runner().Exec(ctx, w.worktreePath, "status", "--porcelain")
    if err != nil {
        return false, err
    }
    return strings.TrimSpace(out) != "", nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run 'TestCommitReviewFixes|TestHasUncommittedChanges' -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCommitReviewFixes_NoChanges` | Returns (false, nil) when status.Clean=true |
| `TestCommitReviewFixes_WithChanges` | Returns (true, nil), calls AddAll and Commit |
| `TestCommitReviewFixes_ProtectedBranch` | Returns error wrapping ErrProtectedBranch |
| `TestCommitReviewFixes_StatusError` | Returns error from Status |
| `TestCommitReviewFixes_AddAllError` | Returns error from AddAll |
| `TestCommitReviewFixes_CommitMessage` | Commit called with "fix: address code review feedback" |
| `TestCommitReviewFixes_NoVerify` | Commit called with NoVerify=true |
| `TestHasUncommittedChanges_Clean` | Returns (false, nil) when Clean=true |
| `TestHasUncommittedChanges_Modified` | Returns (true, nil) when files modified |
| `TestHasUncommittedChanges_NilGitOps` | Falls back to legacy behavior |

### Test Implementation

```go
func TestCommitReviewFixes_NoChanges(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: true}

    w := &Worker{gitOps: mockOps}

    committed, err := w.commitReviewFixes(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if committed {
        t.Error("expected committed=false when no changes")
    }
    mockOps.AssertCalled(t, "Status")
    mockOps.AssertNotCalled(t, "AddAll")
    mockOps.AssertNotCalled(t, "Commit")
}

func TestCommitReviewFixes_WithChanges(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

    w := &Worker{gitOps: mockOps}

    committed, err := w.commitReviewFixes(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if !committed {
        t.Error("expected committed=true when changes exist")
    }
    mockOps.AssertCalled(t, "AddAll")
    mockOps.AssertCalled(t, "Commit")
}

func TestCommitReviewFixes_ProtectedBranch(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
    mockOps.CommitErr = fmt.Errorf("%w: main", git.ErrProtectedBranch)

    w := &Worker{gitOps: mockOps}

    _, err := w.commitReviewFixes(context.Background())

    if err == nil {
        t.Error("expected error for protected branch")
    }
    if !errors.Is(err, git.ErrProtectedBranch) {
        t.Errorf("expected error to wrap ErrProtectedBranch, got %v", err)
    }
}

func TestCommitReviewFixes_CommitMessage(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

    w := &Worker{gitOps: mockOps}
    w.commitReviewFixes(context.Background())

    commitCalls := mockOps.GetCallsFor("Commit")
    if len(commitCalls) != 1 {
        t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
    }
    msg := commitCalls[0].Args[0].(string)
    if msg != "fix: address code review feedback" {
        t.Errorf("expected message 'fix: address code review feedback', got %s", msg)
    }
}

func TestCommitReviewFixes_NoVerify(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

    w := &Worker{gitOps: mockOps}
    w.commitReviewFixes(context.Background())

    commitCalls := mockOps.GetCallsFor("Commit")
    if len(commitCalls) != 1 {
        t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
    }
    opts := commitCalls[0].Args[1].(git.CommitOpts)
    if !opts.NoVerify {
        t.Error("expected NoVerify=true")
    }
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: true}

    w := &Worker{gitOps: mockOps}

    hasChanges, err := w.hasUncommittedChanges(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if hasChanges {
        t.Error("expected hasChanges=false when Clean=true")
    }
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
    mockOps := git.NewMockGitOps("/test/worktree")
    mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

    w := &Worker{gitOps: mockOps}

    hasChanges, err := w.hasUncommittedChanges(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if !hasChanges {
        t.Error("expected hasChanges=true when files modified")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- commitReviewFixes uses NoVerify=true to skip pre-commit hooks
- Status is checked first to avoid unnecessary AddAll/Commit calls
- Protected branch errors are explicitly handled and wrapped
- Legacy fallback retained for Phase 1-2 backward compatibility

## NOT In Scope

- cleanupWorktree migration (Task #3)
- Full test migration (Task #5)
