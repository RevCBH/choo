package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeManager_Create_UsesExpectedBranchAndPath(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("worktree list --porcelain", "", nil) // No existing worktrees
	runner.stub("worktree add -b ralph/unit-1 /tmp/repo/.ralph/worktrees/unit-1 HEAD", "", nil)
	useRunner(t, runner)

	manager := NewWorktreeManager("/tmp/repo", nil)
	wt, err := manager.CreateWorktree(context.Background(), "unit-1", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if !strings.HasSuffix(wt.Path, filepath.Join(".ralph", "worktrees", "unit-1")) {
		t.Errorf("unexpected worktree path: %s", wt.Path)
	}
	if wt.Branch != "ralph/unit-1" {
		t.Errorf("expected branch ralph/unit-1, got %s", wt.Branch)
	}
}

func TestWorktreeManager_Remove_DeletesDirectory(t *testing.T) {
	dir := t.TempDir()
	worktreeDir := filepath.Join(dir, ".ralph", "worktrees", "unit-2")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	runner := newFakeRunner()
	runner.stub("worktree remove "+worktreeDir+" --force", "", nil)
	useRunner(t, runner)

	manager := NewWorktreeManager(dir, nil)
	if err := manager.RemoveWorktree(context.Background(), &Worktree{
		Path:   worktreeDir,
		Branch: "ralph/unit-2",
		UnitID: "unit-2",
	}); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed")
	}
}

func TestWorktreeManager_List_ParsesOutput(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".ralph", "worktrees")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("failed to create base dir: %v", err)
	}
	resolvedBase, _ := filepath.EvalSymlinks(base)

	output := strings.Join([]string{
		"worktree " + filepath.Join(resolvedBase, "unit-a"),
		"branch refs/heads/ralph/unit-a",
		"",
		"worktree " + filepath.Join(resolvedBase, "unit-b"),
		"branch refs/heads/ralph/unit-b",
		"",
	}, "\n")

	runner := newFakeRunner()
	runner.stub("worktree list --porcelain", output, nil)
	useRunner(t, runner)

	manager := NewWorktreeManager(dir, nil)
	worktrees, err := manager.ListWorktrees(context.Background())
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	found := map[string]bool{}
	for _, wt := range worktrees {
		found[wt.UnitID] = true
	}
	if !found["unit-a"] || !found["unit-b"] {
		t.Errorf("expected unit-a and unit-b in list, got %+v", found)
	}
}

func TestWorktreeManager_IsWorktreeResumable(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".ralph", "worktrees", "unit-1")
	unitDir := filepath.Join(worktreePath, "specs", "tasks", "unit-1")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	spec := `---
task: 1
status: complete
backpressure: go test ./...
---
`
	if err := os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	manager := NewWorktreeManager(repoRoot, nil)
	if !manager.isWorktreeResumable(worktreePath, "unit-1") {
		t.Fatalf("expected worktree to be resumable")
	}
}

func TestWorktreeManager_IsWorktreeResumable_PendingOnly(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".ralph", "worktrees", "unit-2")
	unitDir := filepath.Join(worktreePath, "specs", "tasks", "unit-2")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	spec := `---
task: 1
status: pending
backpressure: go test ./...
---
`
	if err := os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(spec), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	manager := NewWorktreeManager(repoRoot, nil)
	if manager.isWorktreeResumable(worktreePath, "unit-2") {
		t.Fatalf("expected worktree to be non-resumable")
	}
}
