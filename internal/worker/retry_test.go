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
