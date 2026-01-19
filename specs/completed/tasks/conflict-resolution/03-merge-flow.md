---
task: 3
status: complete
backpressure: go test ./internal/worker/... -run Retry && go test ./internal/worker/... -run MergeConflict
depends_on: [1, 2]
---

# Merge Flow with Conflict Resolution

**Parent spec**: `/specs/CONFLICT-RESOLUTION.md`
**Task**: #3 of 4 in implementation plan

## Objective

Implement the `mergeWithConflictResolution` worker method that handles the full merge flow including conflict detection, Claude delegation, and retry with exponential backoff.

## Dependencies

### External Specs (must be implemented)
- EVENTS spec (completed) - provides event types and bus
- ESCALATION spec - provides Escalator interface

### Task Dependencies (within this unit)
- Task 1: Git helpers (IsRebaseInProgress, AbortRebase, GetConflictedFiles)
- Task 2: BuildConflictPrompt function

## Deliverables

### Files to Create/Modify
```
internal/worker/
├── merge.go        # NEW: Merge flow with conflict resolution
├── merge_test.go   # NEW: Tests for merge flow
├── retry.go        # NEW: Retry with backoff utilities
└── retry_test.go   # NEW: Tests for retry utilities
```

### Types to Add

```go
// internal/worker/merge.go
package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
)

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
package worker

import (
	"context"
	"time"
)

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

// RetryWithBackoff executes a function with exponential backoff retry
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

### Main Merge Flow

```go
// internal/worker/merge.go (continued)

// mergeWithConflictResolution performs a full merge with conflict handling
// This is called by the worker after PR approval
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

// invokeClaude invokes Claude CLI to resolve conflicts
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
	// This will be implemented using exec.CommandContext
	// For now, the interface allows mocking in tests
	return nil
}
```

### Tests

```go
// internal/worker/retry_test.go
package worker

import (
	"context"
	"fmt"
	"testing"
	"time"
)

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
// internal/worker/merge_test.go
package worker

import (
	"context"
	"testing"
)

func TestMergeConflictResolution_NoConflicts(t *testing.T) {
	// Test that when rebase succeeds without conflicts,
	// forcePushAndMerge is called directly
	// This requires mocking git operations
	t.Skip("requires git mocking infrastructure")
}

func TestMergeConflictResolution_WithConflicts(t *testing.T) {
	// Test that when rebase has conflicts:
	// 1. PRConflict event is emitted
	// 2. Claude is invoked with conflict prompt
	// 3. Retry logic is applied
	t.Skip("requires git and Claude mocking infrastructure")
}

func TestMergeConflictResolution_EscalateOnFailure(t *testing.T) {
	// Test that after max retries:
	// 1. Rebase is aborted
	// 2. Escalation is sent
	// 3. Error is returned
	t.Skip("requires git and escalator mocking infrastructure")
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/worker/... -run "Retry|MergeConflict"
```

## NOT In Scope
- Git helper implementations (Task 1)
- Prompt building (Task 2)
- Force push and PR merge implementation (Task 4)
- Full Claude CLI invocation (separate concern)
