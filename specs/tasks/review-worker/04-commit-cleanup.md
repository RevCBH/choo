---
task: 4
status: pending
backpressure: "go test ./internal/worker/... -run TestCommitReviewFixes"
depends_on: [3]
---

# Commit and Cleanup Operations

**Parent spec**: `specs/REVIEW-WORKER.md`
**Task**: #4 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement `commitReviewFixes()` for committing fix changes and `cleanupWorktree()` for resetting the worktree after failed fix attempts. These operations ensure the worktree is in a clean state for merge regardless of fix outcome.

## Dependencies

### External Specs (must be implemented)
- None directly, but builds on existing git infrastructure

### Task Dependencies (within this unit)
- Task #3 (Fix Loop) - calls these functions

### Package Dependencies
- `github.com/RevCBH/choo/internal/git` - for git.Runner interface
- `context` - for context handling
- `fmt` - for error formatting

## Deliverables

### Files to Create/Modify

```
internal/worker/
└── review.go    # MODIFY: Add commitReviewFixes and cleanupWorktree
```

### Functions to Implement

```go
// commitReviewFixes commits any changes made during the fix attempt.
// Returns (true, nil) if changes were committed, (false, nil) if no changes.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error) {
    // 1. Check for staged/unstaged changes using git status
    hasChanges, err := w.hasUncommittedChanges(ctx)
    if err != nil {
        return false, fmt.Errorf("checking for changes: %w", err)
    }
    if !hasChanges {
        return false, nil // No changes to commit
    }

    // 2. Stage all changes
    if _, err := w.runner().Exec(ctx, w.worktreePath, "add", "-A"); err != nil {
        return false, fmt.Errorf("staging changes: %w", err)
    }

    // 3. Commit with standardized message (--no-verify to skip hooks)
    commitMsg := "fix: address code review feedback"
    if _, err := w.runner().Exec(ctx, w.worktreePath, "commit", "-m", commitMsg, "--no-verify"); err != nil {
        return false, fmt.Errorf("committing changes: %w", err)
    }

    return true, nil
}

// hasUncommittedChanges returns true if there are staged or unstaged changes.
func (w *Worker) hasUncommittedChanges(ctx context.Context) (bool, error) {
    // git status --porcelain returns empty string if clean
    output, err := w.runner().Exec(ctx, w.worktreePath, "status", "--porcelain")
    if err != nil {
        return false, err
    }
    return strings.TrimSpace(output) != "", nil
}

// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
// This ensures the worktree is clean before proceeding to merge.
// Errors are logged but not returned - cleanup is best-effort.
func (w *Worker) cleanupWorktree(ctx context.Context) {
    // 1. Reset staged changes (git reset)
    if _, err := w.runner().Exec(ctx, w.worktreePath, "reset", "HEAD"); err != nil {
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git reset failed: %v\n", err)
        }
        // Continue with cleanup
    }

    // 2. Clean untracked files (git clean -fd)
    if _, err := w.runner().Exec(ctx, w.worktreePath, "clean", "-fd"); err != nil {
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git clean failed: %v\n", err)
        }
        // Continue with cleanup
    }

    // 3. Restore modified files (git checkout .)
    if _, err := w.runner().Exec(ctx, w.worktreePath, "checkout", "."); err != nil {
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Warning: git checkout failed: %v\n", err)
        }
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestCommitReviewFixes
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCommitReviewFixes_WithChanges` | Returns (true, nil) when changes exist |
| `TestCommitReviewFixes_NoChanges` | Returns (false, nil) when worktree is clean |
| `TestCommitReviewFixes_StageError` | Returns error when git add fails |
| `TestCommitReviewFixes_CommitError` | Returns error when git commit fails |
| `TestCommitReviewFixes_CommitMessage` | Uses standardized "fix: address code review feedback" message |
| `TestHasUncommittedChanges_Clean` | Returns false for clean worktree |
| `TestHasUncommittedChanges_Modified` | Returns true for modified files |
| `TestHasUncommittedChanges_Untracked` | Returns true for untracked files |
| `TestCleanupWorktree_ResetsStaged` | Calls git reset |
| `TestCleanupWorktree_CleansUntracked` | Calls git clean -fd |
| `TestCleanupWorktree_RestoresModified` | Calls git checkout . |
| `TestCleanupWorktree_ContinuesOnError` | Continues cleanup even if one step fails |

### Test Fixtures

```go
func TestCommitReviewFixes_WithChanges(t *testing.T) {
    // Setup real git repo for this test
    repoDir := setupTestRepo(t)

    // Create a modified file
    testFile := filepath.Join(repoDir, "test.txt")
    require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

    w := &Worker{
        worktreePath: repoDir,
        gitRunner:    git.DefaultRunner(),
    }

    ctx := context.Background()
    committed, err := w.commitReviewFixes(ctx)

    require.NoError(t, err)
    assert.True(t, committed)

    // Verify commit was created
    output, err := w.runner().Exec(ctx, repoDir, "log", "-1", "--pretty=format:%s")
    require.NoError(t, err)
    assert.Equal(t, "fix: address code review feedback", output)
}

func TestCommitReviewFixes_NoChanges(t *testing.T) {
    repoDir := setupTestRepo(t) // Clean repo

    w := &Worker{
        worktreePath: repoDir,
        gitRunner:    git.DefaultRunner(),
    }

    ctx := context.Background()
    committed, err := w.commitReviewFixes(ctx)

    require.NoError(t, err)
    assert.False(t, committed)
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
    repoDir := setupTestRepo(t)

    w := &Worker{
        worktreePath: repoDir,
        gitRunner:    git.DefaultRunner(),
    }

    ctx := context.Background()
    hasChanges, err := w.hasUncommittedChanges(ctx)

    require.NoError(t, err)
    assert.False(t, hasChanges)
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
    repoDir := setupTestRepo(t)

    // Modify existing file
    testFile := filepath.Join(repoDir, "README.md")
    require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

    w := &Worker{
        worktreePath: repoDir,
        gitRunner:    git.DefaultRunner(),
    }

    ctx := context.Background()
    hasChanges, err := w.hasUncommittedChanges(ctx)

    require.NoError(t, err)
    assert.True(t, hasChanges)
}

func TestCleanupWorktree_FullCleanup(t *testing.T) {
    repoDir := setupTestRepo(t)

    // Create dirty state: modified file + untracked file + staged file
    require.NoError(t, os.WriteFile(filepath.Join(repoDir, "modified.txt"), []byte("mod"), 0644))
    require.NoError(t, os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("new"), 0644))

    w := &Worker{
        worktreePath: repoDir,
        gitRunner:    git.DefaultRunner(),
        reviewConfig: &config.CodeReviewConfig{Verbose: true},
    }

    ctx := context.Background()
    w.cleanupWorktree(ctx)

    // Verify worktree is clean
    output, err := w.runner().Exec(ctx, repoDir, "status", "--porcelain")
    require.NoError(t, err)
    assert.Empty(t, strings.TrimSpace(output), "worktree should be clean after cleanup")
}

func TestCleanupWorktree_ContinuesOnError(t *testing.T) {
    // Use mock runner that fails on reset but succeeds on clean/checkout
    mockRunner := &mockGitRunner{
        failCommands: map[string]bool{"reset": true},
    }

    w := &Worker{
        worktreePath: "/tmp/test",
        gitRunner:    mockRunner,
        reviewConfig: &config.CodeReviewConfig{Verbose: true},
    }

    ctx := context.Background()
    // Should not panic even if reset fails
    w.cleanupWorktree(ctx)

    // Should have called all three commands
    assert.Contains(t, mockRunner.commands, "reset")
    assert.Contains(t, mockRunner.commands, "clean")
    assert.Contains(t, mockRunner.commands, "checkout")
}

// setupTestRepo creates a git repo for testing
func setupTestRepo(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()

    runner := git.DefaultRunner()
    ctx := context.Background()

    _, err := runner.Exec(ctx, dir, "init")
    require.NoError(t, err)

    _, err = runner.Exec(ctx, dir, "config", "user.email", "test@test.com")
    require.NoError(t, err)

    _, err = runner.Exec(ctx, dir, "config", "user.name", "Test")
    require.NoError(t, err)

    // Create initial commit
    readmePath := filepath.Join(dir, "README.md")
    require.NoError(t, os.WriteFile(readmePath, []byte("# Test"), 0644))

    _, err = runner.Exec(ctx, dir, "add", ".")
    require.NoError(t, err)

    _, err = runner.Exec(ctx, dir, "commit", "-m", "Initial commit")
    require.NoError(t, err)

    return dir
}

// mockGitRunner for testing error scenarios
type mockGitRunner struct {
    failCommands map[string]bool
    commands     []string
}

func (m *mockGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
    if len(args) > 0 {
        m.commands = append(m.commands, args[0])
        if m.failCommands[args[0]] {
            return "", errors.New("mock error")
        }
    }
    return "", nil
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Commit Message Convention

The commit message `"fix: address code review feedback"` is:
- **Deterministic**: Same message every time for testing predictability
- **Conventional**: Follows conventional commits format
- **Descriptive**: Clearly identifies the purpose

### Git Command Sequence

The cleanup sequence must run in order:
1. `git reset HEAD` - Unstage any staged changes
2. `git clean -fd` - Remove untracked files and directories
3. `git checkout .` - Restore modified tracked files

This order is important:
- Reset before checkout (checkout might fail on staged changes)
- Clean removes files that checkout won't touch
- All three together restore to last commit state

### Error Handling in Cleanup

`cleanupWorktree` logs errors but continues. This is intentional:
- Cleanup is best-effort
- One failing step shouldn't prevent others
- Merge can still proceed even if cleanup is partial
- The function doesn't return errors because callers shouldn't fail based on cleanup status

### --no-verify Flag

The commit uses `--no-verify` to skip pre-commit hooks. This is intentional:
- Review fixes might not pass linting (baseline checks will catch this)
- Speed is more important here than hook validation
- Hooks already ran during task execution

### Existing hasUncommittedChanges Note

The worker already has a `getChangedFiles()` function. We're adding `hasUncommittedChanges()` for a simpler boolean check. Consider whether to reuse `getChangedFiles()` internally:

```go
func (w *Worker) hasUncommittedChanges(ctx context.Context) (bool, error) {
    files, err := w.getChangedFiles(ctx)
    if err != nil {
        return false, err
    }
    return len(files) > 0, nil
}
```

This reuses existing code and maintains consistency.

## NOT In Scope

- Re-running review after cleanup (out of scope)
- Partial cleanup (all-or-nothing)
- Preserving fix attempts that failed (we discard them)
