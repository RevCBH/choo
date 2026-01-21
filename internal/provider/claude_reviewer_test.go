package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewClaudeReviewer_DefaultCommand(t *testing.T) {
	reviewer := NewClaudeReviewer("")
	if reviewer.command != "claude" {
		t.Errorf("Expected default command to be 'claude', got %q", reviewer.command)
	}
}

func TestNewClaudeReviewer_CustomCommand(t *testing.T) {
	customCmd := "/usr/local/bin/claude"
	reviewer := NewClaudeReviewer(customCmd)
	if reviewer.command != customCmd {
		t.Errorf("Expected command to be %q, got %q", customCmd, reviewer.command)
	}
}

func TestClaudeReviewer_Name(t *testing.T) {
	reviewer := NewClaudeReviewer("")
	if reviewer.Name() != ProviderClaude {
		t.Errorf("Expected Name() to return ProviderClaude, got %q", reviewer.Name())
	}
}

func TestClaudeReviewer_Review_EmptyDiff(t *testing.T) {
	// Create a temporary git repo with no changes
	tmpDir := t.TempDir()

	// Initialize git repo
	if err := setupGitRepo(tmpDir); err != nil {
		t.Fatalf("Failed to setup git repo: %v", err)
	}

	reviewer := NewClaudeReviewer("")
	result, err := reviewer.Review(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("Expected no error for empty diff, got %v", err)
	}

	if !result.Passed {
		t.Error("Expected Passed=true for empty diff")
	}

	if result.Summary != "No changes to review" {
		t.Errorf("Expected summary 'No changes to review', got %q", result.Summary)
	}
}

// setupGitRepo initializes a basic git repo for testing
func setupGitRepo(dir string) error {
	// Initialize git repo
	if err := runGitCommand(dir, "init"); err != nil {
		return err
	}

	// Configure user for commits
	if err := runGitCommand(dir, "config", "user.name", "Test User"); err != nil {
		return err
	}
	if err := runGitCommand(dir, "config", "user.email", "test@example.com"); err != nil {
		return err
	}

	// Create initial file and commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content\n"), 0644); err != nil {
		return err
	}

	if err := runGitCommand(dir, "add", "test.txt"); err != nil {
		return err
	}

	if err := runGitCommand(dir, "commit", "-m", "Initial commit"); err != nil {
		return err
	}

	// Create main branch
	if err := runGitCommand(dir, "branch", "-M", "main"); err != nil {
		return err
	}

	return nil
}

// runGitCommand is a helper to run git commands in a directory
func runGitCommand(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %w, output: %s", err, string(output))
	}
	return nil
}
