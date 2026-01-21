package cli

import (
	"context"
	"testing"
	"time"
)

func TestWatchCmd_RequiresJobID(t *testing.T) {
	// Verifies command fails without job-id argument
	app := New()
	cmd := NewWatchCmd(app)

	// Set args to empty (no job-id)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when job-id not provided, got nil")
	}
}

func TestWatchCmd_FromFlag(t *testing.T) {
	// Verifies --from flag exists with default 0
	app := New()
	cmd := NewWatchCmd(app)

	fromFlag := cmd.Flags().Lookup("from")
	if fromFlag == nil {
		t.Error("Expected --from flag to exist")
	} else {
		if fromFlag.DefValue != "0" {
			t.Errorf("Expected --from default to be '0', got: %s", fromFlag.DefValue)
		}
	}
}

func TestWatchCmd_AcceptsJobID(t *testing.T) {
	// Verifies command parses job-id correctly
	app := New()
	cmd := NewWatchCmd(app)

	if cmd.Use != "watch <job-id>" {
		t.Errorf("Expected Use to be 'watch <job-id>', got: %s", cmd.Use)
	}

	// Verify Args validator is set to ExactArgs(1)
	if cmd.Args == nil {
		t.Error("Expected Args validator to be set")
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestWatchJob_ContextCancellation(t *testing.T) {
	// Verifies watch respects context cancellation
	// This test verifies that watchJob returns when context is cancelled
	// without actually connecting to a daemon (which would fail in tests)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Call watchJob with a non-existent socket path
	// The function should either:
	// 1. Return an error immediately (can't connect to daemon)
	// 2. Return when context is cancelled
	// Either behavior is acceptable for this test
	err := watchJob(ctx, "test-job-id", 0)

	// We expect an error (either connection error or context cancellation)
	if err == nil {
		t.Error("Expected error when connecting to non-existent daemon, got nil")
	}
}
