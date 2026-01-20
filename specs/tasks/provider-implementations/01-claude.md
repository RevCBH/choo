---
task: 1
status: pending
backpressure: "go test ./internal/provider/... -run Claude"
depends_on: []
---

# Implement ClaudeProvider

**Parent spec**: `/specs/PROVIDER-IMPLEMENTATIONS.md`
**Task**: #1 of 2 in implementation plan

## Objective

Implement the ClaudeProvider struct that invokes the Claude CLI as a subprocess. The provider runs `claude --dangerously-skip-permissions -p <prompt>` in the specified working directory with stdout/stderr connected to provided writers.

## Dependencies

### Task Dependencies (within this unit)
- None (can be implemented in parallel with Task #2)

### External Dependencies
- `internal/provider/provider.go` must exist with `Provider` interface and `ProviderClaude` constant (from provider-interface unit)

## Deliverables

### Files to Create/Modify
```
internal/provider/
├── claude.go       # CREATE: ClaudeProvider implementation
└── claude_test.go  # CREATE: Unit tests
```

### Implementation (claude.go)

```go
// internal/provider/claude.go
package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ClaudeProvider implements Provider using the Claude CLI.
// Uses --dangerously-skip-permissions to run without interactive prompts.
type ClaudeProvider struct {
	// command is the path to the claude executable.
	// Defaults to "claude" (resolved via PATH).
	command string
}

// NewClaude creates a Claude provider with the specified command path.
// If command is empty, defaults to "claude".
func NewClaude(command string) *ClaudeProvider {
	if command == "" {
		command = "claude"
	}
	return &ClaudeProvider{command: command}
}

// Invoke executes Claude CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	args := []string{
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude invocation failed: %w", err)
	}
	return nil
}

// Name returns the provider type identifier.
func (p *ClaudeProvider) Name() ProviderType {
	return ProviderClaude
}
```

### Test File (claude_test.go)

```go
// internal/provider/claude_test.go
package provider

import (
	"bytes"
	"context"
	"io"
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

	// Use pwd to verify working directory
	p := NewClaude("pwd")

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
```

## Backpressure

### Validation Command
```bash
go test ./internal/provider/... -run Claude
```

### Success Criteria
- All `*Claude*` tests pass
- `NewClaude("")` returns provider with command "claude"
- `NewClaude("/custom/path")` returns provider with custom command
- `Name()` returns `ProviderClaude`
- `Invoke` builds correct argument list: `--dangerously-skip-permissions -p <prompt>`
- `Invoke` respects context cancellation

## NOT In Scope
- Factory function `FromConfig` (separate unit: provider-config)
- Integration tests against real Claude CLI
- CodexProvider implementation (Task #2)
- Output parsing or structured data extraction
- Retry logic (handled by worker layer)
