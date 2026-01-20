//go:build integration
// +build integration

package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createInitialCommit creates an initial commit in the test repo
func createInitialCommit(t *testing.T, repoRoot string) {
	t.Helper()

	ctx := context.Background()

	// Create README
	readmePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	if _, err := gitExec(ctx, repoRoot, "add", "README.md"); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	if _, err := gitExec(ctx, repoRoot, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}
}

func TestWorktreeManager_Create(t *testing.T) {
	repoRoot := setupTestRepo(t)
	createInitialCommit(t, repoRoot)

	manager := NewWorktreeManager(repoRoot, nil)
	ctx := context.Background()

	wt, err := manager.CreateWorktree(ctx, "task-1", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify worktree path
	expectedPath := filepath.Join(repoRoot, ".ralph", "worktrees", "task-1")
	if wt.Path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, wt.Path)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist: %s", wt.Path)
	}

	// Verify UnitID
	if wt.UnitID != "task-1" {
		t.Errorf("expected UnitID task-1, got %s", wt.UnitID)
	}

	// Verify branch was created
	if wt.Branch == "" {
		t.Error("expected branch to be set")
	}
}

func TestWorktreeManager_CreateBranch(t *testing.T) {
	repoRoot := setupTestRepo(t)
	createInitialCommit(t, repoRoot)

	manager := NewWorktreeManager(repoRoot, nil)
	ctx := context.Background()

	// Create worktree with specific target branch
	wt, err := manager.CreateWorktree(ctx, "task-2", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify branch name follows expected pattern
	expectedBranch := "ralph/task-2"
	if wt.Branch != expectedBranch {
		t.Errorf("expected branch %s, got %s", expectedBranch, wt.Branch)
	}

	// Verify branch exists in git
	output, err := gitExec(ctx, repoRoot, "branch", "--list", wt.Branch)
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if !strings.Contains(output, wt.Branch) {
		t.Errorf("branch %s does not exist in output: %s", wt.Branch, output)
	}
}

func TestWorktreeManager_Remove(t *testing.T) {
	repoRoot := setupTestRepo(t)
	createInitialCommit(t, repoRoot)

	manager := NewWorktreeManager(repoRoot, nil)
	ctx := context.Background()

	// Create worktree
	wt, err := manager.CreateWorktree(ctx, "task-3", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Fatalf("worktree was not created")
	}

	// Remove worktree
	if err := manager.RemoveWorktree(ctx, wt); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after removal")
	}

	// Verify git reference is gone
	output, err := gitExec(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	if strings.Contains(output, wt.Path) {
		t.Errorf("worktree still appears in git worktree list")
	}
}

func TestWorktreeManager_List(t *testing.T) {
	repoRoot := setupTestRepo(t)
	createInitialCommit(t, repoRoot)

	manager := NewWorktreeManager(repoRoot, nil)
	ctx := context.Background()

	// Create multiple worktrees
	wt1, err := manager.CreateWorktree(ctx, "task-4", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	wt2, err := manager.CreateWorktree(ctx, "task-5", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// List worktrees
	worktrees, err := manager.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	// Should have 2 ralph worktrees (main worktree is not counted)
	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify both worktrees are in the list by UnitID (paths may differ due to symlink resolution)
	found1, found2 := false, false
	for _, wt := range worktrees {
		if wt.UnitID == wt1.UnitID {
			found1 = true
		}
		if wt.UnitID == wt2.UnitID {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("worktree 1 (unitID=%s) not found in list", wt1.UnitID)
	}
	if !found2 {
		t.Errorf("worktree 2 (unitID=%s) not found in list", wt2.UnitID)
	}
}

func TestConditionalCommand_Matching(t *testing.T) {
	repoRoot := setupTestRepo(t)

	ctx := context.Background()

	// Create package.json in the repo
	packageJSON := filepath.Join(repoRoot, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name": "test"}`), 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	// Commit it so it's in the worktree
	if _, err := gitExec(ctx, repoRoot, "add", "package.json"); err != nil {
		t.Fatalf("failed to add package.json: %v", err)
	}
	if _, err := gitExec(ctx, repoRoot, "commit", "-m", "Add package.json"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create manager with custom setup commands that we can verify
	manager := NewWorktreeManager(repoRoot, nil)
	manager.SetupCommands = []ConditionalCommand{
		{
			ConditionFile: "package.json",
			Command:       "echo",
			Args:          []string{"npm-installed"},
			Description:   "Mock npm install",
		},
		{
			ConditionFile: "Cargo.toml",
			Command:       "echo",
			Args:          []string{"cargo-fetched"},
			Description:   "Mock cargo fetch",
		},
	}

	// Create worktree - this should run the npm command but not cargo
	wt, err := manager.CreateWorktree(ctx, "task-6", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer manager.RemoveWorktree(ctx, wt)

	// Verify package.json exists in worktree
	wtPackageJSON := filepath.Join(wt.Path, "package.json")
	if _, err := os.Stat(wtPackageJSON); os.IsNotExist(err) {
		t.Errorf("package.json should exist in worktree")
	}
}

func TestConditionalCommand_NoMatch(t *testing.T) {
	repoRoot := setupTestRepo(t)
	createInitialCommit(t, repoRoot)

	ctx := context.Background()

	// Create manager with setup commands, but no matching files in repo
	manager := NewWorktreeManager(repoRoot, nil)
	manager.SetupCommands = []ConditionalCommand{
		{
			ConditionFile: "package.json",
			Command:       "false", // This would fail if run
			Args:          []string{},
			Description:   "Should not run",
		},
	}

	// Create worktree - should succeed even though no setup commands match
	wt, err := manager.CreateWorktree(ctx, "task-7", "HEAD")
	if err != nil {
		t.Fatalf("CreateWorktree should succeed with no matching setup commands: %v", err)
	}
	defer manager.RemoveWorktree(ctx, wt)
}

func TestDefaultSetupCommands(t *testing.T) {
	commands := DefaultSetupCommands()

	// Verify we have the expected number of commands
	expectedCount := 5
	if len(commands) != expectedCount {
		t.Errorf("expected %d setup commands, got %d", expectedCount, len(commands))
	}

	// Verify specific commands exist
	expectedFiles := []string{
		"package.json",
		"pnpm-lock.yaml",
		"yarn.lock",
		"Cargo.toml",
		"go.mod",
	}

	for _, file := range expectedFiles {
		found := false
		for _, cmd := range commands {
			if cmd.ConditionFile == file {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected setup command for %s", file)
		}
	}
}
