---
task: 2
status: complete
backpressure: "go test ./internal/provider/... -run FromConfig"
depends_on: [1]
---

# Factory Function

**Parent spec**: `/specs/PROVIDER-INTERFACE.md`
**Task**: #2 of 2 in implementation plan

## Objective

Implement the FromConfig factory function that creates a Provider from configuration, including stub provider implementations for Claude and Codex to enable testing.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Core types and interface

### Package Dependencies
- `fmt` (standard library)
- `context` (standard library)
- `io` (standard library)
- `testing` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/provider/
├── factory.go       # CREATE: Factory function
├── factory_test.go  # CREATE: Factory tests
├── claude.go        # CREATE: Claude provider stub (Name only, Invoke placeholder)
└── codex.go         # CREATE: Codex provider stub (Name only, Invoke placeholder)
```

### Types to Implement

```go
// internal/provider/factory.go

package provider

import "fmt"

// FromConfig creates a Provider from the given configuration.
// If cfg.Type is empty, defaults to Claude for backward compatibility.
// Returns an error for unknown provider types.
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

```go
// internal/provider/claude.go

package provider

import (
    "context"
    "io"
)

// ClaudeProvider implements Provider using the Claude CLI
type ClaudeProvider struct {
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

// Invoke executes the Claude CLI with the given prompt.
// NOTE: Full implementation deferred to provider-implementations unit.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Placeholder - full implementation in provider-implementations unit
    return nil
}

// Name returns ProviderClaude
func (p *ClaudeProvider) Name() ProviderType {
    return ProviderClaude
}
```

```go
// internal/provider/codex.go

package provider

import (
    "context"
    "io"
)

// CodexProvider implements Provider using the OpenAI Codex CLI
type CodexProvider struct {
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
// NOTE: Full implementation deferred to provider-implementations unit.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
    // Placeholder - full implementation in provider-implementations unit
    return nil
}

// Name returns ProviderCodex
func (p *CodexProvider) Name() ProviderType {
    return ProviderCodex
}
```

### Tests to Implement

```go
// internal/provider/factory_test.go

package provider

import (
    "testing"
)

func TestFromConfig_ClaudeExplicit(t *testing.T) {
    p, err := FromConfig(Config{Type: ProviderClaude})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if p.Name() != ProviderClaude {
        t.Errorf("expected claude, got %q", p.Name())
    }
}

func TestFromConfig_ClaudeDefault(t *testing.T) {
    p, err := FromConfig(Config{Type: ""})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if p.Name() != ProviderClaude {
        t.Errorf("expected claude as default, got %q", p.Name())
    }
}

func TestFromConfig_CodexExplicit(t *testing.T) {
    p, err := FromConfig(Config{Type: ProviderCodex})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if p.Name() != ProviderCodex {
        t.Errorf("expected codex, got %q", p.Name())
    }
}

func TestFromConfig_CustomCommand(t *testing.T) {
    p, err := FromConfig(Config{Type: ProviderClaude, Command: "/usr/local/bin/claude"})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if p.Name() != ProviderClaude {
        t.Errorf("expected claude, got %q", p.Name())
    }
}

func TestFromConfig_UnknownProvider(t *testing.T) {
    _, err := FromConfig(Config{Type: "gpt4"})
    if err == nil {
        t.Error("expected error for unknown provider type")
    }
}

func TestFromConfig_EmptyConfig(t *testing.T) {
    p, err := FromConfig(Config{})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if p.Name() != ProviderClaude {
        t.Errorf("expected claude as default for empty config, got %q", p.Name())
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/provider/... -run FromConfig
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestFromConfig_ClaudeExplicit | Returns Claude provider when type is "claude" |
| TestFromConfig_ClaudeDefault | Returns Claude provider when type is empty |
| TestFromConfig_CodexExplicit | Returns Codex provider when type is "codex" |
| TestFromConfig_CustomCommand | Accepts custom command path |
| TestFromConfig_UnknownProvider | Returns error for unknown provider type |
| TestFromConfig_EmptyConfig | Returns Claude provider for empty config |

## NOT In Scope
- Full Invoke implementation (deferred to provider-implementations unit)
- Context cancellation handling in Invoke
- Subprocess execution logic
- Integration with Worker package
