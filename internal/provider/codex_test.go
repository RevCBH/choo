// internal/provider/codex_test.go
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

func TestCodexProvider_NewWithDefault(t *testing.T) {
	p := NewCodex("")
	if p.command != "codex" {
		t.Errorf("expected default command 'codex', got %q", p.command)
	}
}

func TestCodexProvider_NewWithCustomCommand(t *testing.T) {
	p := NewCodex("/opt/codex/bin/codex")
	if p.command != "/opt/codex/bin/codex" {
		t.Errorf("expected custom command '/opt/codex/bin/codex', got %q", p.command)
	}
}

func TestCodexProvider_Name(t *testing.T) {
	p := NewCodex("")
	if got := p.Name(); got != ProviderCodex {
		t.Errorf("Name() = %v, want %v", got, ProviderCodex)
	}
}

func TestCodexProvider_Invoke_BuildsCorrectArgs(t *testing.T) {
	// Use echo to verify the arguments passed
	p := NewCodex("echo")

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "test prompt", "/tmp", &stdout, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "exec --yolo test prompt"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestCodexProvider_Invoke_SetsWorkdir(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a wrapper script that ignores all args and runs pwd
	scriptPath := tmpDir + "/test-script.sh"
	script := "#!/bin/sh\npwd\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	p := NewCodex(scriptPath)

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

func TestCodexProvider_Invoke_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	p := NewCodex("sleep")
	err := p.Invoke(ctx, "10", "/tmp", io.Discard, io.Discard)

	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

func TestCodexProvider_Invoke_CapturesStdout(t *testing.T) {
	// Use echo which will output the args
	p := NewCodex("echo")

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "hello world", "/tmp", &stdout, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout.String(), "hello world") {
		t.Errorf("stdout should contain 'hello world', got: %q", stdout.String())
	}
}

func TestCodexProvider_Invoke_ReturnsErrorOnFailure(t *testing.T) {
	p := NewCodex("false") // 'false' command always exits with code 1

	err := p.Invoke(context.Background(), "", "/tmp", io.Discard, io.Discard)
	if err == nil {
		t.Error("expected error from failing command, got nil")
	}

	if !strings.Contains(err.Error(), "codex invocation failed") {
		t.Errorf("error should contain 'codex invocation failed', got: %v", err)
	}
}

func TestCodexProvider_Invoke_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	p := NewCodex("sleep")
	err := p.Invoke(ctx, "10", "/tmp", io.Discard, io.Discard)

	if err == nil {
		t.Error("expected error from context timeout, got nil")
	}
}

func TestCodexProvider_Invoke_EmptyPrompt(t *testing.T) {
	// Verify that empty prompt is handled correctly (passed as empty string)
	p := NewCodex("echo")

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "", "/tmp", &stdout, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := "exec --yolo"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}
