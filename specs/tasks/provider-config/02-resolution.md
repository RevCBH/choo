---
task: 2
status: pending
backpressure: "go test ./internal/config/... -run Provider"
depends_on: [1]
---

# Provider Resolution

**Parent spec**: `/specs/PROVIDER-CONFIG.md`
**Task**: #2 of 4 in implementation plan

## Objective

Implement the provider resolution system with five-level precedence chain, command lookup, and provider validation.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ProviderType, ProviderConfig, ProviderSettings types)

### Package Dependencies
- `os` - Environment variable access
- `fmt` - Error formatting

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    ├── provider.go       # CREATE: Provider resolution logic
    ├── provider_test.go  # CREATE: Tests for provider resolution
    └── env.go            # MODIFY: Add RALPH_PROVIDER and RALPH_CODEX_CMD handlers
```

### Types to Implement

Add to `internal/config/provider.go`:

```go
// ProviderResolutionContext holds all inputs needed to resolve the provider.
type ProviderResolutionContext struct {
    // ForceTaskProvider from CLI --force-task-provider flag
    ForceTaskProvider string

    // UnitProvider from unit frontmatter
    UnitProvider string

    // CLIProvider from CLI --provider flag
    CLIProvider string

    // EnvProvider from RALPH_PROVIDER environment variable
    EnvProvider string

    // ConfigProvider from .choo.yaml provider.type
    ConfigProvider ProviderType
}

// ResolvedProvider contains the final provider selection and its command.
type ResolvedProvider struct {
    // Type is the resolved provider type
    Type ProviderType

    // Command is the binary path to execute
    Command string

    // Source indicates where the provider was determined from
    Source string
}
```

### Constants to Add

Add to `internal/config/env.go`:

```go
const (
    EnvProvider  = "RALPH_PROVIDER"
    EnvCodexCmd  = "RALPH_CODEX_CMD"
)
```

### Functions to Implement

Add to `internal/config/provider.go`:

```go
// ResolveProvider determines the provider to use based on precedence rules.
// Precedence (highest to lowest):
// 1. ForceTaskProvider (--force-task-provider CLI flag)
// 2. UnitProvider (per-unit frontmatter)
// 3. CLIProvider (--provider CLI flag)
// 4. EnvProvider (RALPH_PROVIDER environment variable)
// 5. ConfigProvider (.choo.yaml provider.type)
// 6. Default (claude)
func ResolveProvider(ctx ProviderResolutionContext, cfg *Config) ResolvedProvider {
    var providerType ProviderType
    var source string

    // Level 1: --force-task-provider (highest precedence)
    if ctx.ForceTaskProvider != "" {
        providerType = ProviderType(ctx.ForceTaskProvider)
        source = "cli:--force-task-provider"
    } else if ctx.UnitProvider != "" {
        // Level 2: Per-unit frontmatter
        providerType = ProviderType(ctx.UnitProvider)
        source = "unit:frontmatter"
    } else if ctx.CLIProvider != "" {
        // Level 3: --provider CLI arg
        providerType = ProviderType(ctx.CLIProvider)
        source = "cli:--provider"
    } else if ctx.EnvProvider != "" {
        // Level 4: RALPH_PROVIDER env var
        providerType = ProviderType(ctx.EnvProvider)
        source = "env:RALPH_PROVIDER"
    } else if cfg.Provider.Type != "" {
        // Level 5: .choo.yaml provider.type
        providerType = cfg.Provider.Type
        source = "config:.choo.yaml"
    } else {
        // Default fallback
        providerType = DefaultProviderType
        source = "default"
    }

    return ResolvedProvider{
        Type:    providerType,
        Command: GetProviderCommand(cfg, providerType),
        Source:  source,
    }
}

// GetProviderCommand returns the command for a given provider type.
// Resolution order:
// 1. provider.providers[type].command in config
// 2. Environment variable (RALPH_CODEX_CMD for codex)
// 3. Legacy config field (claude.command for Claude)
// 4. Default constant
func GetProviderCommand(cfg *Config, providerType ProviderType) string {
    // Check per-provider settings first
    if settings, ok := cfg.Provider.Providers[providerType]; ok && settings.Command != "" {
        return settings.Command
    }

    // Check environment variable for codex
    if providerType == ProviderCodex {
        if cmd := os.Getenv(EnvCodexCmd); cmd != "" {
            return cmd
        }
        return DefaultCodexCommand
    }

    // For Claude, check legacy ClaudeConfig first
    if providerType == ProviderClaude {
        if cfg.Claude.Command != "" {
            return cfg.Claude.Command
        }
        return DefaultClaudeCommand
    }

    // Unknown provider, return type as command (will fail at execution)
    return string(providerType)
}

// ValidateProviderType checks if a provider type string is valid.
// Returns an error if the provider is not supported.
// Empty string is valid (uses default).
func ValidateProviderType(provider string) error {
    switch ProviderType(provider) {
    case ProviderClaude, ProviderCodex:
        return nil
    case "":
        return nil // Empty is valid (uses default)
    default:
        return fmt.Errorf("invalid provider type %q: must be one of: claude, codex", provider)
    }
}
```

Update `internal/config/env.go` to add environment variable handlers:

```go
// Add to envOverrides slice
{
    envVar: EnvProvider,
    apply: func(c *Config, v string) {
        c.Provider.Type = ProviderType(v)
    },
},
{
    envVar: EnvCodexCmd,
    apply: func(c *Config, v string) {
        if c.Provider.Providers == nil {
            c.Provider.Providers = make(map[ProviderType]ProviderSettings)
        }
        c.Provider.Providers[ProviderCodex] = ProviderSettings{Command: v}
    },
},
```

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run Provider -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestResolveProvider_ForceTakesPrecedence` | --force-task-provider overrides all other settings |
| `TestResolveProvider_UnitOverridesDefault` | Unit frontmatter takes precedence over CLI default |
| `TestResolveProvider_CLIPrecedenceOverEnv` | --provider flag takes precedence over env var |
| `TestResolveProvider_EnvPrecedenceOverConfig` | RALPH_PROVIDER takes precedence over config file |
| `TestResolveProvider_ConfigFallback` | Config provider.type used when no higher precedence |
| `TestResolveProvider_DefaultFallback` | Claude is default when nothing specified |
| `TestGetProviderCommand_CustomPath` | Custom command from providers map is used |
| `TestGetProviderCommand_EnvOverride` | RALPH_CODEX_CMD overrides default codex command |
| `TestGetProviderCommand_LegacyClaudeConfig` | claude.command is respected for Claude provider |
| `TestValidateProviderType_Valid` | claude and codex pass validation |
| `TestValidateProviderType_Empty` | Empty string passes validation |
| `TestValidateProviderType_Invalid` | Unknown provider returns error |

### Test Implementation

Add to `internal/config/provider_test.go`:

```go
package config

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestResolveProvider_ForceTakesPrecedence(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Type: ProviderClaude,
            Providers: map[ProviderType]ProviderSettings{
                ProviderClaude: {Command: "claude"},
                ProviderCodex:  {Command: "codex"},
            },
        },
    }

    ctx := ProviderResolutionContext{
        ForceTaskProvider: "codex",
        UnitProvider:      "claude",
        CLIProvider:       "claude",
        EnvProvider:       "claude",
        ConfigProvider:    ProviderClaude,
    }

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderCodex, result.Type)
    assert.Equal(t, "codex", result.Command)
    assert.Equal(t, "cli:--force-task-provider", result.Source)
}

func TestResolveProvider_UnitOverridesDefault(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Type: ProviderClaude,
            Providers: map[ProviderType]ProviderSettings{
                ProviderClaude: {Command: "claude"},
                ProviderCodex:  {Command: "codex"},
            },
        },
    }

    ctx := ProviderResolutionContext{
        UnitProvider:   "codex",
        CLIProvider:    "claude",
        EnvProvider:    "claude",
        ConfigProvider: ProviderClaude,
    }

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderCodex, result.Type)
    assert.Equal(t, "codex", result.Command)
    assert.Equal(t, "unit:frontmatter", result.Source)
}

func TestResolveProvider_CLIPrecedenceOverEnv(t *testing.T) {
    cfg := &Config{
        Provider: DefaultProviderConfig(),
    }

    ctx := ProviderResolutionContext{
        CLIProvider: "codex",
        EnvProvider: "claude",
    }

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderCodex, result.Type)
    assert.Equal(t, "cli:--provider", result.Source)
}

func TestResolveProvider_EnvPrecedenceOverConfig(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Type:      ProviderClaude,
            Providers: DefaultProviderConfig().Providers,
        },
    }

    ctx := ProviderResolutionContext{
        EnvProvider:    "codex",
        ConfigProvider: ProviderClaude,
    }

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderCodex, result.Type)
    assert.Equal(t, "env:RALPH_PROVIDER", result.Source)
}

func TestResolveProvider_ConfigFallback(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Type:      ProviderCodex,
            Providers: DefaultProviderConfig().Providers,
        },
    }

    ctx := ProviderResolutionContext{}

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderCodex, result.Type)
    assert.Equal(t, "config:.choo.yaml", result.Source)
}

func TestResolveProvider_DefaultFallback(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Providers: DefaultProviderConfig().Providers,
        },
    }

    ctx := ProviderResolutionContext{}

    result := ResolveProvider(ctx, cfg)

    assert.Equal(t, ProviderClaude, result.Type)
    assert.Equal(t, "claude", result.Command)
    assert.Equal(t, "default", result.Source)
}

func TestGetProviderCommand_CustomPath(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{
            Providers: map[ProviderType]ProviderSettings{
                ProviderCodex: {Command: "/custom/path/codex"},
            },
        },
    }

    cmd := GetProviderCommand(cfg, ProviderCodex)
    assert.Equal(t, "/custom/path/codex", cmd)
}

func TestGetProviderCommand_EnvOverride(t *testing.T) {
    cfg := &Config{
        Provider: DefaultProviderConfig(),
    }

    t.Setenv("RALPH_CODEX_CMD", "/env/codex")

    cmd := GetProviderCommand(cfg, ProviderCodex)
    assert.Equal(t, "/env/codex", cmd)
}

func TestGetProviderCommand_LegacyClaudeConfig(t *testing.T) {
    cfg := &Config{
        Provider: ProviderConfig{},
        Claude: ClaudeConfig{
            Command: "/legacy/claude",
        },
    }

    cmd := GetProviderCommand(cfg, ProviderClaude)
    assert.Equal(t, "/legacy/claude", cmd)
}

func TestValidateProviderType(t *testing.T) {
    tests := []struct {
        provider string
        wantErr  bool
    }{
        {"claude", false},
        {"codex", false},
        {"", false}, // Empty is valid
        {"gemini", true},
        {"CLAUDE", true}, // Case-sensitive
        {"invalid", true},
    }

    for _, tt := range tests {
        t.Run(tt.provider, func(t *testing.T) {
            err := ValidateProviderType(tt.provider)
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), "invalid provider type")
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ResolveProvider must check precedence levels in exact order specified
- GetProviderCommand must check environment variables for codex command
- ValidateProviderType is case-sensitive (provider types are lowercase)
- Empty provider type is valid and results in using the default
- Source field in ResolvedProvider is useful for debugging and logging

## NOT In Scope

- CLI flag parsing (Task #3)
- Frontmatter parsing (Task #4)
- Provider health checks (future enhancement)
- Provider capability detection (future enhancement)
