# REVIEW-WORKER — Worker Integration for Advisory Code Review

## Overview

The REVIEW-WORKER component integrates advisory code review into the run workflow. Review runs at two points:

1. **Per-unit review**: After all tasks in a unit complete, the worker invokes the configured reviewer to analyze changes against the local feature branch (which may contain prior unit merges that haven't been pushed yet).

2. **Pre-merge review**: After all units merge to the feature branch, before the final rebase/merge to the target branch, a comprehensive review runs on the accumulated changes.

If issues are found, the system attempts fixes up to `MaxFixIterations` times (default: 1) using the task provider before proceeding. The review is strictly advisory and non-blocking—the merge always proceeds regardless of whether the review runs successfully, finds issues, or fails entirely. All review outcomes emit events for observability, but no error from the review path ever propagates to block the merge operation.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Run Workflow with Review                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  PER-UNIT PHASE (for each unit):                                        │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐             │
│  │   Execute    │────▶│   Run Code   │────▶│  Merge to    │             │
│  │    Tasks     │     │  Review (1)  │     │  Feature Br  │             │
│  └──────────────┘     └──────────────┘     └──────────────┘             │
│                                                                          │
│  PRE-MERGE PHASE (after all units complete):                            │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐             │
│  │  All Units   │────▶│   Run Code   │────▶│  Rebase/Merge│             │
│  │   Merged     │     │  Review (2)  │     │  to Target   │             │
│  └──────────────┘     └──────────────┘     └──────────────┘             │
│                                                                          │
│  REVIEW FLOW (both phases):                                              │
│                    ┌─────────────────────┐                               │
│                    │   Review Outcome?   │                               │
│                    └─────────┬───────────┘                               │
│                              │                                           │
│         ┌────────────────────┼────────────────────┐                      │
│         ▼                    ▼                    ▼                      │
│   ┌───────────┐       ┌───────────┐       ┌───────────┐                 │
│   │  Passed   │       │  Issues   │       │  Failed   │                 │
│   │  (done)   │       │  Found    │       │  (log it) │                 │
│   └───────────┘       └─────┬─────┘       └───────────┘                 │
│                             │                                            │
│                    ┌────────▼────────┐                                   │
│                    │  Fix Loop       │ ◀── up to MaxFixIterations       │
│                    │  (invoke+commit)│                                   │
│                    └────────┬────────┘                                   │
│                             │                                            │
│                    ┌────────▼────────┐                                   │
│                    │  Clean Worktree │ ◀── reset uncommitted changes    │
│                    └─────────────────┘                                   │
│                                                                          │
│  NOTE: All paths proceed to merge. Review never blocks.                  │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. System MUST run code review at two points: after unit tasks complete, and after all units merge to feature branch (FR-1)
2. System MUST NOT block merge due to review failures or issues (FR-2)
3. System MUST invoke the reviewer with worktree path and base ref (local feature branch)
4. System MUST proceed to merge even when reviewer returns an error
5. System MUST proceed to merge even when issues are found
6. System MUST feed discovered issues to the implementing provider for fix attempts up to MaxFixIterations (FR-6)
7. System MUST commit fix attempts if changes are made (FR-7)
8. System MUST clean up (reset) worktree if fix leaves uncommitted changes (FR-8)
9. System MUST skip review gracefully when reviewer is nil (disabled)
10. System MUST emit events for all review lifecycle states (FR-10)
11. System MUST use the local feature branch (not remote) as the base reference for diffs (FR-11)

### Performance Requirements

| Metric | Target |
|--------|--------|
| Review invocation overhead | <100ms before reviewer.Review() call |
| Fix prompt construction | <10ms for up to 100 issues |
| Event emission latency | <5ms per event |

### Constraints

- Depends on REVIEWER-INTERFACE for the Reviewer interface and ReviewResult types
- Depends on the existing Provider interface for fix invocations
- Must not introduce new error types that propagate outside the review path
- All stderr output must be informational only (not error conditions)

## Design

### Module Structure

```
internal/worker/
├── worker.go        # Add reviewer field to Worker struct
├── review.go        # Review orchestration logic (new file)
└── prompt_review.go # Fix prompt builder (new file)
```

### Core Types

```go
// internal/worker/worker.go (additions)

// Worker executes tasks within a git worktree.
// The reviewer field enables optional code review after task completion.
type Worker struct {
    // ... existing fields ...

    provider     Provider           // For task execution and fix attempts
    reviewer     Reviewer           // For code review (may be nil if disabled)
    reviewConfig *CodeReviewConfig  // Review configuration (MaxFixIterations, Verbose, etc.)
    git          GitOperations      // Git operations interface
    eventBus     EventBus           // For emitting review events
}
```

```go
// internal/events/types.go (additions)

// EventType identifies the type of event emitted during orchestration.
type EventType string

const (
    // CodeReviewStarted is emitted when code review begins
    CodeReviewStarted EventType = "codereview.started"

    // CodeReviewPassed is emitted when review finds no issues
    CodeReviewPassed EventType = "codereview.passed"

    // CodeReviewIssuesFound is emitted when review discovers issues
    CodeReviewIssuesFound EventType = "codereview.issues_found"

    // CodeReviewFixApplied is emitted when fix changes are committed
    CodeReviewFixApplied EventType = "codereview.fix_applied"

    // CodeReviewFailed is emitted when review fails to run
    CodeReviewFailed EventType = "codereview.failed"
)
```

### API Surface

```go
// internal/worker/review.go

// runCodeReview performs advisory code review after task completion.
// This function NEVER returns an error that blocks the merge.
// All review failures are logged but do not prevent merge.
func (w *Worker) runCodeReview(ctx context.Context)

// runReviewFixLoop attempts to fix review issues up to MaxFixIterations times.
func (w *Worker) runReviewFixLoop(ctx context.Context, issues []ReviewIssue) bool

// invokeProviderForFix asks the task provider to address review issues.
func (w *Worker) invokeProviderForFix(ctx context.Context, fixPrompt string) error

// commitReviewFixes commits any changes made during the fix attempt.
// Returns true if changes were committed.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error)

// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
func (w *Worker) cleanupWorktree(ctx context.Context) error

// getBaseRef returns the base branch reference for diff comparison.
// This is the local feature branch containing prior unit merges.
func (w *Worker) getBaseRef() string
```

```go
// internal/worker/prompt_review.go

// BuildReviewFixPrompt creates a prompt for the task provider to fix issues.
func BuildReviewFixPrompt(issues []provider.ReviewIssue) string
```

### Review Orchestration

```go
// internal/worker/review.go

package worker

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/your-org/choo/internal/config"
    "github.com/your-org/choo/internal/events"
    "github.com/your-org/choo/internal/provider"
)

// runCodeReview performs advisory code review after task completion.
// This function NEVER returns an error that blocks the merge.
// All review failures are logged but do not prevent merge.
func (w *Worker) runCodeReview(ctx context.Context) {
    if w.reviewer == nil {
        return // Review disabled
    }

    w.eventBus.Emit(events.Event{
        Type: events.CodeReviewStarted,
        Data: map[string]any{"unit": w.unit.Name},
    })

    // Determine base branch for comparison - use local feature branch
    // which may contain prior unit merges that haven't been pushed
    baseRef := w.getBaseRef()

    // Run the review - errors are logged but don't block
    result, err := w.reviewer.Review(ctx, w.worktreePath, baseRef)
    if err != nil {
        if w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Code review failed to run: %v\n", err)
        }
        w.eventBus.Emit(events.Event{
            Type: events.CodeReviewFailed,
            Data: map[string]any{"error": err.Error()},
        })
        return // Proceed to merge anyway
    }

    // No issues found - success
    if result.Passed || len(result.Issues) == 0 {
        if w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Code review passed: %s\n", result.Summary)
        }
        w.eventBus.Emit(events.Event{
            Type: events.CodeReviewPassed,
            Data: map[string]any{"summary": result.Summary},
        })
        return
    }

    // Issues found - log them (always, since this is actionable)
    fmt.Fprintf(os.Stderr, "Code review found %d issues\n", len(result.Issues))
    w.eventBus.Emit(events.Event{
        Type: events.CodeReviewIssuesFound,
        Data: map[string]any{
            "count":  len(result.Issues),
            "issues": result.Issues,
        },
    })

    // Attempt fix loop if configured
    if w.reviewConfig.MaxFixIterations > 0 {
        w.runReviewFixLoop(ctx, result.Issues)
    }

    // Merge proceeds regardless of fix outcome
}

// runReviewFixLoop attempts to fix review issues up to MaxFixIterations times.
// Returns true if all issues were resolved.
func (w *Worker) runReviewFixLoop(ctx context.Context, issues []provider.ReviewIssue) bool {
    for i := 0; i < w.reviewConfig.MaxFixIterations; i++ {
        if w.reviewConfig.Verbose {
            fmt.Fprintf(os.Stderr, "Fix attempt %d/%d\n", i+1, w.reviewConfig.MaxFixIterations)
        }

        // Build fix prompt and invoke the implementing provider
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
            w.eventBus.Emit(events.Event{
                Type: events.CodeReviewFixApplied,
                Data: map[string]any{"iteration": i + 1},
            })
            return true
        }
    }

    // Cleanup any uncommitted changes left by fix attempts
    w.cleanupWorktree(ctx)
    return false
}

// invokeProviderForFix asks the task provider to address review issues.
func (w *Worker) invokeProviderForFix(ctx context.Context, fixPrompt string) error {
    // Use the same provider that executed the unit tasks
    return w.provider.Invoke(ctx, fixPrompt, w.worktreePath, io.Discard, os.Stderr)
}

// commitReviewFixes commits any changes made during the fix attempt.
// Returns (true, nil) if changes were committed, (false, nil) if no changes.
func (w *Worker) commitReviewFixes(ctx context.Context) (bool, error) {
    // Check for staged/unstaged changes
    hasChanges, err := w.git.HasUncommittedChanges(ctx, w.worktreePath)
    if err != nil {
        return false, fmt.Errorf("checking for changes: %w", err)
    }
    if !hasChanges {
        return false, nil // No changes to commit
    }

    // Stage all changes
    if err := w.git.StageAll(ctx, w.worktreePath); err != nil {
        return false, fmt.Errorf("staging changes: %w", err)
    }

    // Commit with standardized message
    commitMsg := "fix: address code review feedback"
    if err := w.git.Commit(ctx, w.worktreePath, commitMsg); err != nil {
        return false, fmt.Errorf("committing changes: %w", err)
    }

    return true, nil
}

// cleanupWorktree resets any uncommitted changes left by failed fix attempts.
// This ensures the worktree is clean before proceeding to merge.
func (w *Worker) cleanupWorktree(ctx context.Context) error {
    // Reset staged changes
    if err := w.git.Reset(ctx, w.worktreePath); err != nil {
        return fmt.Errorf("resetting staged changes: %w", err)
    }

    // Clean untracked files created by fix attempts
    if err := w.git.Clean(ctx, w.worktreePath); err != nil {
        return fmt.Errorf("cleaning untracked files: %w", err)
    }

    // Restore modified files
    if err := w.git.Checkout(ctx, w.worktreePath, "."); err != nil {
        return fmt.Errorf("restoring modified files: %w", err)
    }

    return nil
}

// getBaseRef returns the base branch reference for diff comparison.
// Uses the local feature branch which may contain prior unit merges
// that haven't been pushed yet.
func (w *Worker) getBaseRef() string {
    // Use the feature branch (local, not remote) as base for review
    // This ensures we review only the current unit's changes, not
    // all changes since the target branch diverged
    if w.featureBranch != "" {
        return w.featureBranch
    }
    // Fallback to unit's target branch if no feature branch context
    if w.unit.TargetBranch != "" {
        return w.unit.TargetBranch
    }
    return "main"
}
```

### Fix Prompt Builder

```go
// internal/worker/prompt_review.go

package worker

import (
    "fmt"
    "strings"

    "github.com/your-org/choo/internal/provider"
)

// BuildReviewFixPrompt creates a prompt for the task provider to fix issues.
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

### Integration Points

Review runs at two points in the workflow:

**1. Per-Unit Review (in Worker)**

The review is invoked from `mergeToFeatureBranch()` in the worker implementation:

```go
// internal/worker/worker.go - in mergeToFeatureBranch()
func (w *Worker) mergeToFeatureBranch(ctx context.Context) error {
    // ... after all unit tasks complete ...

    // Run advisory code review before merge
    w.runCodeReview(ctx) // Never blocks merge

    // ... proceed to merge ...
}
```

**2. Pre-Merge Review (in Orchestrator)**

After all units merge to the feature branch, before final rebase/merge:

```go
// internal/orchestrator/orchestrator.go - in finalizeFeatureBranch()
func (o *Orchestrator) finalizeFeatureBranch(ctx context.Context) error {
    // All units have been merged to feature branch

    // Run comprehensive review of all accumulated changes
    if o.reviewer != nil {
        o.runPreMergeReview(ctx) // Never blocks rebase/merge
    }

    // ... proceed to rebase/merge to target ...
}
```

The pre-merge review uses the same `runCodeReview` logic but with the target branch as the base ref instead of the feature branch.

## Implementation Notes

### Error Handling Philosophy

The review path intentionally suppresses all errors. This is not laziness but a deliberate design choice. The review is advisory: it provides value when it works, but must never block the primary workflow. Every error is logged to stderr for observability, and events are emitted for monitoring, but no error propagates to the caller.

```go
// CORRECT: Log and continue
if err != nil {
    fmt.Fprintf(os.Stderr, "Review failed: %v\n", err)
    w.emitEvent(ctx, CodeReviewFailed, map[string]any{"error": err.Error()})
    return // Merge proceeds
}

// INCORRECT: Return the error
if err != nil {
    return fmt.Errorf("review failed: %w", err) // DON'T DO THIS
}
```

### Configurable Fix Iterations

The system attempts fixes up to `MaxFixIterations` times (default: 1). This is configurable because:
- More iterations may help with complex issues requiring multiple passes
- Set to 0 for review-only mode (no fix attempts)
- Default of 1 is sufficient for most obvious issues
- Each iteration is independent (re-runs the full fix prompt)

The loop terminates early if:
- A fix is successfully committed
- Context is cancelled
- MaxFixIterations is reached

### Nil Reviewer Check

The reviewer field may be nil when code review is disabled. The first line of `runCodeReview` handles this:

```go
if w.reviewer == nil {
    return // Review disabled, nothing to do
}
```

This allows the worker to be constructed without a reviewer in configurations where review is not desired.

### Worktree Cleanup

After fix attempts, the worktree must be left in a clean state for merge operations. The `cleanupWorktree` function ensures:

1. **Reset**: Unstages any staged changes from failed fix attempts
2. **Clean**: Removes untracked files created by the provider
3. **Checkout**: Restores modified files to their committed state

This cleanup runs after the fix loop completes (whether successful or not) to ensure the worktree is ready for merge. Without cleanup, partial fix attempts could leave the worktree dirty, causing merge failures or incorrect commits.

### Event Data Shapes

Events carry structured data for downstream consumers:

| Event | Data Fields |
|-------|-------------|
| `codereview.started` | (none) |
| `codereview.passed` | `summary: string` |
| `codereview.issues_found` | `count: int`, `issues: []ReviewIssue` |
| `codereview.fix_applied` | (none) |
| `codereview.failed` | `error: string` |

### Commit Message Convention

Fix commits use a standardized message: `"fix: address code review feedback"`. This:
- Follows conventional commit format
- Clearly identifies the commit's purpose
- Keeps the commit message deterministic for testing

## Testing Strategy

### Unit Tests

```go
// internal/worker/review_test.go

func TestRunCodeReview_NilReviewer(t *testing.T) {
    w := &Worker{
        reviewer: nil, // Disabled
    }

    // Should return immediately without error
    ctx := context.Background()
    w.runCodeReview(ctx) // No panic, no error
}

func TestRunCodeReview_ReviewerError(t *testing.T) {
    events := &mockEventEmitter{}
    w := &Worker{
        reviewer: &mockReviewer{
            err: errors.New("reviewer unavailable"),
        },
        eventEmitter: events,
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Should emit failed event but not panic
    if len(events.emitted) != 2 {
        t.Errorf("expected 2 events (started, failed), got %d", len(events.emitted))
    }
    if events.emitted[1].Type != CodeReviewFailed {
        t.Errorf("expected CodeReviewFailed, got %v", events.emitted[1].Type)
    }
}

func TestRunCodeReview_Passed(t *testing.T) {
    events := &mockEventEmitter{}
    w := &Worker{
        reviewer: &mockReviewer{
            result: &provider.ReviewResult{
                Passed:  true,
                Summary: "All checks passed",
            },
        },
        eventEmitter: events,
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Should emit started and passed events
    if len(events.emitted) != 2 {
        t.Errorf("expected 2 events, got %d", len(events.emitted))
    }
    if events.emitted[1].Type != CodeReviewPassed {
        t.Errorf("expected CodeReviewPassed, got %v", events.emitted[1].Type)
    }
}

func TestRunCodeReview_IssuesFoundAndFixed(t *testing.T) {
    eventBus := &mockEventBus{}
    prov := &mockProvider{}
    git := &mockGit{hasUncommittedChanges: true}

    w := &Worker{
        reviewer: &mockReviewer{
            result: &provider.ReviewResult{
                Passed: false,
                Issues: []provider.ReviewIssue{
                    {File: "main.go", Line: 10, Message: "unused variable"},
                },
            },
        },
        provider:     prov,
        git:          git,
        eventBus:     eventBus,
        reviewConfig: &config.CodeReviewConfig{
            Enabled:          true,
            MaxFixIterations: 1,
            Verbose:          true,
        },
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Should emit started, issues_found, fix_applied
    eventTypes := make([]events.EventType, len(eventBus.emitted))
    for i, e := range eventBus.emitted {
        eventTypes[i] = e.Type
    }

    expected := []events.EventType{
        events.CodeReviewStarted,
        events.CodeReviewIssuesFound,
        events.CodeReviewFixApplied,
    }
    if !reflect.DeepEqual(eventTypes, expected) {
        t.Errorf("expected events %v, got %v", expected, eventTypes)
    }

    // Provider should have been invoked
    if !prov.invoked {
        t.Error("expected provider to be invoked for fix")
    }

    // Git should have committed
    if !git.committed {
        t.Error("expected git commit for fixes")
    }
}

func TestRunCodeReview_CleanupAfterFailedFix(t *testing.T) {
    eventBus := &mockEventBus{}
    prov := &mockProvider{invokeError: errors.New("fix failed")}
    git := &mockGit{}

    w := &Worker{
        reviewer: &mockReviewer{
            result: &provider.ReviewResult{
                Passed: false,
                Issues: []provider.ReviewIssue{
                    {File: "main.go", Line: 10, Message: "error"},
                },
            },
        },
        provider:     prov,
        git:          git,
        eventBus:     eventBus,
        reviewConfig: &config.CodeReviewConfig{
            Enabled:          true,
            MaxFixIterations: 1,
            Verbose:          true,
        },
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Worktree should have been cleaned up
    if !git.resetCalled {
        t.Error("expected git reset after failed fix")
    }
    if !git.cleanCalled {
        t.Error("expected git clean after failed fix")
    }
}

func TestRunCodeReview_MultipleIterations(t *testing.T) {
    eventBus := &mockEventBus{}
    // First attempt fails, second succeeds
    prov := &mockProvider{failFirstN: 1}
    git := &mockGit{hasUncommittedChanges: true}

    w := &Worker{
        reviewer: &mockReviewer{
            result: &provider.ReviewResult{
                Passed: false,
                Issues: []provider.ReviewIssue{
                    {File: "main.go", Line: 10, Message: "error"},
                },
            },
        },
        provider:     prov,
        git:          git,
        eventBus:     eventBus,
        reviewConfig: &config.CodeReviewConfig{
            Enabled:          true,
            MaxFixIterations: 3, // Allow multiple attempts
            Verbose:          true,
        },
    }

    ctx := context.Background()
    w.runCodeReview(ctx)

    // Should have attempted fix twice (first failed, second succeeded)
    if prov.invokeCount != 2 {
        t.Errorf("expected 2 invoke attempts, got %d", prov.invokeCount)
    }

    // Should have emitted fix_applied on success
    hasFixApplied := false
    for _, e := range eventBus.emitted {
        if e.Type == events.CodeReviewFixApplied {
            hasFixApplied = true
            break
        }
    }
    if !hasFixApplied {
        t.Error("expected CodeReviewFixApplied event")
    }
}
```

```go
// internal/worker/prompt_review_test.go

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

    // Verify structure
    if !strings.Contains(prompt, "## Issue 1: error") {
        t.Error("missing issue header")
    }
    if !strings.Contains(prompt, "**File**: main.go:42") {
        t.Error("missing file location")
    }
    if !strings.Contains(prompt, "**Problem**: undefined variable: foo") {
        t.Error("missing problem description")
    }
    if !strings.Contains(prompt, "**Suggestion**: Did you mean 'f00'?") {
        t.Error("missing suggestion")
    }
}

func TestBuildReviewFixPrompt_MultipleIssues(t *testing.T) {
    issues := []provider.ReviewIssue{
        {Severity: "error", Message: "first issue"},
        {Severity: "warning", Message: "second issue"},
        {Severity: "info", Message: "third issue"},
    }

    prompt := BuildReviewFixPrompt(issues)

    if !strings.Contains(prompt, "## Issue 1:") {
        t.Error("missing issue 1")
    }
    if !strings.Contains(prompt, "## Issue 2:") {
        t.Error("missing issue 2")
    }
    if !strings.Contains(prompt, "## Issue 3:") {
        t.Error("missing issue 3")
    }
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

    // Should not contain **File**: when file is empty
    if strings.Contains(prompt, "**File**:") {
        t.Error("should not include file line when file is empty")
    }
}
```

### Integration Tests

| Scenario | Setup | Verification |
|----------|-------|--------------|
| Review disabled | Worker with nil reviewer | No events emitted, merge proceeds |
| Review passes | Mock reviewer returns Passed=true | CodeReviewPassed event emitted |
| Review finds issues | Mock reviewer returns issues | Provider invoked with fix prompt |
| Fix succeeds | Mock provider succeeds, mock git has changes | Commit created, CodeReviewFixApplied emitted |
| Fix has no changes | Mock provider succeeds, mock git has no changes | No commit, cleanup runs |
| Reviewer fails | Mock reviewer returns error | CodeReviewFailed emitted, merge still proceeds |
| Provider fix fails | Mock provider returns error | Worktree cleaned, merge still proceeds |
| Multiple fix iterations | MaxFixIterations=3, first fails | Second attempt runs, cleanup between |
| Max iterations reached | All attempts fail | Cleanup runs, merge proceeds |
| Zero iterations | MaxFixIterations=0 | No fix attempts, issues logged only |
| Base ref is feature branch | Feature branch with prior merges | Diff against local feature branch, not remote |

### Manual Testing

- [ ] Run with review disabled (nil reviewer) - merge completes normally
- [ ] Run with review that passes - "passed" message in stderr (when verbose)
- [ ] Run with review that finds issues - fix prompt logged, provider invoked
- [ ] Run with MaxFixIterations=0 - issues reported but no fix attempts
- [ ] Run with MaxFixIterations=3 - multiple fix attempts if needed
- [ ] Verify worktree is clean after failed fix attempts
- [ ] Verify events appear in event stream for each scenario
- [ ] Interrupt during review - verify graceful handling and cleanup
- [ ] Review against local feature branch (not remote) - verify correct diff base
- [ ] Run verbose=false - verify quieter output (only issues printed)

## Design Decisions

### Why Non-Blocking?

The review is advisory because:
1. **Reliability**: Merge is the critical path. Review failures should not block deployments.
2. **Flexibility**: Teams can use review feedback without being gated by it.
3. **Graceful degradation**: If the review provider has issues, work continues.
4. **User trust**: Users trust that their completed tasks will merge. Breaking that trust creates friction.

Alternative considered: Making review a hard gate. Rejected because it creates a dependency on external services for the critical path.

### Why Configurable Fix Iterations?

The fix iteration count is configurable (default: 1) to support different workflows:

1. **Default (1)**: Simple and predictable for most use cases
2. **Multiple (2-3)**: Allows complex issues to be resolved in multiple passes
3. **Zero (0)**: Review-only mode for audit/informational purposes

The loop has safeguards:
- Terminates on successful commit
- Cleans up worktree between iterations
- Respects context cancellation

This flexibility accommodates teams with different review/fix workflows while keeping the default simple.

### Why stderr for Logging?

Review output goes to stderr because:
1. **stdout is for data**: stdout might be piped to other tools.
2. **Convention**: Informational/diagnostic output belongs on stderr.
3. **Consistency**: Other worker logging uses stderr.

### Why Events for All States?

Events enable:
1. **Observability**: External systems can track review outcomes.
2. **Metrics**: Aggregate review pass/fail rates over time.
3. **Debugging**: Correlate issues with review events.
4. **Automation**: Trigger alerts or actions based on review outcomes.

## Future Enhancements

1. **Review result caching**: Skip re-review if diff hasn't changed
2. **Severity filtering**: Only attempt fixes for high-severity issues
3. **Review timeouts**: Cap review duration to prevent hangs
4. **Parallel review**: Review while other operations proceed
5. **Review metrics**: Track pass/fail rates, common issue types
6. **Incremental fix**: Pass only unfixed issues to subsequent iterations

## References

- [REVIEWER-INTERFACE spec](./REVIEWER-INTERFACE.md) - Defines Reviewer interface and types used here
- [Worker implementation](../../internal/worker/worker.go) - Integration point
- [Events package](../../internal/events/) - Event emission patterns
