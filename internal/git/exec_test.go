package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git user: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to config git email: %v", err)
	}

	return tmpDir
}

func TestGitExec_Success(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Test git status command
	ctx := context.Background()
	output, err := gitExec(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("gitExec failed: %v", err)
	}

	// Should return empty string for clean repo
	if output != "" {
		t.Errorf("expected empty output for clean repo, got: %q", output)
	}
}

func TestGitExec_Failure(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Try to checkout a non-existent branch
	ctx := context.Background()
	_, err := gitExec(ctx, repoDir, "checkout", "non-existent-branch")
	if err == nil {
		t.Fatal("expected error for non-existent branch, got nil")
	}

	// Error should contain stderr information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed") {
		t.Errorf("error should contain 'failed', got: %q", errMsg)
	}
	if !strings.Contains(errMsg, "stderr:") {
		t.Errorf("error should contain stderr output, got: %q", errMsg)
	}
}

func TestGitExec_Context(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to run a git command with cancelled context
	_, err := gitExec(ctx, repoDir, "status")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	// Should contain context cancellation error
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestGitExec_ContextTimeout(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure timeout
	time.Sleep(10 * time.Millisecond)

	// Try to run a git command
	_, err := gitExec(ctx, repoDir, "status")
	if err == nil {
		t.Fatal("expected error for timeout context, got nil")
	}
}

func TestGitExec_Directory(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create a test file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add the file
	ctx := context.Background()
	_, err := gitExec(ctx, repoDir, "add", "test.txt")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	// Check status to verify file was added in correct directory
	output, err := gitExec(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if !strings.Contains(output, "test.txt") {
		t.Errorf("expected test.txt in status output, got: %q", output)
	}
	if !strings.Contains(output, "A") {
		t.Errorf("expected file to be added (A), got: %q", output)
	}
}

func TestGitExec_StdoutCapture(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create and commit a file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ctx := context.Background()
	_, err := gitExec(ctx, repoDir, "add", "test.txt")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = gitExec(ctx, repoDir, "commit", "-m", "Test commit")
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Get the commit log
	output, err := gitExec(ctx, repoDir, "log", "--oneline", "-n", "1")
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}

	if !strings.Contains(output, "Test commit") {
		t.Errorf("expected commit message in log, got: %q", output)
	}
}

func TestGitExecWithStdin_Success(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Use hash-object to test stdin
	ctx := context.Background()
	stdin := "test content for stdin"
	output, err := gitExecWithStdin(ctx, repoDir, stdin, "hash-object", "--stdin")
	if err != nil {
		t.Fatalf("gitExecWithStdin failed: %v", err)
	}

	// Should return a hash
	output = strings.TrimSpace(output)
	if len(output) != 40 {
		t.Errorf("expected 40 character hash, got %d characters: %q", len(output), output)
	}
}

func TestGitExecWithStdin_Failure(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Try invalid command with stdin
	ctx := context.Background()
	_, err := gitExecWithStdin(ctx, repoDir, "test", "invalid-command")
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}

	// Error should contain stderr information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed") {
		t.Errorf("error should contain 'failed', got: %q", errMsg)
	}
}

func TestGitExecWithStdin_Context(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to run a git command with cancelled context
	_, err := gitExecWithStdin(ctx, repoDir, "test", "hash-object", "--stdin")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}

	// Should contain context cancellation error
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestGitExecWithStdin_Directory(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Use hash-object which respects the directory setting
	ctx := context.Background()
	stdin := "test content"
	output, err := gitExecWithStdin(ctx, repoDir, stdin, "hash-object", "--stdin")
	if err != nil {
		t.Fatalf("gitExecWithStdin failed: %v", err)
	}

	// Should successfully execute in the specified directory
	if len(strings.TrimSpace(output)) != 40 {
		t.Errorf("expected valid hash output, got: %q", output)
	}
}
