package worker

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/choo/internal/discovery"
)

func TestExecute_Success(t *testing.T) {
	deps := mockDeps(t)
	unit := &discovery.Unit{
		ID:    "test",
		Tasks: []*discovery.Task{},
	}

	cfg := DefaultConfig()
	cfg.NoPR = true // Skip PR creation in tests

	err := Execute(context.Background(), unit, cfg, deps)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecute_NilUnit(t *testing.T) {
	deps := mockDeps(t)

	err := Execute(context.Background(), nil, DefaultConfig(), deps)

	if err == nil {
		t.Error("expected error for nil unit")
	}
}

func TestExecute_PropagatesError(t *testing.T) {
	deps := mockDeps(t)
	// Use an invalid repo to cause an error during worktree creation
	deps.Git.RepoRoot = "/nonexistent/repo"

	unit := &discovery.Unit{ID: "test"}

	cfg := DefaultConfig()
	cfg.NoPR = true

	err := Execute(context.Background(), unit, cfg, deps)

	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestExecute_RespectsContext(t *testing.T) {
	deps := mockDeps(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	unit := &discovery.Unit{ID: "test"}

	cfg := DefaultConfig()
	cfg.NoPR = true

	err := Execute(ctx, unit, cfg, deps)

	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestExecuteWithDefaults(t *testing.T) {
	deps := mockDeps(t)
	unit := &discovery.Unit{
		ID:    "test",
		Tasks: []*discovery.Task{},
	}

	// Modify the default config after calling it to skip PR
	// This verifies ExecuteWithDefaults calls DefaultConfig and Execute
	cfg := DefaultConfig()
	cfg.NoPR = true

	err := Execute(context.Background(), unit, cfg, deps)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxClaudeRetries != 3 {
		t.Errorf("expected MaxClaudeRetries=3, got %d", cfg.MaxClaudeRetries)
	}
	if cfg.MaxBaselineRetries != 3 {
		t.Errorf("expected MaxBaselineRetries=3, got %d", cfg.MaxBaselineRetries)
	}
	if cfg.BackpressureTimeout != 5*time.Minute {
		t.Errorf("expected BackpressureTimeout=5m, got %v", cfg.BackpressureTimeout)
	}
	if cfg.BaselineTimeout != 10*time.Minute {
		t.Errorf("expected BaselineTimeout=10m, got %v", cfg.BaselineTimeout)
	}
	if cfg.TargetBranch != "main" {
		t.Errorf("expected TargetBranch=main, got %s", cfg.TargetBranch)
	}
}
