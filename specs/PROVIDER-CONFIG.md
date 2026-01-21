# PROVIDER-CONFIG - Configuration Schema, Environment Variables, CLI Flags, and Precedence Resolution for Provider Selection

## Overview

The provider configuration system enables multi-provider support for task execution in choo. It defines a layered configuration approach where provider selection can be specified at five distinct levels with clear precedence semantics. This flexibility allows project-wide defaults, environment-specific overrides, and per-unit customization.

The core design principle is backward compatibility: existing Claude-only workflows continue working unchanged. The default provider is Claude, and all existing configuration files work without modification. Codex (or future providers) can be enabled incrementally at any configuration level.

```
+-----------------------------------------------------------------------------+
|  Configuration Resolution (for task provider selection)                      |
|                                                                              |
|  Precedence (highest to lowest):                                            |
|  1. --force-task-provider    --> Overrides everything (for inner loops)     |
|  2. Per-unit frontmatter     --> Unit author specifies provider             |
|  3. --provider CLI arg       --> Default for units without override         |
|  4. RALPH_PROVIDER env var   --> Environment-level default                  |
|  5. .choo.yaml global        --> Fallback default                           |
+-----------------------------------------------------------------------------+
          |
          v
+--------------------+     +--------------------+     +--------------------+
|  Force Provider    |     |  Unit Frontmatter  |     |  Resolved Default  |
|  (CLI override)    |     |  (per-unit)        |     |  (env/file/default)|
+--------------------+     +--------------------+     +--------------------+
          |                        |                        |
          +------------------------+------------------------+
                                   |
                                   v
                    +-----------------------------+
                    |  Task Execution Provider    |
                    |  (Claude or Codex binary)   |
                    +-----------------------------+
```

## Requirements

### Functional Requirements

1. Support `claude` and `codex` as valid provider types
2. Default to `claude` when no provider is specified at any level
3. Allow per-unit provider override via frontmatter `provider` field
4. Support `--provider` CLI flag to set default provider for units without frontmatter override
5. Support `--force-task-provider` CLI flag to override all provider settings including per-unit frontmatter
6. Support `RALPH_PROVIDER` environment variable for environment-level defaults
7. Support `RALPH_CODEX_CMD` environment variable to override codex binary path
8. Allow custom binary paths for each provider in `.choo.yaml`
9. Validate provider type is one of the supported values
10. Maintain backward compatibility with existing `.choo.yaml` files

### Performance Requirements

| Metric | Target |
|--------|--------|
| Provider resolution time | <1ms |
| Config validation time | <10ms |
| Memory overhead | <1KB per provider config |

### Constraints

- `--force-task-provider` only affects task execution inner loops, not other LLM operations (merge conflict resolution, branch naming, PR creation remain Claude-only)
- Provider binaries must be available in PATH or specified as absolute paths
- Empty `provider.type` falls back to Claude (backward compatibility)
- Environment variables override config file values
- CLI flags override environment variables

## Design

### Module Structure

```
internal/
+-- config/
|   +-- config.go       # Add ProviderConfig to Config struct
|   +-- defaults.go     # Add DefaultProviderType, DefaultCodexCommand
|   +-- env.go          # Add RALPH_PROVIDER, RALPH_CODEX_CMD handlers
|   +-- provider.go     # Provider resolution logic (NEW)
+-- cli/
|   +-- run.go          # Add --provider and --force-task-provider flags
+-- discovery/
    +-- frontmatter.go  # Add Provider field to UnitFrontmatter
```

### Core Types

```go
// internal/config/config.go

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

// Config holds all configuration for the Ralph Orchestrator.
// Updated to include ProviderConfig.
type Config struct {
    // ... existing fields ...

    // Provider contains provider selection and configuration
    Provider ProviderConfig `yaml:"provider"`

    // Claude contains Claude CLI settings (legacy, still supported)
    Claude ClaudeConfig `yaml:"claude"`

    // ... rest of existing fields ...
}
```

```go
// internal/discovery/frontmatter.go

// UnitFrontmatter represents parsed YAML frontmatter from a unit file.
type UnitFrontmatter struct {
    // Unit is the unique identifier for this unit
    Unit string `yaml:"unit"`

    // DependsOn lists unit IDs this unit depends on
    DependsOn []string `yaml:"depends_on,omitempty"`

    // Provider overrides the default provider for this unit's task execution
    // Valid values: "claude", "codex"
    // Empty means use the resolved default from CLI/env/config
    Provider string `yaml:"provider,omitempty"`

    // ... any other existing fields ...
}
```

```go
// internal/cli/run.go

// RunOptions holds CLI flags for the run command.
type RunOptions struct {
    // ... existing fields ...

    // Provider is the default provider for task execution
    // Units without frontmatter override use this provider
    Provider string

    // ForceTaskProvider overrides all provider settings for task inner loops
    // When set, ignores per-unit frontmatter provider field
    ForceTaskProvider string
}
```

```go
// internal/config/provider.go

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

### API Surface

```go
// internal/config/provider.go

// ResolveProvider determines the provider to use based on precedence rules.
// Returns the resolved provider type, command, and source.
func ResolveProvider(ctx ProviderResolutionContext, cfg *Config) ResolvedProvider

// GetProviderCommand returns the command for a given provider type.
// Checks Providers map first, then falls back to defaults.
func GetProviderCommand(cfg *Config, providerType ProviderType) string

// ValidateProviderType checks if a provider type string is valid.
// Returns an error if the provider is not supported.
func ValidateProviderType(provider string) error
```

```go
// internal/config/defaults.go

const (
    DefaultProviderType   ProviderType = ProviderClaude
    DefaultClaudeCommand  string       = "claude"
    DefaultCodexCommand   string       = "codex"
)
```

```go
// internal/config/env.go

// Environment variable names
const (
    EnvProvider  = "RALPH_PROVIDER"
    EnvCodexCmd  = "RALPH_CODEX_CMD"
)
```

### Implementation Details

```go
// internal/config/provider.go

import (
    "fmt"
    "os"
)

// ResolveProvider implements the five-level precedence chain.
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

```go
// internal/config/env.go (additions)

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

```go
// internal/config/defaults.go (additions)

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

### YAML Configuration Examples

```yaml
# .choo.yaml - Minimal provider config (uses defaults)
# No provider section needed - defaults to claude

# .choo.yaml - Explicit provider selection
provider:
  type: claude

# .choo.yaml - Use codex as default
provider:
  type: codex

# .choo.yaml - Full provider configuration with custom commands
provider:
  type: claude
  providers:
    claude:
      command: /usr/local/bin/claude
    codex:
      command: /opt/codex/bin/codex
```

### Unit Frontmatter Examples

```yaml
---
unit: my-feature
depends_on:
  - base-types
---
# Uses resolved default provider (from CLI/env/config)
```

```yaml
---
unit: codex-optimized-feature
provider: codex
depends_on:
  - base-types
---
# This unit always uses codex (unless --force-task-provider overrides)
```

### CLI Usage Examples

```bash
# Use default provider (from config/env, or claude if unset)
choo run

# Use codex as default for all units without frontmatter override
choo run --provider=codex

# Force ALL units to use codex (ignores per-unit frontmatter)
choo run --force-task-provider=codex

# Combine: codex for most, but specific units can use claude via frontmatter
choo run --provider=codex

# Override everything: all inner loops use codex regardless of frontmatter
choo run --force-task-provider=codex
```

## Implementation Notes

### Force Task Provider Scope

The `--force-task-provider` flag specifically targets task execution inner loops. It does NOT affect:
- Merge conflict resolution (always Claude)
- Branch naming suggestions (always Claude)
- PR description generation (always Claude)
- Any other non-task LLM operations

This scope limitation is intentional per the PRD requirements and ensures predictable behavior for operations that rely on Claude-specific capabilities.

### Backward Compatibility

The system maintains full backward compatibility:

1. **No provider config**: Defaults to Claude, existing `ClaudeConfig` used
2. **Empty provider.type**: Treated as Claude
3. **Legacy ClaudeConfig.Command**: Still respected for Claude provider
4. **No frontmatter provider**: Uses resolved default

Existing `.choo.yaml` files work without any modifications.

### Provider Command Resolution Order

For each provider, the command is resolved in this order:

1. `provider.providers[type].command` in config
2. Environment variable (e.g., `RALPH_CODEX_CMD` for codex)
3. Legacy config field (e.g., `claude.command` for Claude)
4. Default constant (e.g., `"claude"` or `"codex"`)

### Error Handling

Invalid provider types are caught at multiple levels:
- Config validation rejects invalid `provider.type`
- CLI flag validation rejects invalid `--provider` or `--force-task-provider`
- Frontmatter parsing warns on invalid `provider` field (non-fatal, uses default)

## Testing Strategy

### Unit Tests

```go
// internal/config/provider_test.go

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
        UnitProvider:      "claude",  // Should be ignored
        CLIProvider:       "claude",  // Should be ignored
        EnvProvider:       "claude",  // Should be ignored
        ConfigProvider:    ProviderClaude, // Should be ignored
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
                assert.Error(t, err)
                assert.Contains(t, err.Error(), "invalid provider type")
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

```go
// internal/discovery/frontmatter_test.go

func TestParseFrontmatter_WithProvider(t *testing.T) {
    input := `---
unit: my-feature
provider: codex
depends_on:
  - base-types
---
# Task content
`
    fm, err := ParseFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "my-feature", fm.Unit)
    assert.Equal(t, "codex", fm.Provider)
    assert.Equal(t, []string{"base-types"}, fm.DependsOn)
}

func TestParseFrontmatter_WithoutProvider(t *testing.T) {
    input := `---
unit: my-feature
depends_on:
  - base-types
---
# Task content
`
    fm, err := ParseFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "my-feature", fm.Unit)
    assert.Equal(t, "", fm.Provider) // Empty, uses default
}
```

### Integration Tests

| Scenario | Description |
|----------|-------------|
| Default behavior | No provider config, verify Claude is used |
| Config file provider | Set `provider.type: codex`, verify units use codex |
| CLI --provider flag | Run with `--provider=codex`, verify units use codex |
| CLI --force-task-provider | Run with `--force-task-provider=codex`, verify all units use codex regardless of frontmatter |
| Environment variable | Set `RALPH_PROVIDER=codex`, verify units use codex |
| Unit frontmatter override | Unit with `provider: codex`, default Claude, verify unit uses codex |
| Precedence chain | Set all levels to different values, verify correct precedence |
| Custom command paths | Set custom binary paths, verify correct binary is executed |
| Backward compatibility | Existing config with only `claude.command`, verify still works |

### Manual Testing

- [ ] Run `choo run` without any provider config, verify Claude is used
- [ ] Add `provider.type: codex` to `.choo.yaml`, verify units use codex
- [ ] Run `choo run --provider=codex`, verify units use codex
- [ ] Run `choo run --force-task-provider=codex` with mixed frontmatter, verify all use codex
- [ ] Set `RALPH_PROVIDER=codex`, verify units use codex
- [ ] Create unit with `provider: codex` frontmatter, verify it uses codex while others use default
- [ ] Set `RALPH_CODEX_CMD=/custom/codex`, verify custom path is used
- [ ] Verify invalid provider type shows clear error message

## Design Decisions

### Why Five Precedence Levels?

The five-level precedence chain balances flexibility with simplicity:

1. **--force-task-provider**: Needed for testing, debugging, and CI scenarios where you want to force a specific provider regardless of per-unit settings
2. **Per-unit frontmatter**: Allows unit authors to specify the best provider for their task (some tasks may work better with specific models)
3. **--provider CLI**: Provides session-level default without modifying files
4. **Environment variable**: Standard for CI/CD and containerized environments
5. **Config file**: Project-level default committed to version control

Fewer levels would reduce flexibility; more levels would add unnecessary complexity.

### Why Separate --provider and --force-task-provider?

These serve distinct purposes:

- `--provider`: "Use this as the default for units that don't specify their own provider"
- `--force-task-provider`: "Override everything, use this provider for all task execution"

The distinction allows unit authors to specify preferred providers while still giving operators the ability to force a specific provider when needed (e.g., during testing or when one provider is unavailable).

### Why Not Use Generic LLM Abstraction?

A generic LLM abstraction was considered but rejected because:

1. Claude and Codex have different CLI interfaces and arguments
2. The PRD specifies only task execution supports multiple providers
3. Over-abstraction would add complexity without immediate benefit
4. Future providers can be added by extending the current pattern

### Why Keep Legacy ClaudeConfig?

The existing `ClaudeConfig` struct is preserved for backward compatibility:

- Existing configs continue to work unchanged
- `claude.command` is respected as a fallback for Claude provider
- Migration to `provider.providers.claude.command` is optional
- No breaking changes for existing users

## Future Enhancements

1. **Provider health checks**: Verify provider binary exists and is executable at startup
2. **Provider capabilities**: Define what each provider supports (e.g., tool use, vision)
3. **Auto-fallback**: Automatically fall back to alternate provider on failure
4. **Provider-specific settings**: Per-provider configuration beyond just command path
5. **Provider metrics**: Track success/failure rates per provider
6. **Dynamic provider selection**: Choose provider based on task characteristics

## References

- [Multi-Provider PRD](../docs/MULTI-PROVIDER-PRD.md) - Full PRD for multi-provider support
- [CONFIG spec](./completed/CONFIG.md) - Base configuration system
- [DISCOVERY spec](./completed/DISCOVERY.md) - Unit discovery and frontmatter parsing
- [CLI spec](./completed/CLI.md) - CLI flag handling
