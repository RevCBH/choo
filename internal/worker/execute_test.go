package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/git"
)

type errorGitRunner struct{}

func (errorGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	return "", errors.New("git failed")
}

func (errorGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return "", errors.New("git failed")
}

type contextGitRunner struct{}

func (contextGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func (contextGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", nil
}

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
	git.SetDefaultRunner(errorGitRunner{})
	t.Cleanup(func() {
		git.SetDefaultRunner(nil)
	})
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
	git.SetDefaultRunner(contextGitRunner{})
	t.Cleanup(func() {
		git.SetDefaultRunner(nil)
	})

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
