package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/choo/internal/discovery"
)

// Execute runs a single unit to completion (convenience wrapper)
// This is the primary entry point for single-unit execution
func Execute(ctx context.Context, unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) error {
	// Validate inputs
	if unit == nil {
		return fmt.Errorf("unit cannot be nil")
	}

	// Create worker
	worker := NewWorker(unit, cfg, deps)

	// Run worker
	if err := worker.Run(ctx); err != nil {
		return err
	}

	return nil
}

// ExecuteWithDefaults runs a unit with sensible default configuration
func ExecuteWithDefaults(ctx context.Context, unit *discovery.Unit, deps WorkerDeps) error {
	cfg := DefaultConfig()
	return Execute(ctx, unit, cfg, deps)
}

// DefaultConfig returns a WorkerConfig with sensible defaults
func DefaultConfig() WorkerConfig {
	return WorkerConfig{
		MaxClaudeRetries:    3,
		MaxBaselineRetries:  3,
		BackpressureTimeout: 5 * time.Minute,
		BaselineTimeout:     10 * time.Minute,
		WorktreeBase:        "/tmp/ralph-worktrees",
		TargetBranch:        "main",
	}
}
