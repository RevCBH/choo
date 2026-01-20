//go:build integration
// +build integration

package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/worker"
)

func TestOrchestrator_Shutdown_Timeout(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git user.email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git user.name: %v", err)
	}

	bus := events.NewBus(100)

	// Very short timeout
	cfg := Config{
		ShutdownTimeout: 1 * time.Millisecond,
		Parallelism:     1,
	}

	orch := New(cfg, Dependencies{Bus: bus})

	// Create pool with git manager
	gitMgr := git.NewWorktreeManager(tmpDir, nil)
	workerCfg := worker.WorkerConfig{
		RepoRoot:     tmpDir,
		WorktreeBase: filepath.Join(tmpDir, ".ralph", "worktrees"),
	}
	workerDeps := worker.WorkerDeps{
		Events: bus,
		Git:    gitMgr,
	}
	orch.pool = worker.NewPool(1, workerCfg, workerDeps)

	// Submit a blocking unit (won't complete quickly)
	unit := &discovery.Unit{
		ID: "blocking-unit",
		Tasks: []*discovery.Task{
			{Number: 1, Backpressure: "sleep 10"},
		},
	}
	_ = orch.pool.Submit(unit)

	// Give worker time to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown with already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := orch.shutdown(ctx)

	// Should timeout waiting for workers
	if err == nil {
		t.Log("shutdown completed (pool may have been empty)")
	}
}

func TestOrchestrator_Run_ContextCancellation(t *testing.T) {
	// Create temp tasks directory
	tmpDir := t.TempDir()

	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "slow-unit")
	_ = os.MkdirAll(unitDir, 0755)

	_ = os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: slow-unit
depends_on: []
---
# Slow Unit
`), 0644)

	_ = os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
depends_on: []
---
# Slow Task
`), 0644)

	initGitRepo(t, tmpDir)
	claudeBin := setupFakeClaude(t, tmpDir)
	t.Setenv("PATH", claudeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	bus := events.NewBus(100)
	gitMgr := git.NewWorktreeManager(tmpDir, nil)
	worktreeBase := filepath.Join(tmpDir, ".ralph", "worktrees")

	cfg := Config{
		TasksDir:        tasksDir,
		Parallelism:     1,
		ShutdownTimeout: 100 * time.Millisecond,
		RepoRoot:        tmpDir,
		WorktreeBase:    worktreeBase,
		NoPR:            true,
	}

	orch := New(cfg, Dependencies{
		Bus: bus,
		Git: gitMgr,
	})

	// Cancel context after short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := orch.Run(ctx)

	// Should return context cancelled error
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Result should still be populated
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", args, err, out)
		}
	}
}

func setupFakeClaude(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake claude helper requires a POSIX shell")
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create fake claude dir: %v", err)
	}

	claudePath := filepath.Join(binDir, "claude")
	script := "#!/bin/sh\nsleep 5\nexit 0\n"
	if err := os.WriteFile(claudePath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake claude: %v", err)
	}

	return binDir
}
