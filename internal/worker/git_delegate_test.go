package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestGitRepo creates a temporary git repo for testing
func setupTestGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-helper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			cleanup()
			t.Fatalf("git setup failed: %v", err)
		}
	}

	// Create initial commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to write test file: %v", err)
	}

	cmds = [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", "initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			cleanup()
			t.Fatalf("git commit failed: %v", err)
		}
	}

	return dir, cleanup
}

func TestGetHeadRef_ReturnsCommitSHA(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	ref, err := w.getHeadRef(context.Background())
	if err != nil {
		t.Fatalf("getHeadRef failed: %v", err)
	}

	// SHA should be 40 hex characters
	if len(ref) != 40 {
		t.Errorf("expected 40-char SHA, got %d chars: %s", len(ref), ref)
	}

	for _, c := range ref {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("SHA contains non-hex character: %c", c)
		}
	}
}

func TestHasNewCommit_FalseWhenSame(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	ref, _ := w.getHeadRef(context.Background())

	hasNew, err := w.hasNewCommit(context.Background(), ref)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if hasNew {
		t.Error("expected false when HEAD unchanged")
	}
}

func TestHasNewCommit_TrueAfterCommit(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	refBefore, _ := w.getHeadRef(context.Background())

	// Create new commit
	testFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(testFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmds := [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", "new commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command failed: %v", err)
		}
	}

	hasNew, err := w.hasNewCommit(context.Background(), refBefore)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if !hasNew {
		t.Error("expected true after new commit")
	}
}

func TestBranchExistsOnRemote_FalseForNonexistent(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}

	// No remote configured, so any branch should return false
	exists, err := w.branchExistsOnRemote(context.Background(), "nonexistent-branch")
	if err != nil {
		// Error expected since no remote exists
		return
	}
	if exists {
		t.Error("expected false for non-existent branch")
	}
}

func TestGetChangedFiles_Empty(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	files, err := w.getChangedFiles(context.Background())
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty slice for clean worktree, got %v", files)
	}
}

func TestGetChangedFiles_ParsesPorcelain(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create untracked file
	newFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(newFile, []byte("untracked"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Modify existing file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	w := &Worker{worktreePath: dir}
	files, err := w.getChangedFiles(context.Background())
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}

	hasUntracked := false
	hasModified := false
	for _, f := range files {
		if f == "untracked.txt" {
			hasUntracked = true
		}
		if f == "test.txt" {
			hasModified = true
		}
	}

	if !hasUntracked {
		t.Error("expected untracked.txt in changed files")
	}
	if !hasModified {
		t.Error("expected test.txt in changed files")
	}
}
