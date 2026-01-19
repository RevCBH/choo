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
