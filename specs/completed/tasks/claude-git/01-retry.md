---
task: 1
status: complete
backpressure: "go test ./internal/worker/... -run Retry"
depends_on: []
---

# Retry Utilities

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #1 of 6 in implementation plan

## Objective

Implement retry utilities with exponential backoff for transient git operation failures.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── retry.go       # CREATE: Retry types and RetryWithBackoff function
└── retry_test.go  # CREATE: Retry tests
```

### Types to Implement

```go
// internal/worker/retry.go

package worker

import (
    "context"
    "time"
)

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

### Functions to Implement

```go
// RetryWithBackoff retries an operation with exponential backoff.
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

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Retry
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestRetryWithBackoff_SuccessOnFirstAttempt | result.Success == true, result.Attempts == 1, callCount == 1 |
| TestRetryWithBackoff_SuccessOnSecondAttempt | result.Success == true, result.Attempts == 2 |
| TestRetryWithBackoff_ExhaustsRetries | result.Success == false, result.Attempts == MaxAttempts, result.LastErr != nil |
| TestRetryWithBackoff_RespectsContext | result.Success == false, result.LastErr == context.Canceled |
| TestRetryWithBackoff_BackoffIncreases | delays increase between retries |

### Test Implementations

```go
// internal/worker/retry_test.go

package worker

import (
    "context"
    "fmt"
    "testing"
    "time"
)

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

## NOT In Scope

- Integration with escalation system (handled in delegate tasks)
- Git-specific retry logic (all errors treated as transient)
- Jitter for backoff (not required per spec)
