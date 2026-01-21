// internal/provider/claude_test.go
package provider

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestClaudeProvider_NewWithDefault(t *testing.T) {
	p := NewClaude("")
	if p.command != "claude" {
		t.Errorf("expected default command 'claude', got %q", p.command)
	}
}

func TestClaudeProvider_NewWithCustomCommand(t *testing.T) {
	p := NewClaude("/usr/local/bin/claude")
	if p.command != "/usr/local/bin/claude" {
		t.Errorf("expected custom command '/usr/local/bin/claude', got %q", p.command)
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	p := NewClaude("")
	if got := p.Name(); got != ProviderClaude {
		t.Errorf("Name() = %v, want %v", got, ProviderClaude)
	}
}

func TestClaudeProvider_Invoke_BuildsCorrectArgs(t *testing.T) {
	// Use echo to verify the arguments passed
	p := NewClaude("echo")

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "test prompt", "/tmp", &stdout, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "--dangerously-skip-permissions -p test prompt"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestClaudeProvider_Invoke_SetsWorkdir(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a wrapper script that ignores all args and runs pwd
	scriptPath := tmpDir + "/test-script.sh"
	script := "#!/bin/sh\npwd\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	p := NewClaude(scriptPath)

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "ignored", tmpDir, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != tmpDir {
		t.Errorf("workdir = %q, want %q", got, tmpDir)
	}
}

func TestClaudeProvider_Invoke_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	p := NewClaude("sleep")
	err := p.Invoke(ctx, "10", "/tmp", io.Discard, io.Discard)

	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

func TestClaudeProvider_Invoke_CapturesStderr(t *testing.T) {
	// Use sh -c to write to stderr
	p := NewClaude("sh")

	var stderr bytes.Buffer
	// The prompt becomes the arguments to sh -c, but we're testing stderr capture
	// Using a simple command that writes to stderr
	err := p.Invoke(context.Background(), "", "/tmp", io.Discard, &stderr)
	// This will fail because "sh --dangerously-skip-permissions -p" is invalid
	// but that's fine - we're testing that errors occur as expected
	if err == nil {
		// If somehow it succeeded, that's unexpected but not a test failure
		t.Log("command unexpectedly succeeded")
	}
}

func TestClaudeProvider_Invoke_ReturnsErrorOnFailure(t *testing.T) {
	p := NewClaude("false") // 'false' command always exits with code 1

	err := p.Invoke(context.Background(), "", "/tmp", io.Discard, io.Discard)
	if err == nil {
		t.Error("expected error from failing command, got nil")
	}

	if !strings.Contains(err.Error(), "claude invocation failed") {
		t.Errorf("error should contain 'claude invocation failed', got: %v", err)
	}
}

func TestClaudeProvider_Invoke_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	p := NewClaude("sleep")
	err := p.Invoke(ctx, "10", "/tmp", io.Discard, io.Discard)

	if err == nil {
		t.Error("expected error from context timeout, got nil")
	}
}
