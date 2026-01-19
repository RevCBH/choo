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
