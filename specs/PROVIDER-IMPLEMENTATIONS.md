# PROVIDER-IMPLEMENTATIONS — Claude and Codex CLI Subprocess Implementations

## Overview

The PROVIDER-IMPLEMENTATIONS spec covers the concrete implementations of the Provider interface for Claude CLI and OpenAI Codex CLI. Both providers execute as subprocesses, passing prompts via stdin and capturing output streams.

This spec focuses on the subprocess invocation details, command-line arguments, and error handling for each provider. The interface definition and factory pattern are covered in PROVIDER-INTERFACE.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Provider Implementations                                                    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        Provider Interface                            │    │
│  │  Invoke(ctx, prompt, workdir, stdout, stderr) error                 │    │
│  │  Name() ProviderType                                                 │    │
│  └───────────────────────────┬─────────────────────────────────────────┘    │
│                              │                                               │
│            ┌─────────────────┼─────────────────┐                            │
│            ▼                                   ▼                            │
│  ┌─────────────────────┐             ┌─────────────────────┐                │
│  │   ClaudeProvider    │             │   CodexProvider     │                │
│  ├─────────────────────┤             ├─────────────────────┤                │
│  │ command: string     │             │ command: string     │                │
│  ├─────────────────────┤             ├─────────────────────┤                │
│  │ claude --print      │             │ codex --quiet       │                │
│  │ prompt via stdin    │             │ prompt via stdin    │                │
│  │ workdir via --cwd   │             │ workdir via cd      │                │
│  └─────────────────────┘             └─────────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. ClaudeProvider invokes `claude --print` with prompt on stdin
2. CodexProvider invokes `codex --quiet` with prompt on stdin
3. Both providers set working directory for subprocess execution
4. Both providers stream stdout/stderr to provided writers
5. Both providers respect context cancellation
6. Both providers wrap errors with provider name for debugging
7. Both providers support custom command paths
8. Default command is "claude" for ClaudeProvider, "codex" for CodexProvider

### Performance Requirements

| Metric | Target |
|--------|--------|
| Command spawn latency | <50ms |
| Stream buffer size | 64KB minimum |
| Context cancellation response | <100ms |
| Memory per invocation | <1MB overhead |

### Constraints

- Both CLIs must be installed and available in PATH (or custom path specified)
- Providers are stateless; each Invoke is independent
- Subprocess inherits environment from parent process
- Depends on PROVIDER-INTERFACE spec for interface definition

## Design

### Module Structure

```
internal/provider/
├── provider.go    # Interface (from PROVIDER-INTERFACE)
├── factory.go     # Factory (from PROVIDER-INTERFACE)
├── claude.go      # ClaudeProvider implementation ← THIS SPEC
└── codex.go       # CodexProvider implementation  ← THIS SPEC
```

### Core Types

```go
// internal/provider/claude.go
package provider

import (
    "context"
    "fmt"
    "io"
    "os/exec"
    "strings"
)

// ClaudeProvider implements Provider using the Claude CLI
type ClaudeProvider struct {
    // command is the path to the claude executable
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
```

```go
// internal/provider/codex.go
package provider

import (
    "context"
    "fmt"
    "io"
    "os/exec"
    "strings"
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
```

### API Surface

```go
// internal/provider/claude.go

// NewClaude creates a Claude provider with the specified command path
func NewClaude(command string) *ClaudeProvider

// Invoke executes the Claude CLI with the given prompt
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error

// Name returns ProviderClaude
func (p *ClaudeProvider) Name() ProviderType
```

```go
// internal/provider/codex.go

// NewCodex creates a Codex provider with the specified command path
func NewCodex(command string) *CodexProvider

// Invoke executes the Codex CLI with the given prompt
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error

// Name returns ProviderCodex
func (p *CodexProvider) Name() ProviderType
```

## Implementation Notes

### Claude Provider Implementation

The Claude CLI uses `--print` for non-interactive output and `--dangerously-skip-permissions` to bypass permission prompts in automated workflows:

```go
// internal/provider/claude.go

func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Build command with flags for automated execution
    // --print: Output to stdout instead of interactive mode
    // --dangerously-skip-permissions: Skip permission prompts (required for automation)
    cmd := exec.CommandContext(ctx, p.command,
        "--dangerously-skip-permissions",
        "--print",
        "-p", prompt,
    )
    cmd.Dir = workdir
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    if err := cmd.Run(); err != nil {
        // Check if context was cancelled
        if ctx.Err() != nil {
            return fmt.Errorf("claude invocation cancelled: %w", ctx.Err())
        }
        // Wrap error with provider name for debugging
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return fmt.Errorf("claude exited with code %d: %w", exitErr.ExitCode(), err)
        }
        return fmt.Errorf("claude invocation failed: %w", err)
    }

    return nil
}

func (p *ClaudeProvider) Name() ProviderType {
    return ProviderClaude
}
```

### Codex Provider Implementation

The Codex CLI uses `--quiet` for non-interactive mode and `--full-auto` for autonomous operation:

```go
// internal/provider/codex.go

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
        if ctx.Err() != nil {
            return fmt.Errorf("codex invocation cancelled: %w", ctx.Err())
        }
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return fmt.Errorf("codex exited with code %d: %w", exitErr.ExitCode(), err)
        }
        return fmt.Errorf("codex invocation failed: %w", err)
    }

    return nil
}

func (p *CodexProvider) Name() ProviderType {
    return ProviderCodex
}
```

### Context Cancellation

Both providers use `exec.CommandContext` which sends SIGKILL on context cancellation. The subprocess is terminated immediately when the context is cancelled:

```go
// Example: graceful shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

err := provider.Invoke(ctx, prompt, workdir, os.Stdout, os.Stderr)
if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
    log.Println("Provider invocation was cancelled")
}
```

### Error Handling

Both implementations follow the same error handling pattern:

1. Check for context cancellation first (user-initiated abort)
2. Extract exit code from ExitError if available
3. Wrap all errors with provider name for debugging

```go
// Error checking in caller code
err := provider.Invoke(ctx, prompt, workdir, stdout, stderr)
if err != nil {
    if strings.Contains(err.Error(), "claude") {
        // Handle Claude-specific error
    } else if strings.Contains(err.Error(), "codex") {
        // Handle Codex-specific error
    }
}
```

## Testing Strategy

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

func TestClaudeProvider_Invoke_CommandNotFound(t *testing.T) {
    p := NewClaude("/nonexistent/claude")

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

func TestCodexProvider_Invoke_CommandNotFound(t *testing.T) {
    p := NewCodex("/nonexistent/codex")

    var stdout, stderr bytes.Buffer
    err := p.Invoke(context.Background(), "test", "/tmp", &stdout, &stderr)

    if err == nil {
        t.Error("expected error for nonexistent command")
    }
}
```

### Integration Tests

| Scenario | Setup | Expected Behavior |
|----------|-------|-------------------|
| Claude invocation | Install Claude CLI | Prompt executed, output captured |
| Codex invocation | Install Codex CLI | Prompt executed, output captured |
| Output streaming | Long-running prompt | stdout/stderr stream in real-time |
| Context cancellation | Cancel during execution | Subprocess terminated within 100ms |
| Working directory | Set workdir parameter | Subprocess runs in specified directory |
| Exit code propagation | Provider returns non-zero | Error includes exit code |

### Manual Testing

- [ ] `NewClaude("")` creates provider with command "claude"
- [ ] `NewCodex("")` creates provider with command "codex"
- [ ] Claude provider invokes `claude --dangerously-skip-permissions --print -p <prompt>`
- [ ] Codex provider invokes `codex --quiet --full-auto <prompt>`
- [ ] Output streams to provided writers in real-time
- [ ] Working directory is set correctly for subprocess
- [ ] Context cancellation terminates subprocess
- [ ] Error messages include provider name

## Design Decisions

### Why Different Invocation Patterns?

Each CLI has its own preferred way of receiving prompts and operating non-interactively:

- **Claude**: Uses `--print` for stdout output and `-p` flag for prompt. The `--dangerously-skip-permissions` flag is required to bypass interactive permission prompts.

- **Codex**: Uses `--quiet` to suppress interactive UI and `--full-auto` for autonomous operation. Prompt is passed as a positional argument.

These patterns match each CLI's documented usage for automation.

### Why Not Use Stdin for Both?

While both CLIs support stdin, using their native argument patterns:
- Is more explicit in logs and process listings
- Avoids potential stdin buffering issues
- Matches documentation examples

### Why exec.CommandContext Instead of Manual Process Management?

`exec.CommandContext` provides:
- Automatic SIGKILL on context cancellation
- Clean process cleanup
- Standard Go patterns for subprocess management

Manual process management would add complexity without benefits for this use case.

### Why Wrap Errors with Provider Name?

When multiple providers are in use, error messages need to identify which provider failed. Wrapping with provider name enables:
- Quick identification in logs
- Provider-specific error handling if needed
- Better debugging experience

## Future Enhancements

1. **Configurable CLI flags**: Allow per-provider flag customization
2. **Environment variable injection**: Pass additional env vars to subprocess
3. **Output parsing**: Parse structured output from providers
4. **Retry logic**: Automatic retry on transient failures
5. **Health check**: Verify CLI availability before invocation

## References

- [PROVIDER-INTERFACE spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/PROVIDER-INTERFACE.md) - Interface definition
- [Multi-Provider PRD](/Users/bennett/conductor/workspaces/choo/san-jose/docs/MULTI-PROVIDER-PRD.md) - Product requirements
- [Claude CLI Documentation](https://docs.anthropic.com/claude-code/cli)
- [OpenAI Codex CLI](https://github.com/openai/codex-cli)
