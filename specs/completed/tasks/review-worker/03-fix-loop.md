---
task: 3
status: complete
backpressure: "go test ./internal/worker/... -run TestReviewFixLoop"
depends_on: [2]
---

# Review Fix Loop

**Parent spec**: `specs/REVIEW-WORKER.md`
**Task**: #3 of 4 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the fix iteration loop (`runReviewFixLoop`) and provider invocation (`invokeProviderForFix`) that attempt to address review issues. Also implement the fix prompt builder that formats issues for the provider.

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `ReviewIssue` type
- REVIEW-CONFIG - provides `CodeReviewConfig` with `MaxFixIterations`, `Verbose`

### Task Dependencies (within this unit)
- Task #2 (Review Orchestration) - calls this function, provides context

### Package Dependencies
- `github.com/RevCBH/choo/internal/provider` - for ReviewIssue type and Provider interface
- `github.com/RevCBH/choo/internal/events` - for event emission
- `github.com/RevCBH/choo/internal/config` - for CodeReviewConfig
- `io` - for io.Discard
- `os` - for os.Stderr
- `os/exec` - for test setup (git init)
- `path/filepath` - for test file paths
- `strings` - for string building
- `fmt` - for formatting

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── review.go        # MODIFY: Add runReviewFixLoop and invokeProviderForFix
└── prompt_review.go # CREATE: Fix prompt builder
```

### Functions to Implement

#### review.go additions

```go
// runReviewFixLoop attempts to fix review issues up to MaxFixIterations times.
// Returns true if all issues were resolved (a fix was committed).
func (w *Worker) runReviewFixLoop(ctx context.Context, issues []provider.ReviewIssue) bool {
    maxIterations := 1
    if w.reviewConfig != nil {
        maxIterations = w.reviewConfig.MaxFixIterations
    }

    for i := 0; i < maxIterations; i++ {
        // Emit fix attempt event
        if w.events != nil {
            evt := events.NewEvent(events.CodeReviewFixAttempt, w.unit.ID).
                WithPayload(map[string]any{
                    "iteration":      i + 1,
                    "max_iterations": maxIterations,
                })
            w.events.Emit(evt)
        }

        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Fix attempt %d/%d\n", i+1, maxIterations)
        }

        // Build fix prompt and invoke provider
        fixPrompt := BuildReviewFixPrompt(issues)
        if err := w.invokeProviderForFix(ctx, fixPrompt); err != nil {
            fmt.Fprintf(os.Stderr, "Fix attempt failed: %v\n", err)
            w.cleanupWorktree(ctx) // Reset any partial changes
            continue
        }

        // Commit any fix changes
        committed, err := w.commitReviewFixes(ctx)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to commit review fixes: %v\n", err)
            w.cleanupWorktree(ctx)
            continue
        }

        if committed {
            if w.events != nil {
                evt := events.NewEvent(events.CodeReviewFixApplied, w.unit.ID).
                    WithPayload(map[string]any{"iteration": i + 1})
                w.events.Emit(evt)
            }
            return true // Success
        }

        // No changes made - provider didn't fix anything
        if w.reviewConfig != nil && w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "No changes made by fix attempt %d\n", i+1)
        }
    }

    // Cleanup any uncommitted changes left by fix attempts
    w.cleanupWorktree(ctx)
    return false
}

// invokeProviderForFix asks the task provider to address review issues.
// Uses the same provider that executed the unit tasks.
func (w *Worker) invokeProviderForFix(ctx context.Context, fixPrompt string) error {
    if w.provider == nil {
        return fmt.Errorf("no provider configured")
    }

    // Invoke provider with fix prompt
    // stdout discarded (we only care about file changes)
    // stderr passed through for visibility
    return w.provider.Invoke(ctx, fixPrompt, w.worktreePath, io.Discard, os.Stderr)
}
```

#### prompt_review.go (new file)

```go
// internal/worker/prompt_review.go

package worker

import (
    "fmt"
    "strings"

    "github.com/RevCBH/choo/internal/provider"
)

// BuildReviewFixPrompt creates a prompt for the task provider to fix review issues.
func BuildReviewFixPrompt(issues []provider.ReviewIssue) string {
    var sb strings.Builder

    sb.WriteString("Code review found the following issues that need to be addressed:\n\n")

    for i, issue := range issues {
        sb.WriteString(fmt.Sprintf("## Issue %d: %s\n", i+1, issue.Severity))
        if issue.File != "" {
            sb.WriteString(fmt.Sprintf("**File**: %s", issue.File))
            if issue.Line > 0 {
                sb.WriteString(fmt.Sprintf(":%d", issue.Line))
            }
            sb.WriteString("\n")
        }
        sb.WriteString(fmt.Sprintf("**Problem**: %s\n", issue.Message))
        if issue.Suggestion != "" {
            sb.WriteString(fmt.Sprintf("**Suggestion**: %s\n", issue.Suggestion))
        }
        sb.WriteString("\n")
    }

    sb.WriteString("Please address these issues. Focus on the most critical ones first.\n")
    sb.WriteString("Make minimal changes needed to resolve the issues.\n")

    return sb.String()
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run TestReviewFixLoop
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestReviewFixLoop_Success` | Returns true when fix commits successfully |
| `TestReviewFixLoop_ProviderError` | Continues to next iteration on provider error |
| `TestReviewFixLoop_CommitError` | Cleans up worktree on commit error |
| `TestReviewFixLoop_NoChanges` | Returns false when no changes made |
| `TestReviewFixLoop_MaxIterations` | Stops after MaxFixIterations attempts |
| `TestReviewFixLoop_CleanupOnExit` | Calls cleanupWorktree on final exit |
| `TestInvokeProviderForFix_NilProvider` | Returns error when provider is nil |
| `TestBuildReviewFixPrompt_SingleIssue` | Formats single issue correctly |
| `TestBuildReviewFixPrompt_MultipleIssues` | Formats multiple issues with numbering |
| `TestBuildReviewFixPrompt_NoFileLocation` | Handles issues without file/line |
| `TestBuildReviewFixPrompt_WithSuggestion` | Includes suggestion when present |

### Test Fixtures

```go
// collectEvents subscribes to the event bus and collects events for testing.
// Returns a pointer to a slice that will be populated with events as they occur.
func collectEvents(bus *events.Bus) *[]events.Event {
    collected := &[]events.Event{}
    bus.Subscribe(func(e events.Event) {
        *collected = append(*collected, e)
    })
    return collected
}

func TestReviewFixLoop_Success(t *testing.T) {
    eventBus := events.NewBus()
    collected := collectEvents(eventBus)

    prov := &mockProvider{} // Succeeds
    worktreePath := t.TempDir()

    // Create a file so commitReviewFixes has something to commit
    testFile := filepath.Join(worktreePath, "test.go")
    os.WriteFile(testFile, []byte("package test"), 0644)

    w := &Worker{
        unit:         &discovery.Unit{ID: "test-unit"},
        provider:     prov,
        events:       eventBus,
        worktreePath: worktreePath,
        gitRunner:    git.DefaultRunner(),
        reviewConfig: &config.CodeReviewConfig{
            Enabled:          true,
            MaxFixIterations: 1,
            Verbose:          true,
        },
    }

    // Initialize git repo for test
    exec.Command("git", "init", worktreePath).Run()
    exec.Command("git", "-C", worktreePath, "add", ".").Run()

    issues := []provider.ReviewIssue{
        {File: "main.go", Line: 10, Severity: "error", Message: "unused variable"},
    }

    ctx := context.Background()
    result := w.runReviewFixLoop(ctx, issues)

    assert.True(t, result, "expected fix loop to succeed")
    assert.True(t, prov.invoked, "expected provider to be invoked")

    // Should emit fix_attempt and fix_applied
    var hasAttempt, hasApplied bool
    for _, e := range *collected {
        if e.Type == events.CodeReviewFixAttempt {
            hasAttempt = true
        }
        if e.Type == events.CodeReviewFixApplied {
            hasApplied = true
        }
    }
    assert.True(t, hasAttempt, "expected CodeReviewFixAttempt event")
    assert.True(t, hasApplied, "expected CodeReviewFixApplied event")
}

func TestReviewFixLoop_ProviderError(t *testing.T) {
    prov := &mockProvider{invokeError: errors.New("provider failed")}
    worktreePath := t.TempDir()

    // Initialize git repo for cleanup operations
    exec.Command("git", "init", worktreePath).Run()

    w := &Worker{
        unit:         &discovery.Unit{ID: "test-unit"},
        provider:     prov,
        events:       events.NewBus(),
        worktreePath: worktreePath,
        gitRunner:    git.DefaultRunner(),
        reviewConfig: &config.CodeReviewConfig{
            Enabled:          true,
            MaxFixIterations: 2,
            Verbose:          true,
        },
    }

    issues := []provider.ReviewIssue{
        {Severity: "error", Message: "issue"},
    }

    ctx := context.Background()
    result := w.runReviewFixLoop(ctx, issues)

    assert.False(t, result, "expected fix loop to fail")
    assert.Equal(t, 2, prov.invokeCount, "expected 2 invoke attempts")
}

func TestBuildReviewFixPrompt_SingleIssue(t *testing.T) {
    issues := []provider.ReviewIssue{
        {
            File:       "main.go",
            Line:       42,
            Severity:   "error",
            Message:    "undefined variable: foo",
            Suggestion: "Did you mean 'f00'?",
        },
    }

    prompt := BuildReviewFixPrompt(issues)

    assert.Contains(t, prompt, "## Issue 1: error")
    assert.Contains(t, prompt, "**File**: main.go:42")
    assert.Contains(t, prompt, "**Problem**: undefined variable: foo")
    assert.Contains(t, prompt, "**Suggestion**: Did you mean 'f00'?")
}

func TestBuildReviewFixPrompt_NoFileLocation(t *testing.T) {
    issues := []provider.ReviewIssue{
        {
            Severity: "warning",
            Message:  "general code smell",
            // No File or Line
        },
    }

    prompt := BuildReviewFixPrompt(issues)

    assert.NotContains(t, prompt, "**File**:")
    assert.Contains(t, prompt, "**Problem**: general code smell")
}

// mockProvider for testing
type mockProvider struct {
    invokeError error
    invoked     bool
    invokeCount int
}

func (m *mockProvider) Invoke(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error {
    m.invoked = true
    m.invokeCount++
    return m.invokeError
}

func (m *mockProvider) Name() provider.ProviderType {
    return "mock"
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- **Provider Reuse**: The fix loop uses the same provider that executed unit tasks. This ensures consistency and leverages the provider's existing context.
- **Output Handling**: stdout is discarded (fixes are detected via git status). stderr is passed through for visibility into what the provider is doing.
- **Early Exit**: The loop exits early on first successful commit. We don't re-run review to verify fixes - that would add complexity and latency.
- **Cleanup Between Iterations**: Each failed attempt triggers cleanup to ensure a clean state for the next iteration.
- **No Re-review**: We don't re-run the reviewer after fixes. The fix is best-effort, and the merge proceeds regardless.

### Provider.Invoke Signature

The provider interface expects:
```go
Invoke(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error
```

We pass `io.Discard` for stdout since we don't need the output - we detect success via git operations.

## NOT In Scope

- Commit and cleanup git operations (Task #4)
- Re-running review after fixes (out of scope per spec)
- Severity-based filtering of issues (future enhancement)
