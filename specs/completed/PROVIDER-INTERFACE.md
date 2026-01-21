# PROVIDER-INTERFACE — Provider Interface and Factory Pattern for CLI-based LLM Providers

## Overview

The Provider Interface defines an abstraction layer for multi-provider support in choo, enabling different CLI-based LLM tools to execute tasks within the ralph inner loops. The primary motivation is to add OpenAI Codex CLI as an alternative to Claude CLI for task execution.

Both providers are invoked as CLI subprocesses rather than direct API calls. This keeps the architecture simple: choo orchestrates work and manages git operations, while the provider handles the actual code generation. The interface-based design with a factory pattern allows swapping providers transparently.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Provider Interface                                                          │
│                                                                              │
│  internal/provider/                                                          │
│  ├── provider.go     ──► Interface definition + Config type                 │
│  ├── factory.go      ──► FromConfig(cfg) (Provider, error)                  │
│  ├── claude.go       ──► Claude CLI implementation                          │
│  └── codex.go        ──► Codex CLI implementation                           │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        Provider Interface                            │    │
│  │  Invoke(ctx, prompt, workdir, stdout, stderr) error                 │    │
│  │  Name() ProviderType                                                 │    │
│  └───────────────────────────┬─────────────────────────────────────────┘    │
│                              │                                               │
│            ┌─────────────────┼─────────────────┐                            │
│            ▼                                   ▼                            │
│  ┌─────────────────┐                 ┌─────────────────┐                    │
│  │   ClaudeProvider │                 │   CodexProvider │                    │
│  │   (subprocess)   │                 │   (subprocess)   │                    │
│  └─────────────────┘                 └─────────────────┘                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Define a Provider interface with `Invoke()` and `Name()` methods
2. Support Claude CLI as the default provider (backward compatible)
3. Support Codex CLI as an alternative provider
4. Implement factory function `FromConfig()` that instantiates providers from configuration
5. Allow empty provider type to default to Claude (backward compatibility)
6. Pass prompts to providers via stdin or command-line arguments
7. Capture provider stdout/stderr and stream to provided writers
8. Support configurable command paths for both providers
9. Respect context cancellation for graceful shutdown
10. Return structured errors for provider failures

### Performance Requirements

| Metric | Target |
|--------|--------|
| Provider instantiation | <1ms |
| Command spawn latency | <50ms |
| Stream buffer size | 64KB minimum |
| Context cancellation response | <100ms |

### Constraints

- Both providers are CLI tools invoked as subprocesses (no direct API)
- Provider selection is determined at unit or config level, not per-task
- Providers must support working directory specification
- Providers must accept prompts via stdin
- Must work on macOS and Linux

## Design

### Module Structure

```
internal/provider/
├── provider.go    # Interface definition and ProviderType constants
├── factory.go     # FromConfig factory function
├── claude.go      # Claude CLI provider implementation
└── codex.go       # Codex CLI provider implementation
```

### Core Types

```go
// internal/provider/provider.go
package provider

import (
    "context"
    "io"
)

// ProviderType identifies which LLM provider to use
type ProviderType string

const (
    // ProviderClaude uses the Claude CLI (default)
    ProviderClaude ProviderType = "claude"

    // ProviderCodex uses the OpenAI Codex CLI
    ProviderCodex ProviderType = "codex"
)

// Provider defines the interface for CLI-based LLM providers
type Provider interface {
    // Invoke executes the provider with the given prompt in the specified workdir.
    // Output is streamed to stdout and stderr writers.
    // Returns an error if the provider fails or context is cancelled.
    Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error

    // Name returns the provider type identifier
    Name() ProviderType
}

// Config holds provider configuration
type Config struct {
    // Type specifies which provider to use (defaults to "claude" if empty)
    Type ProviderType

    // Command is the path to the provider CLI executable.
    // If empty, uses the default command name ("claude" or "codex").
    Command string
}
```

```go
// internal/provider/claude.go
package provider

import (
    "context"
    "io"
    "os/exec"
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
```

### API Surface

```go
// internal/provider/provider.go

// Provider interface (see Core Types above)
type Provider interface {
    Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error
    Name() ProviderType
}
```

```go
// internal/provider/factory.go

// FromConfig creates a Provider from the given configuration.
// If cfg.Type is empty, defaults to Claude for backward compatibility.
// Returns an error for unknown provider types.
func FromConfig(cfg Config) (Provider, error)
```

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

### Factory Implementation

The factory function handles defaulting and validation:

```go
// internal/provider/factory.go
package provider

import "fmt"

// FromConfig creates a Provider from the given configuration.
func FromConfig(cfg Config) (Provider, error) {
    switch cfg.Type {
    case ProviderClaude, "":
        // Empty type defaults to Claude for backward compatibility
        return NewClaude(cfg.Command), nil
    case ProviderCodex:
        return NewCodex(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
    }
}
```

### Claude Provider Implementation

The Claude CLI is invoked with the prompt passed via stdin:

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

func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Build command with print mode for non-interactive execution
    cmd := exec.CommandContext(ctx, p.command, "--print")
    cmd.Dir = workdir
    cmd.Stdin = strings.NewReader(prompt)
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    if err := cmd.Run(); err != nil {
        // Check if context was cancelled
        if ctx.Err() != nil {
            return fmt.Errorf("claude invocation cancelled: %w", ctx.Err())
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

The Codex CLI has a different invocation pattern:

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

func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Codex uses --quiet for non-interactive mode and reads prompt from stdin
    cmd := exec.CommandContext(ctx, p.command, "--quiet")
    cmd.Dir = workdir
    cmd.Stdin = strings.NewReader(prompt)
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    if err := cmd.Run(); err != nil {
        if ctx.Err() != nil {
            return fmt.Errorf("codex invocation cancelled: %w", ctx.Err())
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

Both providers use `exec.CommandContext` which sends SIGKILL on context cancellation. For graceful shutdown, the caller should:

1. Cancel the context
2. Allow a brief grace period for cleanup
3. The subprocess will be terminated if it doesn't exit

```go
// Example usage with graceful shutdown
ctx, cancel := context.WithCancel(context.Background())

// Start provider in goroutine
errCh := make(chan error, 1)
go func() {
    errCh <- provider.Invoke(ctx, prompt, workdir, os.Stdout, os.Stderr)
}()

// On shutdown signal
cancel()

// Wait for completion or timeout
select {
case err := <-errCh:
    // Provider finished (possibly with cancellation error)
case <-time.After(5 * time.Second):
    // Force killed
}
```

### Error Handling

Provider errors are wrapped with context about which provider failed:

```go
// Error types that callers may check
var (
    ErrUnknownProvider = errors.New("unknown provider type")
    ErrProviderNotFound = errors.New("provider executable not found")
)

// Wrapping pattern in Invoke methods
if err := cmd.Run(); err != nil {
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return fmt.Errorf("%s exited with code %d: %w",
            p.Name(), exitErr.ExitCode(), err)
    }
    return fmt.Errorf("%s invocation failed: %w", p.Name(), err)
}
```

### Integration with Worker

The worker package uses the provider interface:

```go
// internal/worker/worker.go (relevant excerpt)
type Worker struct {
    provider provider.Provider
    // ... other fields
}

func (w *Worker) ExecuteTask(ctx context.Context, task *discovery.Task) error {
    prompt := w.buildPrompt(task)

    var stdout, stderr bytes.Buffer
    if err := w.provider.Invoke(ctx, prompt, w.worktree, &stdout, &stderr); err != nil {
        return fmt.Errorf("task %s failed: %w", task.ID, err)
    }

    // Process output...
    return nil
}
```

## Testing Strategy

### Unit Tests

```go
// internal/provider/factory_test.go
package provider

import (
    "testing"
)

func TestFromConfig(t *testing.T) {
    tests := []struct {
        name     string
        cfg      Config
        wantType ProviderType
        wantErr  bool
    }{
        {
            name:     "claude explicit",
            cfg:      Config{Type: ProviderClaude},
            wantType: ProviderClaude,
            wantErr:  false,
        },
        {
            name:     "claude default (empty type)",
            cfg:      Config{Type: ""},
            wantType: ProviderClaude,
            wantErr:  false,
        },
        {
            name:     "codex explicit",
            cfg:      Config{Type: ProviderCodex},
            wantType: ProviderCodex,
            wantErr:  false,
        },
        {
            name:     "custom command path",
            cfg:      Config{Type: ProviderClaude, Command: "/usr/local/bin/claude"},
            wantType: ProviderClaude,
            wantErr:  false,
        },
        {
            name:    "unknown provider",
            cfg:     Config{Type: "gpt4"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p, err := FromConfig(tt.cfg)
            if (err != nil) != tt.wantErr {
                t.Errorf("FromConfig() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && p.Name() != tt.wantType {
                t.Errorf("FromConfig() provider type = %v, want %v", p.Name(), tt.wantType)
            }
        })
    }
}
```

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
    // This test uses a mock command that sleeps
    p := NewClaude("sleep") // sleep command as stand-in

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    var stdout, stderr bytes.Buffer
    err := p.Invoke(ctx, "10", "/tmp", &stdout, &stderr) // sleep 10 would take 10s

    if err == nil {
        t.Error("expected error due to context cancellation")
    }
}
```

```go
// internal/provider/codex_test.go
package provider

import (
    "testing"
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
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Claude provider invocation | Install claude CLI, verify prompt execution and output capture |
| Codex provider invocation | Install codex CLI, verify prompt execution and output capture |
| Provider switching | Configure both providers, verify correct CLI is invoked |
| Context cancellation | Start long-running prompt, cancel context, verify subprocess terminated |
| Error propagation | Invoke with invalid prompt, verify error contains provider name |

### Manual Testing

- [ ] `FromConfig({})` returns Claude provider (backward compatibility)
- [ ] Claude provider invokes `claude --print` with prompt on stdin
- [ ] Codex provider invokes `codex --quiet` with prompt on stdin
- [ ] Output streams to provided stdout/stderr writers
- [ ] Context cancellation terminates subprocess within 100ms
- [ ] Custom command paths are respected
- [ ] Unknown provider type returns descriptive error

## Design Decisions

### Why Interface-Based Design?

The interface pattern allows:
- Adding new providers without modifying existing code
- Unit testing with mock providers
- Runtime provider selection based on configuration
- Clear contract for what providers must implement

Alternative considered: Function callbacks. Rejected because interfaces provide better type safety and clearer API boundaries.

### Why Subprocess Execution Instead of Direct API?

Both Claude and Codex provide CLI tools that handle authentication, rate limiting, and session management. Using subprocesses:
- Leverages existing CLI infrastructure
- Avoids duplicating auth token management
- Allows users to configure CLIs independently
- Simplifies error handling (just check exit codes)

Alternative considered: Direct API calls. Rejected because it would require managing API keys, rate limits, and complex response parsing that the CLIs already handle.

### Why Default to Claude?

Claude is the current provider used by ralph. Defaulting to Claude when `provider.type` is empty ensures:
- Existing configurations continue working
- No migration required for current users
- Explicit opt-in for alternative providers

### Why Pass Prompt via Stdin?

Both CLIs support stdin for prompts, which:
- Avoids shell escaping issues with complex prompts
- Supports prompts of any length (no argument limit)
- Keeps command-line arguments simple

Alternative considered: Command-line argument with `--prompt`. Rejected due to shell escaping complexity and length limits.

## Future Enhancements

1. **Provider health checks**: Add `Ping()` method to verify provider availability before task execution
2. **Streaming output parsing**: Parse provider output in real-time for progress updates
3. **Provider metrics**: Track invocation counts, latencies, and success rates per provider
4. **Provider fallback**: Automatically retry with alternative provider on failure
5. **Additional providers**: Support for other CLI tools (e.g., Cursor, Aider)
6. **Provider configuration per task**: Allow overriding provider at task level for specialized use cases

## References

- [Multi-Provider PRD](/Users/bennett/conductor/workspaces/choo/san-jose/docs/MULTI-PROVIDER-PRD.md)
- [Worker Spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/completed/WORKER.md)
- [Config Spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/completed/CONFIG.md)
- [Go exec package](https://pkg.go.dev/os/exec)
