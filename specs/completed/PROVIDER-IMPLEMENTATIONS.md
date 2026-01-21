# PROVIDER-IMPLEMENTATIONS — Claude and Codex CLI Subprocess Implementations

## Overview

The provider implementations module contains concrete implementations of the Provider interface for invoking AI coding assistants as subprocesses. Each implementation encapsulates the specific CLI invocation pattern for a particular tool while presenting a uniform interface to the worker system.

This module provides two implementations: ClaudeProvider for Claude CLI and CodexProvider for OpenAI's Codex CLI. Both follow the same pattern of spawning a subprocess with tool-specific arguments, connecting stdout/stderr for output capture, and running in a specified working directory. The implementations respect context cancellation for graceful shutdown during orchestration.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Provider Layer                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    Provider Interface                             │   │
│  │  Invoke(ctx, prompt, workdir, stdout, stderr) error               │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                              │                                           │
│              ┌───────────────┴───────────────┐                          │
│              ▼                               ▼                          │
│  ┌─────────────────────┐         ┌─────────────────────┐               │
│  │   ClaudeProvider    │         │   CodexProvider     │               │
│  │                     │         │                     │               │
│  │  claude             │         │  codex exec         │               │
│  │  --dangerously-     │         │  --yolo             │               │
│  │  skip-permissions   │         │  <prompt>           │               │
│  │  -p <prompt>        │         │                     │               │
│  └─────────────────────┘         └─────────────────────┘               │
│              │                               │                          │
│              └───────────────┬───────────────┘                          │
│                              ▼                                          │
│                    ┌─────────────────┐                                  │
│                    │   Subprocess    │                                  │
│                    │   exec.Command  │                                  │
│                    └─────────────────┘                                  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. ClaudeProvider invokes `claude --dangerously-skip-permissions -p <prompt>` as a subprocess
2. CodexProvider invokes `codex exec --yolo <prompt>` as a subprocess
3. Both providers set the working directory to the unit's worktree path
4. Both providers connect stdout and stderr to provided writers for output capture
5. Both providers respect context cancellation for graceful termination
6. Factory function creates the appropriate provider based on configuration
7. Provider command paths are configurable (not hardcoded to system PATH)
8. Subprocess errors are wrapped with provider context for debugging

### Performance Requirements

| Metric | Target |
|--------|--------|
| Subprocess spawn time | <50ms |
| Context cancellation response | <100ms |
| Memory overhead per provider | <1KB |

### Constraints

- Depends on PROVIDER-INTERFACE spec for the Provider interface definition
- Requires `claude` CLI installed for ClaudeProvider
- Requires `codex` CLI installed for CodexProvider
- Cannot capture structured output (stdout is raw text)
- Cannot control model parameters (MaxTurns, model selection) through the abstraction
- PR URL extraction and branch naming bypass this abstraction (remain Claude-specific)

## Design

### Module Structure

```
internal/provider/
├── provider.go     # Interface definition (see PROVIDER-INTERFACE spec)
├── claude.go       # ClaudeProvider implementation
├── codex.go        # CodexProvider implementation
└── factory.go      # FromConfig factory function
```

### Core Types

```go
// internal/provider/claude.go

// ClaudeProvider invokes Claude CLI as a subprocess.
// Uses --dangerously-skip-permissions to run without interactive prompts.
type ClaudeProvider struct {
    // command is the path to the claude executable.
    // Defaults to "claude" (resolved via PATH).
    command string
}

// NewClaudeProvider creates a provider that invokes Claude CLI.
func NewClaudeProvider(command string) *ClaudeProvider {
    if command == "" {
        command = "claude"
    }
    return &ClaudeProvider{command: command}
}
```

```go
// internal/provider/codex.go

// CodexProvider invokes Codex CLI as a subprocess.
// Uses exec subcommand with --yolo flag for autonomous execution.
type CodexProvider struct {
    // command is the path to the codex executable.
    // Defaults to "codex" (resolved via PATH).
    command string
}

// NewCodexProvider creates a provider that invokes Codex CLI.
func NewCodexProvider(command string) *CodexProvider {
    if command == "" {
        command = "codex"
    }
    return &CodexProvider{command: command}
}
```

```go
// internal/provider/factory.go

// ProviderType identifies which AI provider to use.
type ProviderType string

const (
    ProviderTypeClaude ProviderType = "claude"
    ProviderTypeCodex  ProviderType = "codex"
)

// ProviderConfig holds configuration for provider creation.
type ProviderConfig struct {
    // Type selects which provider implementation to use.
    Type ProviderType

    // Command overrides the default executable path.
    // If empty, uses the default ("claude" or "codex").
    Command string
}
```

### API Surface

```go
// internal/provider/claude.go

// Invoke executes Claude CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error
```

```go
// internal/provider/codex.go

// Invoke executes Codex CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error
```

```go
// internal/provider/factory.go

// FromConfig creates a Provider based on the given configuration.
// Returns an error if the provider type is unknown.
func FromConfig(cfg ProviderConfig) (Provider, error)
```

### Full Implementation

```go
// internal/provider/claude.go

package provider

import (
    "context"
    "fmt"
    "io"
    "os/exec"
)

type ClaudeProvider struct {
    command string
}

func NewClaudeProvider(command string) *ClaudeProvider {
    if command == "" {
        command = "claude"
    }
    return &ClaudeProvider{command: command}
}

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
```

```go
// internal/provider/codex.go

package provider

import (
    "context"
    "fmt"
    "io"
    "os/exec"
)

type CodexProvider struct {
    command string
}

func NewCodexProvider(command string) *CodexProvider {
    if command == "" {
        command = "codex"
    }
    return &CodexProvider{command: command}
}

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
```

```go
// internal/provider/factory.go

package provider

import "fmt"

type ProviderType string

const (
    ProviderTypeClaude ProviderType = "claude"
    ProviderTypeCodex  ProviderType = "codex"
)

type ProviderConfig struct {
    Type    ProviderType
    Command string
}

func FromConfig(cfg ProviderConfig) (Provider, error) {
    switch cfg.Type {
    case ProviderTypeClaude:
        return NewClaudeProvider(cfg.Command), nil
    case ProviderTypeCodex:
        return NewCodexProvider(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown provider type: %q", cfg.Type)
    }
}
```

## Implementation Notes

### Command Argument Patterns

The two CLIs have different argument structures:

| Provider | Command Pattern |
|----------|-----------------|
| Claude | `claude --dangerously-skip-permissions -p <prompt>` |
| Codex | `codex exec --yolo <prompt>` |

Note that Claude uses a flag (`-p`) before the prompt while Codex takes the prompt as a positional argument after `exec --yolo`.

### Context Cancellation Behavior

When the context is cancelled, `exec.CommandContext` sends SIGKILL to the subprocess on Unix systems. This is immediate termination without graceful shutdown. The calling code (worker) should handle this appropriately:

```go
err := provider.Invoke(ctx, prompt, workdir, stdout, stderr)
if ctx.Err() == context.Canceled {
    // Graceful shutdown, not a failure
    return nil
}
if err != nil {
    // Actual invocation failure
    return fmt.Errorf("provider error: %w", err)
}
```

### Working Directory Requirements

The `workdir` parameter should be an absolute path to the unit's worktree. The provider does not validate the path exists before invocation. Invalid paths result in an error from `cmd.Run()`.

### Output Capture Considerations

Both providers stream output directly to the provided writers. The current design has known limitations:

1. **No structured output parsing**: Output is raw text; parsing PR URLs or other structured data requires post-processing outside the Provider interface
2. **Interleaved stdout/stderr**: When both writers point to the same destination, output may interleave unpredictably
3. **No buffering**: Output streams directly without buffering; callers should buffer if needed

### Error Wrapping

Provider errors include context about which provider failed:

```go
// ClaudeProvider error:
"claude invocation failed: exit status 1"

// CodexProvider error:
"codex invocation failed: exit status 1"
```

This helps distinguish provider-specific failures when debugging.

### Security Considerations

Both providers use permissive flags:
- `--dangerously-skip-permissions` (Claude): Bypasses permission prompts
- `--yolo` (Codex): Runs without confirmation prompts

These flags are intentional for automated orchestration but mean the AI has full autonomy. The orchestrator should ensure:
1. Prompts are well-scoped to specific tasks
2. Worktrees are isolated from the main repository
3. Network access is appropriately restricted if needed

## Testing Strategy

### Unit Tests

```go
// internal/provider/claude_test.go

func TestClaudeProvider_NewWithDefault(t *testing.T) {
    p := NewClaudeProvider("")
    if p.command != "claude" {
        t.Errorf("expected default command 'claude', got %q", p.command)
    }
}

func TestClaudeProvider_NewWithCustomCommand(t *testing.T) {
    p := NewClaudeProvider("/usr/local/bin/claude")
    if p.command != "/usr/local/bin/claude" {
        t.Errorf("expected custom command, got %q", p.command)
    }
}

func TestClaudeProvider_Invoke_BuildsCorrectArgs(t *testing.T) {
    // Use a mock command that echoes its arguments
    p := NewClaudeProvider("echo")

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
    p := NewClaudeProvider("pwd")

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

    p := NewClaudeProvider("sleep")
    err := p.Invoke(ctx, "10", "/tmp", io.Discard, io.Discard)

    if err == nil {
        t.Error("expected error from cancelled context, got nil")
    }
}
```

```go
// internal/provider/codex_test.go

func TestCodexProvider_NewWithDefault(t *testing.T) {
    p := NewCodexProvider("")
    if p.command != "codex" {
        t.Errorf("expected default command 'codex', got %q", p.command)
    }
}

func TestCodexProvider_NewWithCustomCommand(t *testing.T) {
    p := NewCodexProvider("/opt/codex/bin/codex")
    if p.command != "/opt/codex/bin/codex" {
        t.Errorf("expected custom command, got %q", p.command)
    }
}

func TestCodexProvider_Invoke_BuildsCorrectArgs(t *testing.T) {
    p := NewCodexProvider("echo")

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
    tmpDir := t.TempDir()
    p := NewCodexProvider("pwd")

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
```

```go
// internal/provider/factory_test.go

func TestFromConfig_Claude(t *testing.T) {
    cfg := ProviderConfig{
        Type:    ProviderTypeClaude,
        Command: "/custom/claude",
    }

    p, err := FromConfig(cfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    claude, ok := p.(*ClaudeProvider)
    if !ok {
        t.Fatalf("expected *ClaudeProvider, got %T", p)
    }
    if claude.command != "/custom/claude" {
        t.Errorf("command = %q, want %q", claude.command, "/custom/claude")
    }
}

func TestFromConfig_Codex(t *testing.T) {
    cfg := ProviderConfig{
        Type:    ProviderTypeCodex,
        Command: "",
    }

    p, err := FromConfig(cfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    codex, ok := p.(*CodexProvider)
    if !ok {
        t.Fatalf("expected *CodexProvider, got %T", p)
    }
    if codex.command != "codex" {
        t.Errorf("command = %q, want default 'codex'", codex.command)
    }
}

func TestFromConfig_UnknownType(t *testing.T) {
    cfg := ProviderConfig{
        Type: "unknown",
    }

    _, err := FromConfig(cfg)
    if err == nil {
        t.Error("expected error for unknown provider type")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Claude provider invokes real CLI | Requires claude CLI installed; verify --version works |
| Codex provider invokes real CLI | Requires codex CLI installed; verify help output |
| Context cancellation terminates subprocess | Start long-running command, cancel context, verify termination |
| Output capture works correctly | Invoke command with known output, verify capture |
| Working directory isolation | Create isolated directory, verify command runs there |

### Manual Testing

- [ ] `ClaudeProvider.Invoke` successfully runs claude with a simple prompt
- [ ] `CodexProvider.Invoke` successfully runs codex with a simple prompt
- [ ] Cancelling context during invocation terminates the subprocess
- [ ] stdout and stderr are correctly captured to separate writers
- [ ] Invalid command path returns descriptive error
- [ ] Invalid working directory returns descriptive error

## Design Decisions

### Why Subprocess Invocation Instead of Library Integration?

Both Claude CLI and Codex CLI are designed as standalone command-line tools without stable Go library APIs. Subprocess invocation:
- Works with any CLI version without code changes
- Avoids tight coupling to internal APIs
- Allows drop-in replacement with wrapper scripts
- Matches how developers use these tools manually

Trade-off: Higher overhead than library calls, no structured output parsing.

### Why Configurable Command Paths?

Hardcoded command names would require the CLIs to be in PATH. Configurable paths support:
- Custom installation locations
- Version-specific binaries (e.g., `/opt/claude/v2/claude`)
- Wrapper scripts for logging or environment setup
- Testing with mock commands

### Why Separate Types Instead of a Generic SubprocessProvider?

A generic provider could accept command and args as configuration:

```go
// Alternative: generic approach
type SubprocessProvider struct {
    command string
    args    []string
    // How to interpolate prompt?
}
```

Problems with the generic approach:
- Each CLI has different argument patterns
- Prompt interpolation varies (flag vs positional)
- Future providers may need provider-specific logic
- Type safety helps catch configuration errors

The concrete types are more explicit and easier to test.

### Why Simple Error Wrapping?

The current implementation wraps errors with provider name only. More detailed error handling could include:
- Exit code extraction
- stderr capture in error
- Timeout detection

These are deferred because:
- The worker layer handles retry logic
- Exit codes vary by provider and aren't standardized
- Simple errors are sufficient for the MVP

## Future Enhancements

1. **Output parsing hooks**: Allow providers to register output parsers for extracting structured data (PR URLs, branch names)
2. **Model parameter support**: Extend the interface to accept model configuration (MaxTurns, temperature)
3. **Timeout configuration**: Add per-provider timeout settings with graceful shutdown
4. **Retry integration**: Built-in retry logic for transient failures
5. **Additional providers**: Support for other AI coding tools (Cursor, Aider, etc.)
6. **Streaming callback**: Support for real-time output processing during invocation

## References

- [PROVIDER-INTERFACE spec](PROVIDER-INTERFACE.md) - Interface definition this module implements
- [WORKER spec](completed/WORKER.md) - Consumer of provider implementations
- [CONFIG spec](completed/CONFIG.md) - Configuration that feeds into FromConfig
- [PRD Multi-Provider Section](../docs/MULTI-PROVIDER-PRD.md) - Original requirements
