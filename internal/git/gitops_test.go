//go:build integration
// +build integration

package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewGitOps_EmptyPath(t *testing.T) {
	_, err := NewGitOps("", GitOpsOpts{})
	if !errors.Is(err, ErrEmptyPath) {
		t.Errorf("expected ErrEmptyPath, got %v", err)
	}
}

func TestNewGitOps_RelativePath(t *testing.T) {
	_, err := NewGitOps("./relative/path", GitOpsOpts{})
	if !errors.Is(err, ErrRelativePath) {
		t.Errorf("expected ErrRelativePath, got %v", err)
	}
}

func TestNewGitOps_NonExistentPath(t *testing.T) {
	_, err := NewGitOps("/nonexistent/path/that/does/not/exist", GitOpsOpts{})
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestNewGitOps_PathIsFile(t *testing.T) {
	f, _ := os.CreateTemp("", "gitops-test")
	f.Close()
	defer os.Remove(f.Name())

	_, err := NewGitOps(f.Name(), GitOpsOpts{})
	if !errors.Is(err, ErrNotDirectory) {
		t.Errorf("expected ErrNotDirectory, got %v", err)
	}
}

func TestNewGitOps_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	if !errors.Is(err, ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestNewGitOps_ValidPath(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedPath, _ := filepath.EvalSymlinks(dir)
	if ops.Path() != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, ops.Path())
	}
}

func TestNewGitOps_RepoRootNotAllowed(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	_, err := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: false})
	if !errors.Is(err, ErrRepoRootNotAllowed) {
		t.Errorf("expected ErrRepoRootNotAllowed, got %v", err)
	}
}

func TestGitOps_PathIsImmutable(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

	path1 := ops.Path()
	path2 := ops.Path()
	if path1 != path2 {
		t.Error("Path() returned different values")
	}
}

// Read operation tests

func TestGitOps_ReadStatus_Clean(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	result, err := ops.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Clean {
		t.Error("expected Clean=true for fresh repo")
	}
}

func TestGitOps_ReadStatus_Modified(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	result, _ := ops.Status(context.Background())

	if result.Clean {
		t.Error("expected Clean=false")
	}
	if len(result.Modified) != 1 || result.Modified[0] != "file.txt" {
		t.Errorf("expected Modified=[file.txt], got %v", result.Modified)
	}
}

func TestGitOps_ReadStatus_Staged(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", "file.txt").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	result, _ := ops.Status(context.Background())

	if result.Clean {
		t.Error("expected Clean=false")
	}
	if len(result.Staged) != 1 || result.Staged[0] != "file.txt" {
		t.Errorf("expected Staged=[file.txt], got %v", result.Staged)
	}
}

func TestGitOps_ReadStatus_Untracked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("content"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	result, _ := ops.Status(context.Background())

	if result.Clean {
		t.Error("expected Clean=false")
	}
	if len(result.Untracked) != 1 || result.Untracked[0] != "untracked.txt" {
		t.Errorf("expected Untracked=[untracked.txt], got %v", result.Untracked)
	}
}

func TestGitOps_ReadStatus_Conflicted(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	// Create branch with different change
	exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("feature change"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feature").Run()

	// Go back to main and make conflicting change
	exec.Command("git", "-C", dir, "checkout", "main").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("main change"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "main").Run()

	// Try to merge (will conflict)
	exec.Command("git", "-C", dir, "merge", "feature").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	result, _ := ops.Status(context.Background())

	if result.Clean {
		t.Error("expected Clean=false")
	}
	if len(result.Conflicted) != 1 || result.Conflicted[0] != "file.txt" {
		t.Errorf("expected Conflicted=[file.txt], got %v", result.Conflicted)
	}
}

func TestGitOps_ReadRevParse(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	sha, err := ops.RevParse(context.Background(), "HEAD")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %s (len=%d)", sha, len(sha))
	}
}

func TestGitOps_ReadCurrentBranch(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	branch, err := ops.CurrentBranch(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected main, got %s", branch)
	}
}

func TestGitOps_ReadBranchExists_Local(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", dir, "branch", "feature").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	exists, err := ops.BranchExists(context.Background(), "feature")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected feature branch to exist")
	}
}

func TestGitOps_ReadBranchExists_NotFound(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	exists, err := ops.BranchExists(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected nonexistent branch to not exist")
	}
}

// Write operation tests

func TestGitOps_WriteAdd(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	err := ops.Add(context.Background(), "file.txt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, _ := ops.Status(context.Background())
	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}
}

func TestGitOps_WriteAddAll(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	err := ops.AddAll(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, _ := ops.Status(context.Background())
	if len(status.Staged) != 2 {
		t.Errorf("expected 2 staged files, got %d", len(status.Staged))
	}
}

func TestGitOps_WriteReset(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", "file.txt").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

	// Verify file is staged
	status, _ := ops.Status(context.Background())
	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file before reset, got %d", len(status.Staged))
	}

	err := ops.Reset(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file is unstaged
	status, _ = ops.Status(context.Background())
	if len(status.Staged) != 0 {
		t.Errorf("expected 0 staged files after reset, got %d", len(status.Staged))
	}
	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file after reset, got %d", len(status.Untracked))
	}
}

func TestGitOps_WriteCommit(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "feature", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	err := ops.Commit(context.Background(), "test commit", CommitOpts{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify commit was created
	logs, _ := ops.Log(context.Background(), LogOpts{MaxCount: 1})
	if len(logs) == 0 {
		t.Fatal("expected at least one commit")
	}
	if logs[0].Subject != "test commit" {
		t.Errorf("expected subject 'test commit', got %s", logs[0].Subject)
	}
}

func TestGitOps_WriteCommit_NoVerify(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "feature", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	// Create a pre-commit hook that always fails
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1"), 0755)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

	// Without NoVerify, commit should fail
	err := ops.Commit(context.Background(), "test commit", CommitOpts{NoVerify: false})
	if err == nil {
		t.Error("expected commit to fail without NoVerify")
	}

	// With NoVerify, commit should succeed
	err = ops.Commit(context.Background(), "test commit", CommitOpts{NoVerify: true})
	if err != nil {
		t.Fatalf("unexpected error with NoVerify: %v", err)
	}
}

func TestGitOps_WriteCommit_ProtectedBranch(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot: true,
		BranchGuard:   &BranchGuard{}, // Uses default protected: main, master
	})

	err := ops.Commit(context.Background(), "test", CommitOpts{})

	if !errors.Is(err, ErrProtectedBranch) {
		t.Errorf("expected ErrProtectedBranch, got %v", err)
	}
}

func TestGitOps_WriteCheckoutBranch_Create(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	err := ops.CheckoutBranch(context.Background(), "feature/test", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	branch, _ := ops.CurrentBranch(context.Background())
	if branch != "feature/test" {
		t.Errorf("expected feature/test, got %s", branch)
	}
}

func TestGitOps_WriteCheckoutBranch_Existing(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", dir, "branch", "feature").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
	err := ops.CheckoutBranch(context.Background(), "feature", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	branch, _ := ops.CurrentBranch(context.Background())
	if branch != "feature" {
		t.Errorf("expected feature, got %s", branch)
	}
}

// Destructive operation tests

func TestGitOps_DestructiveCheckoutFiles_Blocked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
	})

	err := ops.CheckoutFiles(context.Background(), ".")
	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
}

func TestGitOps_DestructiveClean_Blocked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
	})

	err := ops.Clean(context.Background(), CleanOpts{Force: true})
	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
}

func TestGitOps_DestructiveResetHard_Blocked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
	})

	err := ops.ResetHard(context.Background(), "HEAD")
	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
}

func TestGitOps_DestructiveCheckoutFiles_Allowed(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	os.WriteFile(filePath, []byte("modified"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: true,
	})

	err := ops.CheckoutFiles(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(filePath)
	if string(content) != "original" {
		t.Errorf("expected 'original', got '%s'", content)
	}
}

func TestGitOps_DestructiveClean_Allowed(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	untrackedPath := filepath.Join(dir, "untracked.txt")
	os.WriteFile(untrackedPath, []byte("untracked"), 0644)

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: true,
	})

	err := ops.Clean(context.Background(), CleanOpts{Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Error("expected untracked file to be removed")
	}
}

func TestGitOps_DestructiveResetHard_Allowed(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "feature", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	os.WriteFile(filePath, []byte("modified"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "modified").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: true,
	})

	// Get the first commit SHA
	sha, _ := ops.RevParse(context.Background(), "HEAD~1")

	err := ops.ResetHard(context.Background(), sha)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(filePath)
	if string(content) != "original" {
		t.Errorf("expected 'original', got '%s'", content)
	}
}

// Remote operation tests

func TestGitOps_RemoteForcePush_Blocked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
	})

	err := ops.Push(context.Background(), "origin", "main", PushOpts{Force: true})
	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
}

func TestGitOps_RemoteForceWithLeasePush_Blocked(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
	})

	err := ops.Push(context.Background(), "origin", "main", PushOpts{ForceWithLease: true})
	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed for force-with-lease, got %v", err)
	}
}

// Merge operation tests

func TestGitOps_MergeFastForward(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create initial commit on main
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	// Create feature branch with a new commit
	exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feature").Run()

	// Go back to main and merge
	exec.Command("git", "-C", dir, "checkout", "main").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

	err := ops.Merge(context.Background(), "feature", MergeOpts{FFOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify feature file now exists on main
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); os.IsNotExist(err) {
		t.Error("expected feature.txt to exist after merge")
	}
}

func TestGitOps_MergeAbort(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", "-b", "main", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	// Create branch with conflicting change
	exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("feature change"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feature").Run()

	// Go back to main and make conflicting change
	exec.Command("git", "-C", dir, "checkout", "main").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("main change"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "main").Run()

	// Try to merge (will conflict)
	exec.Command("git", "-C", dir, "merge", "feature").Run()

	ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})

	err := ops.MergeAbort(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we're back to clean state with main's content
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "main change" {
		t.Errorf("expected 'main change', got '%s'", content)
	}
}
