# CLAUDE-GIT - Delegation of Git Operations to Claude Code

## Overview

The CLAUDE-GIT component delegates git operations (commit, push, PR creation) to Claude Code instead of having the orchestrator execute them directly. The orchestrator invokes Claude with specific prompts for each operation and verifies outcomes, maintaining a coordination role rather than an execution role.

Claude Code already handles the hard work - implementing tasks, resolving conflicts, fixing baseline failures. Git operations like staging, committing, and pushing are simpler by comparison. Delegating these to Claude provides richer commit messages, contextual PR descriptions, and a consistent model where Claude handles all creative work while the orchestrator coordinates.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Current vs. Proposed Delegation                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   CURRENT MODEL                    PROPOSED MODEL                        │
│   ┌─────────────────┐              ┌─────────────────┐                  │
│   │   Orchestrator  │              │   Orchestrator  │                  │
│   │  ┌───────────┐  │              │  ┌───────────┐  │                  │
│   │  │git add -A │  │              │  │  Invoke   │  │                  │
│   │  │git commit │  │     ──▶      │  │  Claude   │  │                  │
│   │  │git push   │  │              │  │  Verify   │  │                  │
│   │  │gh pr      │  │              │  └───────────┘  │                  │
│   │  └───────────┘  │              └────────┬────────┘                  │
│   └────────┬────────┘                       │                           │
│            │                                ▼                            │
│            ▼                       ┌─────────────────┐                  │
│   ┌─────────────────┐              │   Claude Code   │                  │
│   │   Claude Code   │              │  ┌───────────┐  │                  │
│   │  ┌───────────┐  │              │  │git add -A │  │                  │
│   │  │Task work  │  │              │  │git commit │  │                  │
│   │  │Conflicts  │  │              │  │git push   │  │                  │
│   │  └───────────┘  │              │  │gh pr      │  │                  │
│   └─────────────────┘              │  └───────────┘  │                  │
│                                    └─────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Operation Delegation Matrix

| Operation | Current | Proposed |
|-----------|---------|----------|
| File editing | Claude Code | Claude Code (unchanged) |
| Staging | Orchestrator | Claude Code |
| Commit message | Template | Claude Code writes |
| Git push | Orchestrator | Claude Code |
| PR creation | Direct `gh` call | Claude Code via `gh` |
| PR description | Template | Claude Code writes |
| Conflict resolution | Claude Code | Claude Code (unchanged) |
| PR merge | GitHub API | GitHub API (unchanged) |

### Benefits

1. **Rich commit messages** - Claude understands *why* changes were made and can articulate this
2. **Contextual PR descriptions** - Summarizes implementation approach from actual work done
3. **Simpler orchestrator** - Less Go code to maintain, fewer execution paths
4. **Natural workflow** - Mirrors how a human developer works with their tools
5. **Consistent delegation model** - Claude handles all creative work; orchestrator coordinates

## Requirements

### Functional Requirements

1. Invoke Claude Code to stage and commit changes after task completion
2. Verify commit was created by checking for new commits on the branch
3. Invoke Claude Code to push branches to remote origin
4. Verify branch exists on remote after push operation
5. Invoke Claude Code to create PRs via `gh pr create` with contextual descriptions
6. Extract and capture PR URL from Claude's output
7. Retry all operations with exponential backoff on transient failures
8. Escalate to user after maximum retries exhausted (no fallback to direct operations)
9. Support configurable retry parameters (max attempts, backoff timing)

### Performance Requirements

| Metric | Target |
|--------|--------|
| Commit operation (Claude invoke + verify) | <30s |
| Push operation (Claude invoke + verify) | <60s |
| PR creation (Claude invoke + URL extract) | <60s |
| Retry initial backoff | 1s |
| Retry max backoff | 30s |

### Constraints

- Claude invocations MUST use subprocess, NEVER the Claude API directly
- Verification MUST use git commands, not trust Claude's output
- Retries apply to ALL errors (transient network, rate limits, etc.)
- No fallback to direct git operations - escalate on persistent failure
- Depends on: `internal/worker`, `internal/git`, `internal/events`, `internal/escalate`

## Design

### Module Structure

```
internal/worker/
├── prompt_git.go       # Git operation prompt builders
├── git_delegate.go     # Claude delegation for git operations
├── retry.go            # Retry with exponential backoff
└── ...                 # Existing worker files
```

### Core Types

```go
// internal/worker/retry.go

// RetryConfig controls retry behavior for git operations
type RetryConfig struct {
    // MaxAttempts is the maximum number of attempts before giving up
    MaxAttempts int

    // InitialBackoff is the delay before the first retry
    InitialBackoff time.Duration

    // MaxBackoff is the maximum delay between retries
    MaxBackoff time.Duration

    // BackoffMultiply is the factor to multiply backoff by after each attempt
    BackoffMultiply float64
}

// DefaultRetryConfig provides sensible defaults for git operations
var DefaultRetryConfig = RetryConfig{
    MaxAttempts:     3,
    InitialBackoff:  1 * time.Second,
    MaxBackoff:      30 * time.Second,
    BackoffMultiply: 2.0,
}

// RetryResult indicates the outcome of a retried operation
type RetryResult struct {
    // Success indicates if the operation eventually succeeded
    Success bool

    // Attempts is how many attempts were made
    Attempts int

    // LastErr is the error from the final failed attempt (if any)
    LastErr error
}
```

```go
// internal/worker/git_delegate.go

// GitDelegateConfig configures the git delegation behavior
type GitDelegateConfig struct {
    // Retry configuration for all git operations
    Retry RetryConfig

    // TargetBranch is the branch PRs will target (e.g., "main")
    TargetBranch string
}

// GitVerifier provides methods to verify git operation outcomes
type GitVerifier interface {
    // HasNewCommit checks if a new commit was created since the given ref
    HasNewCommit(ctx context.Context, worktreePath, sinceRef string) (bool, error)

    // BranchExistsOnRemote checks if a branch exists on the remote
    BranchExistsOnRemote(ctx context.Context, worktreePath, branch string) (bool, error)

    // GetChangedFiles returns list of changed files in the worktree
    GetChangedFiles(ctx context.Context, worktreePath string) ([]string, error)
}
```

### API Surface

```go
// internal/worker/prompt_git.go

// BuildCommitPrompt creates a prompt for Claude to commit changes
func BuildCommitPrompt(taskTitle string, files []string) string

// BuildPushPrompt creates a prompt for Claude to push the branch
func BuildPushPrompt(branch string) string

// BuildPRPrompt creates a prompt for Claude to create a PR
func BuildPRPrompt(branch, targetBranch, unitTitle string) string
```

```go
// internal/worker/git_delegate.go

// commitViaClaudeCode invokes Claude to stage and commit changes
func (w *Worker) commitViaClaudeCode(ctx context.Context, taskTitle string) error

// pushViaClaudeCode invokes Claude to push the branch to remote
func (w *Worker) pushViaClaudeCode(ctx context.Context) error

// createPRViaClaudeCode invokes Claude to create a PR and returns the URL
func (w *Worker) createPRViaClaudeCode(ctx context.Context) (string, error)
```

```go
// internal/worker/retry.go

// RetryWithBackoff retries an operation with exponential backoff
// Retries on ANY error - assumes Claude failures are transient
func RetryWithBackoff(
    ctx context.Context,
    cfg RetryConfig,
    operation func(ctx context.Context) error,
) RetryResult
```

### Delegation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                  Claude Git Delegation Flow                      │
└─────────────────────────────────────────────────────────────────┘

  Task Complete
       │
       ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Invoke    │────▶│   Verify    │────▶│   Invoke    │
│   Claude:   │     │   Commit    │     │   Claude:   │
│   Commit    │     │   Exists    │     │   Push      │
└─────────────┘     └─────────────┘     └─────────────┘
                          │                    │
                    retry with                 │
                    backoff                    ▼
                          │             ┌─────────────┐
                          │             │   Verify    │
                          ▼             │   Branch    │
                    ┌───────────┐       │   Pushed    │
                    │ Escalate  │       └─────────────┘
                    │ to User   │             │
                    └───────────┘             │
                                              ▼
                                   (After all tasks)
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Invoke    │
                                 │   Claude:   │
                                 │   Create PR │
                                 └─────────────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Verify    │
                                 │   PR URL    │
                                 └─────────────┘
```

### Prompt Builders

```go
// internal/worker/prompt_git.go

package worker

import (
    "fmt"
    "strings"
)

// BuildCommitPrompt creates a prompt for Claude to commit changes
func BuildCommitPrompt(taskTitle string, files []string) string {
    return fmt.Sprintf(`Task "%s" is complete.

Stage and commit the changes:
1. Run: git add -A
2. Run: git commit with a conventional commit message

Guidelines for the commit message:
- Use conventional commit format (feat:, fix:, refactor:, etc.)
- First line: concise summary of what changed (50 chars or less)
- If needed, add a blank line then detailed explanation
- Explain WHY, not just WHAT

Files changed:
%s

Do NOT push yet. Just stage and commit.`, taskTitle, formatFileList(files))
}

// BuildPushPrompt creates a prompt for Claude to push the branch
func BuildPushPrompt(branch string) string {
    return fmt.Sprintf(`Push the branch to origin:

git push -u origin %s

If the push fails due to a transient error (network, etc.), that's okay -
the orchestrator will retry. Just attempt the push.`, branch)
}

// BuildPRPrompt creates a prompt for Claude to create a PR
func BuildPRPrompt(branch, targetBranch, unitTitle string) string {
    return fmt.Sprintf(`All tasks for unit "%s" are complete.

Create a pull request:
- Source branch: %s
- Target branch: %s

Use the gh CLI:
  gh pr create --base %s --head %s --title "..." --body "..."

Guidelines for the PR:
- Title: Clear, concise summary of the unit's purpose
- Body:
  - Brief overview of what was implemented
  - Key changes or decisions made
  - Any notes for reviewers

Print the PR URL when done so the orchestrator can capture it.`, unitTitle, branch, targetBranch, targetBranch, branch)
}

// formatFileList formats a list of files for inclusion in prompts
func formatFileList(files []string) string {
    if len(files) == 0 {
        return "(no files listed)"
    }
    var result strings.Builder
    for _, f := range files {
        result.WriteString("- ")
        result.WriteString(f)
        result.WriteString("\n")
    }
    return result.String()
}
```

### Retry Implementation

```go
// internal/worker/retry.go

package worker

import (
    "context"
    "time"
)

// RetryConfig controls retry behavior
type RetryConfig struct {
    MaxAttempts     int
    InitialBackoff  time.Duration
    MaxBackoff      time.Duration
    BackoffMultiply float64
}

var DefaultRetryConfig = RetryConfig{
    MaxAttempts:     3,
    InitialBackoff:  1 * time.Second,
    MaxBackoff:      30 * time.Second,
    BackoffMultiply: 2.0,
}

// RetryResult indicates what happened
type RetryResult struct {
    Success  bool
    Attempts int
    LastErr  error
}

// RetryWithBackoff retries an operation with exponential backoff
// It retries on ANY error - the assumption is that Claude failures
// are transient (network, rate limits, etc.)
func RetryWithBackoff(
    ctx context.Context,
    cfg RetryConfig,
    operation func(ctx context.Context) error,
) RetryResult {
    var lastErr error
    backoff := cfg.InitialBackoff

    for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
        err := operation(ctx)
        if err == nil {
            return RetryResult{Success: true, Attempts: attempt}
        }

        lastErr = err

        if attempt < cfg.MaxAttempts {
            select {
            case <-ctx.Done():
                return RetryResult{Success: false, Attempts: attempt, LastErr: ctx.Err()}
            case <-time.After(backoff):
            }

            // Exponential backoff
            backoff = time.Duration(float64(backoff) * cfg.BackoffMultiply)
            if backoff > cfg.MaxBackoff {
                backoff = cfg.MaxBackoff
            }
        }
    }

    return RetryResult{Success: false, Attempts: cfg.MaxAttempts, LastErr: lastErr}
}
```

### Worker Integration

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "fmt"
    "regexp"

    "github.com/anthropics/choo/internal/escalate"
    "github.com/anthropics/choo/internal/events"
)

// commitViaClaudeCode invokes Claude to stage and commit
func (w *Worker) commitViaClaudeCode(ctx context.Context, taskTitle string) error {
    // Get the HEAD ref before invoking Claude
    headBefore, err := w.getHeadRef(ctx)
    if err != nil {
        return fmt.Errorf("failed to get HEAD ref: %w", err)
    }

    files, _ := w.getChangedFiles(ctx)
    prompt := BuildCommitPrompt(taskTitle, files)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify commit was created
        hasCommit, err := w.hasNewCommit(ctx, headBefore)
        if err != nil {
            return err
        }
        if !hasCommit {
            return fmt.Errorf("claude did not create a commit")
        }
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to commit changes",
            Message:  fmt.Sprintf("Claude could not commit after %d attempts", result.Attempts),
            Context: map[string]string{
                "task":  taskTitle,
                "error": result.LastErr.Error(),
            },
        })
        return result.LastErr
    }

    return nil
}

// pushViaClaudeCode invokes Claude to push the branch
func (w *Worker) pushViaClaudeCode(ctx context.Context) error {
    prompt := BuildPushPrompt(w.branch)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify branch exists on remote
        exists, err := w.branchExistsOnRemote(ctx, w.branch)
        if err != nil {
            return err
        }
        if !exists {
            return fmt.Errorf("branch not found on remote after push")
        }
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to push branch",
            Message:  fmt.Sprintf("Claude could not push after %d attempts", result.Attempts),
            Context: map[string]string{
                "branch": w.branch,
                "error":  result.LastErr.Error(),
            },
        })
        return result.LastErr
    }

    if w.events != nil {
        evt := events.NewEvent(events.BranchPushed, w.unit.ID).
            WithPayload(map[string]interface{}{"branch": w.branch})
        w.events.Emit(evt)
    }

    return nil
}

// createPRViaClaudeCode invokes Claude to create the PR
func (w *Worker) createPRViaClaudeCode(ctx context.Context) (string, error) {
    prompt := BuildPRPrompt(w.branch, w.config.TargetBranch, w.unit.Title)

    var prURL string

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        output, err := w.invokeClaudeWithOutput(ctx, prompt)
        if err != nil {
            return err
        }

        // Extract PR URL from output
        url := extractPRURL(output)
        if url == "" {
            return fmt.Errorf("could not find PR URL in claude output")
        }

        prURL = url
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to create PR",
            Message:  fmt.Sprintf("Claude could not create PR after %d attempts", result.Attempts),
            Context: map[string]string{
                "branch": w.branch,
                "target": w.config.TargetBranch,
                "error":  result.LastErr.Error(),
            },
        })
        return "", result.LastErr
    }

    return prURL, nil
}

// prURLPattern matches GitHub PR URLs
var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

// extractPRURL extracts a GitHub PR URL from Claude's output
func extractPRURL(output string) string {
    match := prURLPattern.FindString(output)
    return match
}
```

### Git Verification Helpers

```go
// internal/worker/git_delegate.go (continued)

import (
    "os/exec"
    "strings"
)

// getHeadRef returns the current HEAD commit SHA
func (w *Worker) getHeadRef(ctx context.Context) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

// hasNewCommit checks if HEAD has moved since the given ref
func (w *Worker) hasNewCommit(ctx context.Context, sinceRef string) (bool, error) {
    currentHead, err := w.getHeadRef(ctx)
    if err != nil {
        return false, err
    }
    return currentHead != sinceRef, nil
}

// branchExistsOnRemote checks if a branch exists on the remote
func (w *Worker) branchExistsOnRemote(ctx context.Context, branch string) (bool, error) {
    cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return false, err
    }
    return strings.TrimSpace(string(out)) != "", nil
}

// getChangedFiles returns list of modified/added/deleted files
func (w *Worker) getChangedFiles(ctx context.Context) ([]string, error) {
    cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
    cmd.Dir = w.worktreePath
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var files []string
    lines := strings.Split(string(out), "\n")
    for _, line := range lines {
        if len(line) >= 3 {
            // Format: "XY filename" where XY is status
            files = append(files, strings.TrimSpace(line[3:]))
        }
    }
    return files, nil
}

// invokeClaude invokes Claude CLI with the given prompt (no output capture)
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
    taskPrompt := TaskPrompt{Content: prompt}
    return w.invokeClaudeForTask(ctx, taskPrompt)
}

// invokeClaudeWithOutput invokes Claude CLI and captures stdout
func (w *Worker) invokeClaudeWithOutput(ctx context.Context, prompt string) (string, error) {
    cmd := exec.CommandContext(ctx, "claude",
        "--dangerously-skip-permissions",
        "-p", prompt,
    )
    cmd.Dir = w.worktreePath

    out, err := cmd.Output()
    if err != nil {
        return "", err
    }

    return string(out), nil
}
```

### Configuration

```yaml
# Configuration for git delegation retry behavior
retry:
  max_attempts: 3
  initial_backoff: 1s
  max_backoff: 30s
  backoff_multiply: 2.0
```

```go
// internal/worker/worker.go (additions to WorkerConfig)

type WorkerConfig struct {
    // ... existing fields ...

    // GitRetry configures retry behavior for git operations
    GitRetry RetryConfig
}

// DefaultConfig updated to include GitRetry
func DefaultConfig() WorkerConfig {
    return WorkerConfig{
        // ... existing defaults ...
        GitRetry: DefaultRetryConfig,
    }
}
```

## Implementation Notes

### Verification Over Trust

The orchestrator never trusts Claude's assertion that an operation succeeded. Every operation is verified independently:

| Operation | Verification Method |
|-----------|---------------------|
| Commit | Compare HEAD ref before/after |
| Push | `git ls-remote --heads origin <branch>` |
| PR creation | Regex extract URL from output |

### Error Classification

All errors are treated as potentially transient and retried:

```go
// We do NOT distinguish between error types
// Network errors, rate limits, Claude failures - all get retried
// Only after max retries do we escalate
```

This simplifies the logic and handles the reality that most failures during automation are transient.

### Output Capture for PR URL

The PR creation operation requires capturing Claude's output to extract the PR URL. This differs from other operations that only need success/failure verification.

```go
// For commit/push: fire-and-forget invocation, verify separately
err := w.invokeClaude(ctx, prompt)

// For PR creation: capture output for URL extraction
output, err := w.invokeClaudeWithOutput(ctx, prompt)
url := extractPRURL(output)
```

### Escalation Integration

When retries are exhausted, the system escalates to the user rather than falling back to direct git operations:

```go
// internal/escalate/escalate.go

type Escalation struct {
    Severity Severity
    Unit     string
    Title    string
    Message  string
    Context  map[string]string
}

type Severity int

const (
    SeverityInfo Severity = iota
    SeverityWarning
    SeverityBlocking  // Requires user intervention
)
```

This maintains the principle that Claude handles all creative work - if Claude cannot complete an operation, a human should review rather than the orchestrator attempting a potentially incorrect action.

### Event Emission

Git delegation operations emit events for observability:

| Event | When |
|-------|------|
| GitCommitStarted | Before invoking Claude for commit |
| GitCommitComplete | After successful commit verification |
| GitCommitFailed | After retry exhaustion for commit |
| BranchPushed | After successful push verification |
| BranchPushFailed | After retry exhaustion for push |
| PRCreationStarted | Before invoking Claude for PR |
| PRCreated | After successful PR URL extraction |
| PRCreationFailed | After retry exhaustion for PR |

## Testing Strategy

### Unit Tests

```go
// internal/worker/prompt_git_test.go

func TestBuildCommitPrompt_IncludesTaskTitle(t *testing.T) {
    prompt := BuildCommitPrompt("Implement user authentication", []string{"auth.go", "auth_test.go"})

    if !strings.Contains(prompt, "Implement user authentication") {
        t.Error("prompt should contain task title")
    }
    if !strings.Contains(prompt, "git add -A") {
        t.Error("prompt should instruct to stage changes")
    }
    if !strings.Contains(prompt, "conventional commit") {
        t.Error("prompt should mention conventional commit format")
    }
}

func TestBuildCommitPrompt_FormatsFileList(t *testing.T) {
    files := []string{"src/main.go", "src/utils.go", "README.md"}
    prompt := BuildCommitPrompt("Test task", files)

    for _, f := range files {
        if !strings.Contains(prompt, f) {
            t.Errorf("prompt should contain file %s", f)
        }
    }
}

func TestBuildCommitPrompt_EmptyFileList(t *testing.T) {
    prompt := BuildCommitPrompt("Test task", nil)

    if !strings.Contains(prompt, "(no files listed)") {
        t.Error("prompt should handle empty file list")
    }
}

func TestBuildPushPrompt_IncludesBranch(t *testing.T) {
    prompt := BuildPushPrompt("ralph/my-feature-abc123")

    if !strings.Contains(prompt, "ralph/my-feature-abc123") {
        t.Error("prompt should contain branch name")
    }
    if !strings.Contains(prompt, "git push -u origin") {
        t.Error("prompt should instruct to push with upstream tracking")
    }
}

func TestBuildPRPrompt_IncludesAllDetails(t *testing.T) {
    prompt := BuildPRPrompt("ralph/feature-xyz", "main", "User Authentication")

    if !strings.Contains(prompt, "ralph/feature-xyz") {
        t.Error("prompt should contain source branch")
    }
    if !strings.Contains(prompt, "main") {
        t.Error("prompt should contain target branch")
    }
    if !strings.Contains(prompt, "User Authentication") {
        t.Error("prompt should contain unit title")
    }
    if !strings.Contains(prompt, "gh pr create") {
        t.Error("prompt should instruct to use gh CLI")
    }
}
```

```go
// internal/worker/retry_test.go

func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
    callCount := 0
    result := RetryWithBackoff(context.Background(), DefaultRetryConfig, func(ctx context.Context) error {
        callCount++
        return nil
    })

    if !result.Success {
        t.Error("expected success")
    }
    if result.Attempts != 1 {
        t.Errorf("expected 1 attempt, got %d", result.Attempts)
    }
    if callCount != 1 {
        t.Errorf("expected 1 call, got %d", callCount)
    }
}

func TestRetryWithBackoff_SuccessOnSecondAttempt(t *testing.T) {
    callCount := 0
    cfg := RetryConfig{
        MaxAttempts:     3,
        InitialBackoff:  1 * time.Millisecond,
        MaxBackoff:      10 * time.Millisecond,
        BackoffMultiply: 2.0,
    }

    result := RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        callCount++
        if callCount < 2 {
            return fmt.Errorf("transient error")
        }
        return nil
    })

    if !result.Success {
        t.Error("expected success")
    }
    if result.Attempts != 2 {
        t.Errorf("expected 2 attempts, got %d", result.Attempts)
    }
}

func TestRetryWithBackoff_ExhaustsRetries(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:     3,
        InitialBackoff:  1 * time.Millisecond,
        MaxBackoff:      10 * time.Millisecond,
        BackoffMultiply: 2.0,
    }
    callCount := 0

    result := RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        callCount++
        return fmt.Errorf("persistent error")
    })

    if result.Success {
        t.Error("expected failure")
    }
    if result.Attempts != 3 {
        t.Errorf("expected 3 attempts, got %d", result.Attempts)
    }
    if callCount != 3 {
        t.Errorf("expected 3 calls, got %d", callCount)
    }
    if result.LastErr == nil {
        t.Error("expected LastErr to be set")
    }
}

func TestRetryWithBackoff_RespectsContext(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cfg := RetryConfig{
        MaxAttempts:     5,
        InitialBackoff:  100 * time.Millisecond,
        MaxBackoff:      1 * time.Second,
        BackoffMultiply: 2.0,
    }

    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()

    result := RetryWithBackoff(ctx, cfg, func(ctx context.Context) error {
        return fmt.Errorf("error")
    })

    if result.Success {
        t.Error("expected failure due to context cancellation")
    }
    if result.LastErr != context.Canceled {
        t.Errorf("expected context.Canceled error, got %v", result.LastErr)
    }
}

func TestRetryWithBackoff_BackoffIncreases(t *testing.T) {
    cfg := RetryConfig{
        MaxAttempts:     4,
        InitialBackoff:  10 * time.Millisecond,
        MaxBackoff:      1 * time.Second,
        BackoffMultiply: 2.0,
    }

    var timestamps []time.Time
    RetryWithBackoff(context.Background(), cfg, func(ctx context.Context) error {
        timestamps = append(timestamps, time.Now())
        return fmt.Errorf("error")
    })

    // Check that delays increase
    for i := 1; i < len(timestamps)-1; i++ {
        delay1 := timestamps[i].Sub(timestamps[i-1])
        delay2 := timestamps[i+1].Sub(timestamps[i])
        if delay2 < delay1 {
            t.Errorf("backoff should increase: delay %d (%v) < delay %d (%v)",
                i+1, delay2, i, delay1)
        }
    }
}
```

```go
// internal/worker/git_delegate_test.go

func TestExtractPRURL_ValidURL(t *testing.T) {
    output := `Creating pull request...
https://github.com/anthropics/choo/pull/42
Done!`

    url := extractPRURL(output)
    if url != "https://github.com/anthropics/choo/pull/42" {
        t.Errorf("expected PR URL, got %q", url)
    }
}

func TestExtractPRURL_NoURL(t *testing.T) {
    output := "Error: could not create PR"

    url := extractPRURL(output)
    if url != "" {
        t.Errorf("expected empty string, got %q", url)
    }
}

func TestExtractPRURL_MultipleURLs(t *testing.T) {
    // Should return the first match
    output := `https://github.com/anthropics/choo/pull/41
https://github.com/anthropics/choo/pull/42`

    url := extractPRURL(output)
    if url != "https://github.com/anthropics/choo/pull/41" {
        t.Errorf("expected first PR URL, got %q", url)
    }
}

func TestFormatFileList_Empty(t *testing.T) {
    result := formatFileList(nil)
    if result != "(no files listed)" {
        t.Errorf("expected empty message, got %q", result)
    }
}

func TestFormatFileList_MultipleFiles(t *testing.T) {
    files := []string{"a.go", "b.go", "c.go"}
    result := formatFileList(files)

    for _, f := range files {
        if !strings.Contains(result, "- "+f) {
            t.Errorf("result should contain '- %s'", f)
        }
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Commit via Claude | Mock Claude that runs `git add -A && git commit -m "test"` |
| Push via Claude | Mock Claude that runs `git push`, verify remote branch |
| PR via Claude | Mock Claude that outputs PR URL, verify extraction |
| Retry on transient failure | Mock Claude that fails twice then succeeds |
| Escalation on persistent failure | Mock Claude that always fails, verify escalation |

### Manual Testing

- [ ] Claude creates commit with meaningful message based on task context
- [ ] Claude pushes branch successfully and branch appears on remote
- [ ] Claude creates PR with contextual title and description
- [ ] PR URL is correctly extracted from Claude's output
- [ ] Retries occur with increasing backoff on transient failures
- [ ] Escalation triggers after max retries exhausted
- [ ] Events are emitted for all git delegation operations
- [ ] Context cancellation stops retry loop appropriately

## Design Decisions

### Why Delegate Git Operations to Claude?

Claude already handles the complex work (implementation, conflict resolution). Git operations are simpler but benefit from Claude's understanding of *why* changes were made:

1. **Commit messages**: Template-based messages like `feat(unit): complete task #N` provide no insight. Claude can write `feat(auth): add JWT validation with configurable expiry` because it knows what it implemented.

2. **PR descriptions**: Claude can summarize the implementation approach, key decisions, and what reviewers should focus on.

3. **Consistency**: All creative work goes through Claude; orchestrator only coordinates.

Alternative considered: Keep orchestrator executing git commands. Rejected because:
- Loses opportunity for rich, contextual messages
- Creates two models of execution (Claude for code, orchestrator for git)
- Adds Go code to maintain

### Why Retry All Errors?

The retry logic does not distinguish between error types. All errors are treated as potentially transient:

```go
// We don't check: if isTransientError(err) { retry }
// Instead: always retry, assume transient
```

Rationale:
1. Most automation failures ARE transient (network, rate limits, timeouts)
2. Classifying errors requires deep knowledge of Claude's failure modes
3. If an error is truly permanent, we'll hit max retries quickly and escalate
4. Simpler code with fewer edge cases

### Why Escalate Instead of Fallback?

When retries are exhausted, the system escalates to the user rather than falling back to direct git operations:

```go
// NOT: if claude fails, do it ourselves
// INSTEAD: if claude fails, ask the human
```

Rationale:
1. Maintains the principle that Claude handles creative work
2. If Claude cannot commit, there may be an issue worth human review
3. Direct git operations might make incorrect assumptions (e.g., wrong commit message)
4. Better to pause and get human input than proceed incorrectly

### Why Verify After Every Operation?

Each operation is verified independently rather than trusting Claude's output:

```go
// After commit prompt: check if HEAD moved
// After push prompt: check if branch exists on remote
```

Rationale:
1. Claude might output "Done!" without actually completing the operation
2. Network issues could interrupt git commands
3. Verification is cheap (simple git commands)
4. Defense in depth for automation reliability

## Future Enhancements

1. **Intelligent commit grouping** - Allow Claude to combine related changes into single commits
2. **PR template support** - Load PR description template from repo and have Claude fill it in
3. **Commit signing** - Support for GPG-signed commits via Claude
4. **Interactive rebase delegation** - Have Claude handle complex rebase scenarios
5. **Branch cleanup delegation** - Let Claude decide when to delete merged branches

## References

- [WORKER Spec](/Users/bennett/conductor/workspaces/choo/las-vegas/specs/completed/WORKER.md) - Worker execution model
- [GIT Spec](/Users/bennett/conductor/workspaces/choo/las-vegas/specs/completed/GIT.md) - Existing git operations
- [EVENTS Spec](/Users/bennett/conductor/workspaces/choo/las-vegas/specs/completed/EVENTS.md) - Event bus for observability
