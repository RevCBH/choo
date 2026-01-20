---
task: 1
status: pending
backpressure: "go build ./internal/provider/..."
depends_on: []
---

# Core Types and Interface

**Parent spec**: `/specs/PROVIDER-INTERFACE.md`
**Task**: #1 of 2 in implementation plan

## Objective

Define the core Provider interface, ProviderType constants, and Config struct that all provider implementations will use.

## Dependencies

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `context` (standard library)
- `io` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/provider/
└── provider.go    # CREATE: Interface and types
```

### Types to Implement

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

## Backpressure

### Validation Command
```bash
go build ./internal/provider/...
```

### Must Pass
| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| ProviderType constants defined | `ProviderClaude` and `ProviderCodex` constants exist |
| Provider interface compiles | Interface has `Invoke` and `Name` methods |
| Config struct compiles | Struct has `Type` and `Command` fields |

## NOT In Scope
- Provider implementations (ClaudeProvider, CodexProvider)
- Factory function
- Tests (no behavior to test, just type definitions)
