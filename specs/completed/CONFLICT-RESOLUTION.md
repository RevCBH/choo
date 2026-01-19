# CONFLICT-RESOLUTION - Merge Conflict Detection and Claude-Delegated Resolution

## Overview

The CONFLICT-RESOLUTION module handles merge conflicts that arise when multiple units merge to main in parallel. When a worker attempts to rebase its branch onto the target branch and encounters conflicts, the system delegates resolution to Claude Code rather than attempting automated resolution.

The orchestrator's role is limited to detection, delegation, verification, and final merge operations. Claude performs the actual conflict resolution by editing files, removing conflict markers, and staging the resolved files. After successful resolution, the orchestrator force-pushes with lease and completes the merge via GitHub API.

```
┌─────────────────────────────────────────────────────────────────┐
│                   Merge Conflict Flow                           │
└─────────────────────────────────────────────────────────────────┘

  ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
  │  Fetch  │────▶│ Rebase  │────▶│Conflict?│────▶│  Merge  │
  │ Latest  │     │         │     │         │     │         │
  └─────────┘     └─────────┘     └─────────┘     └─────────┘
                                       │
                                       │ Yes
                                       ▼
                                 ┌───────────┐
                                 │  Invoke   │
                                 │  Claude   │
                                 │  to Fix   │
                                 └───────────┘
                                       │
                                       ▼
                                 ┌───────────┐
                                 │  Verify   │
                                 │ Resolved  │
                                 └───────────┘
                                       │
                          ┌────────────┴────────────┐
                          │                         │
                          ▼                         ▼
                    ┌───────────┐            ┌───────────┐
                    │   Force   │            │ Escalate  │
                    │   Push    │            │ to User   │
                    └───────────┘            └───────────┘
```

## Requirements

### Functional Requirements

1. Detect merge conflicts during rebase onto target branch
2. Extract list of conflicted files from git state
3. Build a conflict resolution prompt for Claude with file list and instructions
4. Delegate conflict resolution to Claude via CLI subprocess
5. Verify rebase completed (no longer in rebase state) after Claude invocation
6. Retry conflict resolution with exponential backoff on failure
7. Abort rebase and escalate to user after max retry attempts
8. Force push with lease after successful resolution
9. Merge PR via GitHub API after successful push
10. Emit PRConflict event when conflicts detected
11. Emit PRMerged event after successful merge

### Performance Requirements

| Metric | Target |
|--------|--------|
| Conflict detection latency | <100ms |
| Prompt construction | <10ms |
| Verification check | <50ms |
| Force push | <5s |
| Total resolution (excluding Claude) | <10s |

### Constraints

- Depends on: `internal/git` (rebase, conflict detection, force push)
- Depends on: `internal/github` (PR merge)
- Depends on: `internal/events` (event emission)
- Depends on: Claude CLI subprocess for resolution
- Must not block other merge operations longer than necessary
- Rebase must be aborted on unrecoverable failure to leave repo clean

## Design

### Module Structure

```
internal/worker/
├── merge.go           # Conflict resolution and merge logic
├── prompt_git.go      # Conflict resolution prompt builder
└── retry.go           # Retry with backoff utilities
```

### Core Types

```go
// internal/worker/merge.go

// MergeConfig holds configuration for merge operations
type MergeConfig struct {
    // TargetBranch is the branch to rebase onto and merge into
    TargetBranch string

    // MaxConflictRetries is the max attempts for conflict resolution
    MaxConflictRetries int

    // RetryConfig configures backoff behavior
    RetryConfig RetryConfig
}

// ConflictInfo contains information about detected conflicts
type ConflictInfo struct {
    // Files is the list of files with conflicts
    Files []string

    // TargetBranch is the branch being rebased onto
    TargetBranch string

    // SourceBranch is the branch being rebased
    SourceBranch string
}
```

```go
// internal/worker/retry.go

// RetryConfig configures retry behavior with exponential backoff
type RetryConfig struct {
    // MaxAttempts is the maximum number of retry attempts
    MaxAttempts int

    // InitialDelay is the delay before the first retry
    InitialDelay time.Duration

    // MaxDelay is the maximum delay between retries
    MaxDelay time.Duration

    // Multiplier is the factor by which delay increases each attempt
    Multiplier float64
}

// RetryResult holds the outcome of a retry operation
type RetryResult struct {
    // Success indicates if any attempt succeeded
    Success bool

    // Attempts is how many attempts were made
    Attempts int

    // LastErr is the error from the final attempt (nil if Success)
    LastErr error
}

// DefaultRetryConfig returns sensible defaults for conflict resolution
var DefaultRetryConfig = RetryConfig{
    MaxAttempts:  3,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
}
```

```go
// internal/escalate/escalate.go

// Severity indicates how urgent the escalation is
type Severity string

const (
    SeverityInfo     Severity = "info"
    SeverityWarning  Severity = "warning"
    SeverityBlocking Severity = "blocking"
)

// Escalation represents an issue that requires user attention
type Escalation struct {
    // Severity indicates urgency
    Severity Severity

    // Unit is the unit ID this escalation relates to
    Unit string

    // Title is a short description
    Title string

    // Message is a detailed explanation
    Message string

    // Context contains additional key-value details
    Context map[string]string
}

// Escalator handles user escalations
type Escalator interface {
    Escalate(ctx context.Context, e Escalation) error
}
```

### API Surface

```go
// internal/worker/merge.go

// mergeWithConflictResolution performs a full merge with conflict handling
// This is called by the worker after PR approval
func (w *Worker) mergeWithConflictResolution(ctx context.Context) error

// forcePushAndMerge pushes the rebased branch and merges via GitHub API
func (w *Worker) forcePushAndMerge(ctx context.Context) error
```

```go
// internal/worker/prompt_git.go

// BuildConflictPrompt creates the prompt for Claude to resolve merge conflicts
func BuildConflictPrompt(targetBranch string, conflictedFiles []string) string

// formatFileList formats a slice of file paths for display in prompts
func formatFileList(files []string) string
```

```go
// internal/worker/retry.go

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) RetryResult
```

```go
// internal/git/merge.go (additions)

// IsRebaseInProgress checks if a rebase is currently in progress
func IsRebaseInProgress(ctx context.Context, worktreePath string) (bool, error)

// AbortRebase aborts an in-progress rebase
func AbortRebase(ctx context.Context, worktreePath string) error

// GetConflictedFiles returns the list of files with merge conflicts
func GetConflictedFiles(ctx context.Context, worktreePath string) ([]string, error)
```

### Conflict Resolution Flow

```
mergeWithConflictResolution()
    │
    ▼
┌──────────────────────────────┐
│ 1. Fetch latest from origin  │
│    git fetch origin <target> │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 2. Rebase onto target        │
│    git rebase origin/<target>│
└──────────────┬───────────────┘
               │
      ┌────────┴────────┐
      │                 │
      ▼                 ▼
 No Conflicts      Has Conflicts
      │                 │
      │                 ▼
      │      ┌──────────────────────────────┐
      │      │ 3. Emit PRConflict event     │
      │      └──────────────┬───────────────┘
      │                     │
      │                     ▼
      │      ┌──────────────────────────────┐
      │      │ 4. Get conflicted file list  │
      │      │    git diff --name-only      │
      │      │    --diff-filter=U           │
      │      └──────────────┬───────────────┘
      │                     │
      │                     ▼
      │      ┌──────────────────────────────┐
      │      │ 5. Build conflict prompt     │
      │      └──────────────┬───────────────┘
      │                     │
      │                     ▼
      │      ┌──────────────────────────────────────┐
      │      │ 6. Retry loop with backoff:          │
      │      │    a. Invoke Claude CLI              │
      │      │    b. Check if rebase still active   │
      │      │    c. If still in rebase → retry     │
      │      │    d. If rebase done → success       │
      │      └──────────────┬───────────────────────┘
      │                     │
      │        ┌────────────┴────────────┐
      │        │                         │
      │        ▼                         ▼
      │   Resolution OK           Max Retries
      │        │                         │
      │        │                         ▼
      │        │              ┌──────────────────────────────┐
      │        │              │ 7a. Abort rebase             │
      │        │              │     git rebase --abort       │
      │        │              └──────────────┬───────────────┘
      │        │                             │
      │        │                             ▼
      │        │              ┌──────────────────────────────┐
      │        │              │ 7b. Escalate to user         │
      │        │              │     (blocking severity)      │
      │        │              └──────────────┬───────────────┘
      │        │                             │
      │        │                             ▼
      │        │                        Return Error
      │        │
      └────────┼───────────────────────────────────────┐
               │                                       │
               ▼                                       │
┌──────────────────────────────┐                       │
│ 8. Force push with lease     │◀──────────────────────┘
│    git push --force-with-    │
│    lease                     │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 9. Merge via GitHub API      │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 10. Emit PRMerged event      │
└──────────────────────────────┘
```

### Conflict Resolution Prompt

The prompt instructs Claude to resolve conflicts manually:

```go
// internal/worker/prompt_git.go

func BuildConflictPrompt(targetBranch string, conflictedFiles []string) string {
    return fmt.Sprintf(`The rebase onto %s resulted in merge conflicts.

Conflicted files:
%s

Please resolve all conflicts:
1. Open each conflicted file
2. Find the conflict markers (<<<<<<, =======, >>>>>>>)
3. Edit to resolve - keep the correct code, remove markers
4. Stage resolved files: git add <file>
5. Continue the rebase: git rebase --continue

If the rebase continues successfully, do NOT push - the orchestrator will handle that.

If you cannot resolve a conflict, explain why in your response.`, targetBranch, formatFileList(conflictedFiles))
}

func formatFileList(files []string) string {
    var sb strings.Builder
    for _, f := range files {
        fmt.Fprintf(&sb, "- %s\n", f)
    }
    return sb.String()
}
```

### Retry with Backoff

```go
// internal/worker/retry.go

func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) RetryResult {
    result := RetryResult{
        Success:  false,
        Attempts: 0,
    }

    delay := cfg.InitialDelay

    for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
        result.Attempts = attempt

        err := fn(ctx)
        if err == nil {
            result.Success = true
            result.LastErr = nil
            return result
        }

        result.LastErr = err

        // Don't sleep after last attempt
        if attempt < cfg.MaxAttempts {
            select {
            case <-ctx.Done():
                result.LastErr = ctx.Err()
                return result
            case <-time.After(delay):
            }

            // Exponential backoff with cap
            delay = time.Duration(float64(delay) * cfg.Multiplier)
            if delay > cfg.MaxDelay {
                delay = cfg.MaxDelay
            }
        }
    }

    return result
}
```

### Merge Implementation

```go
// internal/worker/merge.go

func (w *Worker) mergeWithConflictResolution(ctx context.Context) error {
    // Fetch latest
    if err := git.Fetch(ctx, w.config.RepoRoot, w.config.TargetBranch); err != nil {
        return fmt.Errorf("fetch failed: %w", err)
    }

    // Try rebase
    targetRef := fmt.Sprintf("origin/%s", w.config.TargetBranch)
    hasConflicts, err := git.Rebase(ctx, w.worktreePath, targetRef)
    if err != nil {
        return fmt.Errorf("rebase failed: %w", err)
    }

    if !hasConflicts {
        // No conflicts, force push and merge
        return w.forcePushAndMerge(ctx)
    }

    // Get conflicted files
    conflictedFiles, err := git.GetConflictedFiles(ctx, w.worktreePath)
    if err != nil {
        return fmt.Errorf("failed to get conflicted files: %w", err)
    }

    // Emit conflict event
    if w.events != nil {
        evt := events.NewEvent(events.PRConflict, w.unit.ID).
            WithPR(w.prNumber).
            WithPayload(map[string]any{
                "files": conflictedFiles,
            })
        w.events.Emit(evt)
    }

    // Delegate conflict resolution to Claude
    prompt := BuildConflictPrompt(w.config.TargetBranch, conflictedFiles)

    retryResult := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify rebase completed (no longer in rebase state)
        inRebase, err := git.IsRebaseInProgress(ctx, w.worktreePath)
        if err != nil {
            return err
        }
        if inRebase {
            // Claude didn't complete the rebase
            return fmt.Errorf("claude did not complete rebase")
        }
        return nil
    })

    if !retryResult.Success {
        // Clean up - abort the rebase
        _ = git.AbortRebase(ctx, w.worktreePath)

        // Escalate to user
        if w.escalator != nil {
            w.escalator.Escalate(ctx, escalate.Escalation{
                Severity: escalate.SeverityBlocking,
                Unit:     w.unit.ID,
                Title:    "Failed to resolve merge conflicts",
                Message: fmt.Sprintf(
                    "Claude could not resolve conflicts after %d attempts",
                    retryResult.Attempts,
                ),
                Context: map[string]string{
                    "files":  strings.Join(conflictedFiles, ", "),
                    "target": w.config.TargetBranch,
                    "error":  retryResult.LastErr.Error(),
                },
            })
        }

        return retryResult.LastErr
    }

    return w.forcePushAndMerge(ctx)
}

func (w *Worker) forcePushAndMerge(ctx context.Context) error {
    // Force push the rebased branch
    if err := git.ForcePushWithLease(ctx, w.worktreePath); err != nil {
        return fmt.Errorf("force push failed: %w", err)
    }

    // Merge via GitHub API
    if err := w.github.MergePR(ctx, w.prNumber); err != nil {
        return fmt.Errorf("merge failed: %w", err)
    }

    // Emit PRMerged event
    if w.events != nil {
        evt := events.NewEvent(events.PRMerged, w.unit.ID).WithPR(w.prNumber)
        w.events.Emit(evt)
    }

    return nil
}
```

### Git Operations for Conflict Detection

```go
// internal/git/merge.go (additions)

// IsRebaseInProgress checks if a rebase is currently in progress
func IsRebaseInProgress(ctx context.Context, worktreePath string) (bool, error) {
    // Check for .git/rebase-merge or .git/rebase-apply directories
    gitDir := filepath.Join(worktreePath, ".git")

    // For worktrees, .git is a file pointing to the actual git dir
    gitDirContent, err := os.ReadFile(gitDir)
    if err == nil && strings.HasPrefix(string(gitDirContent), "gitdir:") {
        // This is a worktree, extract the actual git dir
        gitDir = strings.TrimSpace(strings.TrimPrefix(string(gitDirContent), "gitdir:"))
    }

    // Check for rebase-merge (interactive rebase)
    if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
        return true, nil
    }

    // Check for rebase-apply (non-interactive rebase)
    if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
        return true, nil
    }

    return false, nil
}

// AbortRebase aborts an in-progress rebase
func AbortRebase(ctx context.Context, worktreePath string) error {
    _, err := gitExec(ctx, worktreePath, "rebase", "--abort")
    return err
}

// GetConflictedFiles returns the list of files with merge conflicts
func GetConflictedFiles(ctx context.Context, worktreePath string) ([]string, error) {
    out, err := gitExec(ctx, worktreePath, "diff", "--name-only", "--diff-filter=U")
    if err != nil {
        return nil, err
    }

    out = strings.TrimSpace(out)
    if out == "" {
        return []string{}, nil
    }

    return strings.Split(out, "\n"), nil
}
```

## Implementation Notes

### Rebase State Detection

Git stores rebase state in `.git/rebase-merge/` (for interactive rebase) or `.git/rebase-apply/` (for non-interactive). For worktrees, the `.git` file contains a `gitdir:` pointer to the actual git directory. The implementation must handle both cases.

### Force Push Safety

Using `--force-with-lease` instead of `--force` prevents overwriting changes pushed by others. If someone else pushes to the branch between our rebase and push, the push will fail safely rather than overwriting their changes.

### Escalation Strategy

When conflict resolution fails after all retries:
1. Abort the rebase to leave the repository in a clean state
2. Escalate with `SeverityBlocking` to notify the user immediately
3. Include context about which files conflicted and the target branch
4. Return the error so the worker can handle failure appropriately

### Claude Invocation

The worker invokes Claude via CLI subprocess:

```go
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
    args := []string{
        "--dangerously-skip-permissions",
        "-p", prompt,
    }

    cmd := exec.CommandContext(ctx, "claude", args...)
    cmd.Dir = w.worktreePath

    return cmd.Run()
}
```

### Event Emission

Two events are emitted during the conflict resolution flow:

| Event | When | Payload |
|-------|------|---------|
| PRConflict | Conflicts detected during rebase | `{"files": ["path/to/file1.go", "path/to/file2.go"]}` |
| PRMerged | PR successfully merged | - |

## Testing Strategy

### Unit Tests

```go
// internal/worker/prompt_git_test.go

func TestBuildConflictPrompt_SingleFile(t *testing.T) {
    prompt := BuildConflictPrompt("main", []string{"src/config.go"})

    if !strings.Contains(prompt, "main") {
        t.Error("prompt should contain target branch")
    }
    if !strings.Contains(prompt, "src/config.go") {
        t.Error("prompt should contain conflicted file")
    }
    if !strings.Contains(prompt, "git rebase --continue") {
        t.Error("prompt should instruct to continue rebase")
    }
}

func TestBuildConflictPrompt_MultipleFiles(t *testing.T) {
    files := []string{
        "src/config.go",
        "src/worker/merge.go",
        "internal/git/rebase.go",
    }

    prompt := BuildConflictPrompt("main", files)

    for _, f := range files {
        if !strings.Contains(prompt, f) {
            t.Errorf("prompt should contain file %s", f)
        }
    }
}

func TestFormatFileList(t *testing.T) {
    files := []string{"a.go", "b.go", "c.go"}
    result := formatFileList(files)

    expected := "- a.go\n- b.go\n- c.go\n"
    if result != expected {
        t.Errorf("expected %q, got %q", expected, result)
    }
}
```

```go
// internal/worker/retry_test.go

func TestRetryWithBackoff_SuccessFirstAttempt(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond,
        MaxDelay:     100 * time.Millisecond,
        Multiplier:   2.0,
    }

    result := RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        return nil // Success on first attempt
    })

    if !result.Success {
        t.Error("expected success")
    }
    if result.Attempts != 1 {
        t.Errorf("expected 1 attempt, got %d", result.Attempts)
    }
    if result.LastErr != nil {
        t.Errorf("expected no error, got %v", result.LastErr)
    }
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond,
        MaxDelay:     100 * time.Millisecond,
        Multiplier:   2.0,
    }

    attempt := 0
    result := RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        attempt++
        if attempt < 3 {
            return fmt.Errorf("attempt %d failed", attempt)
        }
        return nil // Success on third attempt
    })

    if !result.Success {
        t.Error("expected success")
    }
    if result.Attempts != 3 {
        t.Errorf("expected 3 attempts, got %d", result.Attempts)
    }
}

func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond,
        MaxDelay:     100 * time.Millisecond,
        Multiplier:   2.0,
    }

    expectedErr := fmt.Errorf("permanent failure")
    result := RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        return expectedErr
    })

    if result.Success {
        t.Error("expected failure")
    }
    if result.Attempts != 3 {
        t.Errorf("expected 3 attempts, got %d", result.Attempts)
    }
    if result.LastErr != expectedErr {
        t.Errorf("expected %v, got %v", expectedErr, result.LastErr)
    }
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:  5,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     1 * time.Second,
        Multiplier:   2.0,
    }

    ctx, cancel := context.WithCancel(context.Background())

    attempt := 0
    result := RetryWithBackoff(ctx, cfg, func(ctx context.Context) error {
        attempt++
        if attempt == 2 {
            cancel() // Cancel during second attempt's delay
        }
        return fmt.Errorf("failed")
    })

    if result.Success {
        t.Error("expected failure due to cancellation")
    }
    if result.Attempts > 3 {
        t.Errorf("expected early exit, got %d attempts", result.Attempts)
    }
}

func TestRetryWithBackoff_ExponentialDelay(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:  4,
        InitialDelay: 10 * time.Millisecond,
        MaxDelay:     100 * time.Millisecond,
        Multiplier:   2.0,
    }

    var timestamps []time.Time
    RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        timestamps = append(timestamps, time.Now())
        return fmt.Errorf("fail")
    })

    // Check delays are approximately exponential
    // Expected: 10ms, 20ms, 40ms (capped at 100ms)
    if len(timestamps) != 4 {
        t.Fatalf("expected 4 timestamps, got %d", len(timestamps))
    }

    delay1 := timestamps[1].Sub(timestamps[0])
    delay2 := timestamps[2].Sub(timestamps[1])
    delay3 := timestamps[3].Sub(timestamps[2])

    // Allow 50% tolerance for timing
    assertDelayInRange(t, delay1, 10*time.Millisecond, 0.5)
    assertDelayInRange(t, delay2, 20*time.Millisecond, 0.5)
    assertDelayInRange(t, delay3, 40*time.Millisecond, 0.5)
}

func assertDelayInRange(t *testing.T, actual, expected time.Duration, tolerance float64) {
    t.Helper()
    min := time.Duration(float64(expected) * (1 - tolerance))
    max := time.Duration(float64(expected) * (1 + tolerance))
    if actual < min || actual > max {
        t.Errorf("delay %v not in range [%v, %v]", actual, min, max)
    }
}
```

```go
// internal/git/merge_test.go

func TestGetConflictedFiles_NoConflicts(t *testing.T) {
    // Create a clean git repo
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    files, err := GetConflictedFiles(context.Background(), dir)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(files) != 0 {
        t.Errorf("expected no conflicted files, got %v", files)
    }
}

func TestIsRebaseInProgress_NotInRebase(t *testing.T) {
    // Create a clean git repo
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    inRebase, err := IsRebaseInProgress(context.Background(), dir)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if inRebase {
        t.Error("expected not in rebase")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Clean rebase | Create branch, make commits, rebase onto updated main (no conflicts) |
| Conflict detection | Create conflicting changes in same file on two branches |
| Successful resolution | Mock Claude resolving conflicts, verify rebase completes |
| Resolution failure | Mock Claude failing to resolve, verify escalation |
| Force push with lease | After rebase, verify push succeeds with lease |
| Concurrent modification | Simulate someone else pushing during resolution |

### Manual Testing

- [ ] Rebase detects conflicts when same file modified in both branches
- [ ] Conflict prompt includes all conflicted files
- [ ] Claude invocation receives correct working directory
- [ ] Rebase state is correctly detected via filesystem check
- [ ] Retry backoff increases exponentially between attempts
- [ ] Escalation fires after max retries exhausted
- [ ] Rebase is aborted on escalation
- [ ] Force push with lease succeeds after resolution
- [ ] Force push with lease fails if branch modified by others
- [ ] PRConflict event contains file list
- [ ] PRMerged event emits after successful merge

## Design Decisions

### Why Delegate to Claude Instead of Auto-Merge?

Git's automatic merge strategies (`-X theirs`, `-X ours`, recursive) can resolve some conflicts but often produce incorrect results for semantic conflicts. Claude can understand code context and make intelligent decisions about which changes to keep. This matches the PRD requirement for human-level conflict resolution.

### Why Verify Rebase State Instead of Checking File Contents?

Checking for rebase state via filesystem (`.git/rebase-merge/` or `.git/rebase-apply/`) is:
1. More reliable than parsing git status output
2. Faster than re-reading all conflicted files
3. The canonical way to determine git operation state

### Why Exponential Backoff?

Exponential backoff prevents overwhelming the system if Claude is experiencing transient issues. The delays (1s, 2s, 4s with 30s cap) provide enough time for temporary issues to resolve while not waiting excessively.

### Why Abort Rebase on Failure?

Leaving a rebase in progress would block future git operations in the worktree. Aborting returns the repository to a clean state where the user can manually investigate and retry. The escalation provides context about what failed.

### Why Force Push with Lease?

`--force-with-lease` is safer than `--force` because it verifies the remote branch hasn't been updated since we last fetched. This prevents accidentally overwriting someone else's changes in edge cases where multiple developers or systems might touch the same branch.

### Why No Fallback Resolution?

The PRD specifies escalation to user after max retries rather than a fallback strategy. Fallback strategies (like `-X theirs` or `-X ours`) can silently introduce bugs. Escalating ensures a human reviews difficult conflicts.

## Configuration

```yaml
# choo.yaml merge configuration
merge:
  strategy: squash                # squash | merge | rebase
  max_conflict_retries: 3         # Max Claude attempts for conflicts
  retry_initial_delay: 1s         # Initial retry delay
  retry_max_delay: 30s            # Maximum retry delay
  retry_multiplier: 2.0           # Backoff multiplier
```

## Future Enhancements

1. Conflict preview - Show predicted conflicts before attempting rebase
2. Selective resolution - Allow Claude to resolve some conflicts while escalating others
3. Conflict history - Track common conflict patterns for learning
4. Parallel conflict resolution - Allow multiple Claude instances for different files
5. Smart retry - Adjust retry strategy based on conflict type

## References

- [PRD Section 7: Merge Conflict Flow](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [GIT Spec: Merge Serialization](/Users/bennett/conductor/workspaces/choo/las-vegas/specs/completed/GIT.md)
- [EVENTS Spec: PRConflict, PRMerged Events](/Users/bennett/conductor/workspaces/choo/las-vegas/specs/completed/EVENTS.md)
- [Git Rebase Documentation](https://git-scm.com/docs/git-rebase)
- [Git Force Push with Lease](https://git-scm.com/docs/git-push#Documentation/git-push.txt---force-with-leaseltrefnamegt)
