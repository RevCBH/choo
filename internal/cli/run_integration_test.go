//go:build integration
// +build integration

package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunOrchestrator_ContextCancellation(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	_ = os.MkdirAll(unitDir, 0755)

	_ = os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	_ = os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
depends_on: []
---
# Long Task
`), 0644)

	// Create a minimal config to avoid auto-detect
	_ = os.WriteFile(filepath.Join(tmpDir, ".choo.yaml"), []byte(`github:
  owner: testowner
  repo: testrepo
`), 0644)

	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Initialize git repo (required for worktree operations)
	initGitRepo(t, tmpDir)

	claudeBin := setupFakeClaude(t, tmpDir)
	t.Setenv("PATH", claudeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := New()
	opts := RunOptions{
		TasksDir:     tasksDir,
		Parallelism:  1,
		TargetBranch: "main",
		NoPR:         true,
	}

	// Cancel after short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := app.RunOrchestrator(ctx, opts)

	// Should return context cancellation
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// initGitRepo initializes a git repository in the given directory with a main branch
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
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
