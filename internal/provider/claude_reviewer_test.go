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

// Tests for JSON extraction functions

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"passed": true, "summary": "All good", "issues": []}`
	result := extractJSON(input)
	if result != input {
		t.Errorf("Expected %q, got %q", input, result)
	}
}

func TestExtractJSON_JSONWithPrefixText(t *testing.T) {
	input := `Here is my review:
{"passed": true, "summary": "All good", "issues": []}`
	expected := `{"passed": true, "summary": "All good", "issues": []}`
	result := extractJSON(input)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestExtractJSON_JSONWithSuffixText(t *testing.T) {
	input := `{"passed": true, "summary": "All good", "issues": []}
That's all for the review!`
	expected := `{"passed": true, "summary": "All good", "issues": []}`
	result := extractJSON(input)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"passed": false, "summary": "Issues found", "issues": [{"file": "test.go", "line": 10, "severity": "error", "message": "Bug here"}]}`
	result := extractJSON(input)
	if result != input {
		t.Errorf("Expected %q, got %q", input, result)
	}
}

func TestExtractJSON_MarkdownCodeFence(t *testing.T) {
	input := "Here is the review:\n```json\n{\"passed\": true, \"summary\": \"All good\", \"issues\": []}\n```\nThat's it!"
	expected := `{"passed": true, "summary": "All good", "issues": []}`
	result := extractJSON(input)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestExtractJSON_PlainCodeFence(t *testing.T) {
	input := "Here is the review:\n```\n{\"passed\": true, \"summary\": \"All good\", \"issues\": []}\n```\nThat's it!"
	expected := `{"passed": true, "summary": "All good", "issues": []}`
	result := extractJSON(input)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "This is just plain text without any JSON"
	result := extractJSON(input)
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestExtractJSON_IncompleteJSON(t *testing.T) {
	input := `{"passed": true, "summary": "All good", "issues": [`
	result := extractJSON(input)
	if result != "" {
		t.Errorf("Expected empty string for incomplete JSON, got %q", result)
	}
}

func TestClaudeReviewer_ParseOutput_ValidJSON(t *testing.T) {
	reviewer := NewClaudeReviewer("")
	output := `{"passed": true, "summary": "All good", "issues": []}`
	result, err := reviewer.parseOutput(output)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !result.Passed {
		t.Error("Expected Passed=true")
	}
	if result.Summary != "All good" {
		t.Errorf("Expected summary 'All good', got %q", result.Summary)
	}
	if len(result.Issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(result.Issues))
	}
	if result.RawOutput != output {
		t.Error("Expected RawOutput to be preserved")
	}
}

func TestClaudeReviewer_ParseOutput_NoJSON(t *testing.T) {
	reviewer := NewClaudeReviewer("")
	output := "This is just plain text without any JSON"
	result, err := reviewer.parseOutput(output)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !result.Passed {
		t.Error("Expected Passed=true for graceful degradation")
	}
	if result.Summary != "No structured review output" {
		t.Errorf("Expected summary 'No structured review output', got %q", result.Summary)
	}
	if result.RawOutput != output {
		t.Error("Expected RawOutput to be preserved")
	}
}

func TestClaudeReviewer_ParseOutput_MalformedJSON(t *testing.T) {
	reviewer := NewClaudeReviewer("")
	// Use malformed but structurally complete JSON (invalid field type)
	output := `{"passed": "not a boolean", "summary": "All good", "issues": []}`
	result, err := reviewer.parseOutput(output)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !result.Passed {
		t.Error("Expected Passed=true for graceful degradation")
	}
	if result.Summary != "Failed to parse review output" {
		t.Errorf("Expected summary 'Failed to parse review output', got %q", result.Summary)
	}
	if result.RawOutput != output {
		t.Error("Expected RawOutput to be preserved")
	}
}
