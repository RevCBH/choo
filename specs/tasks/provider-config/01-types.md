---
task: 1
status: complete
backpressure: "go build ./internal/config/..."
depends_on: []
---

# Provider Config Types

**Parent spec**: `/specs/PROVIDER-CONFIG.md`
**Task**: #1 of 4 in implementation plan

## Objective

Define ProviderType, ProviderConfig, and ProviderSettings types for multi-provider support. Add the Provider field to the main Config struct and define default constants.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- None (standard library only for types)

## Deliverables

### Files to Modify

```
internal/
└── config/
    ├── config.go    # MODIFY: Add ProviderType, ProviderConfig, ProviderSettings types and Provider field to Config
    └── defaults.go  # MODIFY: Add DefaultProviderType, DefaultCodexCommand constants and DefaultProviderConfig()
```

### Types to Implement

Add to `internal/config/config.go`:

```go
// ProviderType represents a supported LLM provider for task execution.
type ProviderType string

const (
    ProviderClaude ProviderType = "claude"
    ProviderCodex  ProviderType = "codex"
)

// ProviderConfig holds settings for provider selection and configuration.
type ProviderConfig struct {
    // Type is the default provider type: "claude" (default) or "codex"
    Type ProviderType `yaml:"type"`

    // Command overrides the default CLI binary path for the primary provider
    // Deprecated: use Providers map instead
    Command string `yaml:"command,omitempty"`

    // Providers contains per-provider settings
    Providers map[ProviderType]ProviderSettings `yaml:"providers,omitempty"`
}

// ProviderSettings holds configuration for a specific provider.
type ProviderSettings struct {
    // Command is the CLI binary path or name for this provider
    Command string `yaml:"command"`
}
```

Add to Config struct in `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // Provider contains provider selection and configuration
    Provider ProviderConfig `yaml:"provider"`

    // Claude contains Claude CLI settings (legacy, still supported)
    Claude ClaudeConfig `yaml:"claude"`

    // ... rest of existing fields ...
}
```

### Constants to Add

Add to `internal/config/defaults.go`:

```go
const (
    DefaultCodexCommand string = "codex"
)

// DefaultProviderType is the default provider when none is specified
var DefaultProviderType ProviderType = ProviderClaude
```

### Functions to Implement

Add to `internal/config/defaults.go`:

```go
// DefaultProviderConfig returns provider config with default values.
func DefaultProviderConfig() ProviderConfig {
    return ProviderConfig{
        Type: DefaultProviderType,
        Providers: map[ProviderType]ProviderSettings{
            ProviderClaude: {Command: DefaultClaudeCommand},
            ProviderCodex:  {Command: DefaultCodexCommand},
        },
    }
}
```

Update `DefaultConfig()` in `internal/config/defaults.go` to include:

```go
func DefaultConfig() *Config {
    return &Config{
        // ... existing fields ...
        Provider: DefaultProviderConfig(),
        // ... rest of existing fields ...
    }
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/config/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| Type completeness | All struct fields have yaml tags |
| Constants | ProviderClaude and ProviderCodex are defined |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ProviderType is a string type for easy YAML marshaling/unmarshaling
- ProviderConfig.Command is deprecated but kept for backward compatibility
- The Providers map allows per-provider command customization
- DefaultProviderType must be ProviderClaude for backward compatibility
- Keep existing ClaudeConfig for legacy support

## NOT In Scope

- ResolveProvider function (Task #2)
- Environment variable handling for providers (Task #2)
- CLI flag integration (Task #3)
- Frontmatter parsing (Task #4)
- Validation of provider types (Task #2)
