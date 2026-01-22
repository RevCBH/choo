// internal/git/gitops_test.go
package git

import (
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
