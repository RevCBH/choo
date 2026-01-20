---
task: 2
status: pending
backpressure: "go test ./internal/provider/... -run Codex"
depends_on: []
---

# Implement CodexProvider

**Parent spec**: `/specs/PROVIDER-IMPLEMENTATIONS.md`
**Task**: #2 of 2 in implementation plan

## Objective

Implement the CodexProvider struct that invokes the Codex CLI as a subprocess. The provider runs `codex exec --yolo <prompt>` in the specified working directory with stdout/stderr connected to provided writers.

## Dependencies

### Task Dependencies (within this unit)
- None (can be implemented in parallel with Task #1)

### External Dependencies
- `internal/provider/provider.go` must exist with `Provider` interface and `ProviderCodex` constant (from provider-interface unit)

## Deliverables

### Files to Create/Modify
```
internal/provider/
├── codex.go       # CREATE: CodexProvider implementation
└── codex_test.go  # CREATE: Unit tests
```

### Implementation (codex.go)

```go
// internal/provider/codex.go
package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// CodexProvider implements Provider using the OpenAI Codex CLI.
// Uses exec subcommand with --yolo flag for autonomous execution.
type CodexProvider struct {
	// command is the path to the codex executable.
	// Defaults to "codex" (resolved via PATH).
	command string
}

// NewCodex creates a Codex provider with the specified command path.
// If command is empty, defaults to "codex".
func NewCodex(command string) *CodexProvider {
	if command == "" {
		command = "codex"
	}
	return &CodexProvider{command: command}
}

// Invoke executes Codex CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	args := []string{
		"exec",
		"--yolo",
		prompt,
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex invocation failed: %w", err)
	}
	return nil
}

// Name returns the provider type identifier.
func (p *CodexProvider) Name() ProviderType {
	return ProviderCodex
}
```

### Test File (codex_test.go)

```go
// internal/provider/codex_test.go
package provider

import (
	"bytes"
	"context"
	"io"
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

	// Use pwd to verify working directory
	p := NewCodex("pwd")

	var stdout bytes.Buffer
	err := p.Invoke(context.Background(), "", tmpDir, &stdout, io.Discard)
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
```

## Backpressure

### Validation Command
```bash
go test ./internal/provider/... -run Codex
```

### Success Criteria
- All `*Codex*` tests pass
- `NewCodex("")` returns provider with command "codex"
- `NewCodex("/custom/path")` returns provider with custom command
- `Name()` returns `ProviderCodex`
- `Invoke` builds correct argument list: `exec --yolo <prompt>`
- `Invoke` respects context cancellation

## NOT In Scope
- Factory function `FromConfig` (separate unit: provider-config)
- Integration tests against real Codex CLI
- ClaudeProvider implementation (Task #1)
- Output parsing or structured data extraction
- Retry logic (handled by worker layer)
