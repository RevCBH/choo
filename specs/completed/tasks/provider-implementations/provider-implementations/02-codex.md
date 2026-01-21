---
task: 2
status: complete
backpressure: "go test ./internal/provider/... -run Codex"
depends_on: []
---

# Codex Provider Implementation

**Parent spec**: `/specs/PROVIDER-IMPLEMENTATIONS.md`
**Task**: #2 of 2 in implementation plan

## Objective

Implement CodexProvider struct with Invoke and Name methods that invoke the OpenAI Codex CLI as a subprocess. Also add unit tests for both providers.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 (01-claude.md): ClaudeProvider must exist for test patterns

### External Spec Dependencies
- `provider-interface`: Provider interface and ProviderType constants

### Package Dependencies
- `context` (standard library)
- `fmt` (standard library)
- `io` (standard library)
- `os/exec` (standard library)
- `errors` (standard library)
- `testing` (standard library)
- `bytes` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/provider/
├── codex.go         # CREATE: CodexProvider implementation
├── claude_test.go   # CREATE: ClaudeProvider unit tests
└── codex_test.go    # CREATE: CodexProvider unit tests
```

### Types to Implement

```go
// internal/provider/codex.go
package provider

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os/exec"
)

// CodexProvider implements Provider using the OpenAI Codex CLI
type CodexProvider struct {
    // command is the path to the codex executable
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

// Invoke executes the Codex CLI with the given prompt.
// Uses --quiet and --full-auto for automated execution.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Build command with flags for automated execution
    // --quiet: Suppress interactive prompts
    // --full-auto: Enable autonomous operation without confirmations
    cmd := exec.CommandContext(ctx, p.command,
        "--quiet",
        "--full-auto",
        prompt,
    )
    cmd.Dir = workdir
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    if err := cmd.Run(); err != nil {
        // Check if context was cancelled
        if ctx.Err() != nil {
            return fmt.Errorf("codex invocation cancelled: %w", ctx.Err())
        }
        // Wrap error with provider name and exit code if available
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return fmt.Errorf("codex exited with code %d: %w", exitErr.ExitCode(), err)
        }
        return fmt.Errorf("codex invocation failed: %w", err)
    }

    return nil
}

// Name returns the provider type identifier
func (p *CodexProvider) Name() ProviderType {
    return ProviderCodex
}
```

### Unit Tests

```go
// internal/provider/claude_test.go
package provider

import (
    "bytes"
    "context"
    "testing"
    "time"
)

func TestClaudeProvider_Name(t *testing.T) {
    p := NewClaude("")
    if got := p.Name(); got != ProviderClaude {
        t.Errorf("Name() = %v, want %v", got, ProviderClaude)
    }
}

func TestClaudeProvider_DefaultCommand(t *testing.T) {
    p := NewClaude("")
    if p.command != "claude" {
        t.Errorf("default command = %v, want 'claude'", p.command)
    }
}

func TestClaudeProvider_CustomCommand(t *testing.T) {
    p := NewClaude("/opt/claude/bin/claude")
    if p.command != "/opt/claude/bin/claude" {
        t.Errorf("custom command = %v, want '/opt/claude/bin/claude'", p.command)
    }
}

func TestClaudeProvider_ContextCancellation(t *testing.T) {
    // Use sleep as a stand-in for a long-running command
    p := NewClaude("sleep")

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    var stdout, stderr bytes.Buffer
    err := p.Invoke(ctx, "10", "/tmp", &stdout, &stderr)

    if err == nil {
        t.Error("expected error due to context cancellation")
    }
}

func TestClaudeProvider_CommandNotFound(t *testing.T) {
    p := NewClaude("/nonexistent/path/to/claude")

    var stdout, stderr bytes.Buffer
    err := p.Invoke(context.Background(), "test", "/tmp", &stdout, &stderr)

    if err == nil {
        t.Error("expected error for nonexistent command")
    }
}
```

```go
// internal/provider/codex_test.go
package provider

import (
    "bytes"
    "context"
    "testing"
    "time"
)

func TestCodexProvider_Name(t *testing.T) {
    p := NewCodex("")
    if got := p.Name(); got != ProviderCodex {
        t.Errorf("Name() = %v, want %v", got, ProviderCodex)
    }
}

func TestCodexProvider_DefaultCommand(t *testing.T) {
    p := NewCodex("")
    if p.command != "codex" {
        t.Errorf("default command = %v, want 'codex'", p.command)
    }
}

func TestCodexProvider_CustomCommand(t *testing.T) {
    p := NewCodex("/usr/local/bin/codex")
    if p.command != "/usr/local/bin/codex" {
        t.Errorf("custom command = %v, want '/usr/local/bin/codex'", p.command)
    }
}

func TestCodexProvider_ContextCancellation(t *testing.T) {
    p := NewCodex("sleep")

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    var stdout, stderr bytes.Buffer
    err := p.Invoke(ctx, "10", "/tmp", &stdout, &stderr)

    if err == nil {
        t.Error("expected error due to context cancellation")
    }
}

func TestCodexProvider_CommandNotFound(t *testing.T) {
    p := NewCodex("/nonexistent/path/to/codex")

    var stdout, stderr bytes.Buffer
    err := p.Invoke(context.Background(), "test", "/tmp", &stdout, &stderr)

    if err == nil {
        t.Error("expected error for nonexistent command")
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/provider/... -run Provider
```

### Must Pass
| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| CodexProvider compiles | Struct and methods defined |
| Implements Provider | Interface satisfaction verified by compiler |
| TestClaudeProvider_Name | Claude provider returns ProviderClaude |
| TestClaudeProvider_DefaultCommand | Empty command defaults to "claude" |
| TestClaudeProvider_CustomCommand | Custom path is preserved |
| TestCodexProvider_Name | Codex provider returns ProviderCodex |
| TestCodexProvider_DefaultCommand | Empty command defaults to "codex" |
| TestCodexProvider_CustomCommand | Custom path is preserved |

### CI Compatibility
- [ ] `go build ./internal/provider/...` exits 0
- [ ] `go test ./internal/provider/... -run Provider` exits 0
- [ ] `go vet ./internal/provider/...` reports no issues

## Implementation Notes

### Codex CLI Flags

The Codex CLI uses different flags than Claude:

- `--quiet`: Suppresses the interactive UI and prompts
- `--full-auto`: Enables autonomous operation, applying changes without confirmation
- Prompt is passed as a positional argument (not via `-p` flag)

### Error Wrapping

All errors must be wrapped with "codex" prefix for identification:
- Cancellation: `"codex invocation cancelled: ..."`
- Exit error: `"codex exited with code N: ..."`
- Other: `"codex invocation failed: ..."`

### Test Strategy

Tests use `sleep` command as a stand-in for provider CLIs to test:
- Context cancellation behavior
- Error handling for missing commands

This avoids requiring actual Claude/Codex CLIs to be installed for unit tests.

## NOT In Scope
- Integration tests requiring Codex CLI installed
- Custom flag configuration per invocation
- Retry logic on transient failures
- Factory function (covered in PROVIDER-INTERFACE)
