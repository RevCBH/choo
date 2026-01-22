// internal/git/gitops_test.go
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
