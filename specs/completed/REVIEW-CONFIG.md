# REVIEW-CONFIG — Configuration Types and Loading for Advisory Code Review

## Overview

The REVIEW-CONFIG module defines the configuration types and loading logic for the advisory code review system. It provides a `CodeReviewConfig` struct that controls whether code review runs, which provider to use, iteration limits, and verbosity settings.

This module exists to decouple the code review feature from the rest of the system, allowing users to enable/disable reviews, switch providers, and tune behavior without modifying orchestrator logic. Configuration is loaded from `.choo.yaml` with sensible defaults, ensuring backwards compatibility with existing workflows.

**Important**: Code review is integrated into the `choo run` workflow—there is no separate CLI command for review. Review runs automatically after unit task completion and again after all units merge to the feature branch (before rebase/merge to target).

```
┌─────────────────────────────────────────────────────────────────┐
│                     Configuration Loading                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐         ┌──────────────────────────────────┐ │
│   │  .choo.yaml  │────────▶│        Config Loader             │ │
│   │              │         │                                  │ │
│   │ code_review: │         │  1. Parse YAML                   │ │
│   │   enabled:   │         │  2. Apply defaults               │ │
│   │   provider:  │         │  3. Validate                     │ │
│   └──────────────┘         └───────────────┬──────────────────┘ │
│                                            │                     │
│                                            ▼                     │
│                            ┌──────────────────────────────────┐ │
│                            │       CodeReviewConfig           │ │
│                            │                                  │ │
│                            │  Enabled: bool                   │ │
│                            │  Provider: ProviderType          │ │
│                            │  MaxFixIterations: int           │ │
│                            │  Command: string                 │ │
│                            └───────────────┬──────────────────┘ │
│                                            │                     │
│                                            ▼                     │
│                            ┌──────────────────────────────────┐ │
│                            │     Orchestrator.resolveReviewer │ │
│                            │                                  │ │
│                            │  Returns: CodexReviewer          │ │
│                            │       or: ClaudeReviewer         │ │
│                            │       or: nil (disabled)         │ │
│                            └──────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Define `CodeReviewConfig` struct with fields: `Enabled`, `Provider`, `MaxFixIterations`, `Command`
2. Provide `DefaultCodeReviewConfig()` function returning sensible defaults (enabled=true, provider=codex, iterations=1)
3. Support YAML deserialization via struct tags for `.choo.yaml` integration
4. Allow disabling code review entirely via `enabled: false`
5. Support "codex" and "claude" provider types
6. Support configurable max fix iterations (0 disables fix attempts)
7. Support optional CLI command path override
8. Integrate `CodeReviewConfig` into main `Config` struct
9. Provide `resolveReviewer()` function in orchestrator to instantiate the correct reviewer

### Performance Requirements

| Metric | Target |
|--------|--------|
| Config load time | <5ms |
| Memory overhead | <1KB per config instance |
| Default application | Compile-time constants, no runtime overhead |

### Constraints

- Configuration MUST be backwards compatible (missing `code_review` section uses defaults)
- Review provider MUST be independent of task execution provider
- Provider type MUST be validated at load time to fail fast on typos
- `MaxFixIterations` MUST accept 0 to support review-only mode without fix attempts

## Design

### Module Structure

```
internal/config/
├── config.go       # Main Config struct, includes CodeReviewConfig
└── defaults.go     # DefaultCodeReviewConfig() function

internal/orchestrator/
└── orchestrator.go # resolveReviewer() method added
```

### Core Types

```go
// internal/config/config.go

// ProviderType represents a code review provider.
type ProviderType string

const (
    // ProviderCodex uses OpenAI Codex for code review.
    ProviderCodex ProviderType = "codex"

    // ProviderClaude uses Anthropic Claude for code review.
    ProviderClaude ProviderType = "claude"
)

// CodeReviewConfig controls the advisory code review system.
type CodeReviewConfig struct {
    // Enabled controls whether code review runs. Default: true.
    // When enabled, review runs after each unit completes AND after
    // all units merge to the feature branch (before final rebase/merge).
    Enabled bool `yaml:"enabled"`

    // Provider specifies which reviewer to use: "codex" or "claude".
    // Default: "codex".
    Provider ProviderType `yaml:"provider"`

    // MaxFixIterations limits how many times the system attempts fixes
    // per review cycle. Default: 1 (single review-fix cycle).
    // Set to 0 to disable fix attempts (review-only mode).
    MaxFixIterations int `yaml:"max_fix_iterations"`

    // Verbose controls output verbosity. Default: true (noisy).
    // When true, review findings are printed to stderr even when passing.
    // When false, only issues requiring attention are printed.
    Verbose bool `yaml:"verbose"`

    // Command overrides the CLI path for the reviewer.
    // Default: "" (uses system PATH to find "codex" or "claude").
    Command string `yaml:"command,omitempty"`
}

// Config is the main configuration struct for choo.
type Config struct {
    // ... existing fields ...

    // CodeReview configures the advisory code review system.
    CodeReview CodeReviewConfig `yaml:"code_review"`
}
```

```go
// internal/config/defaults.go

// DefaultCodeReviewConfig returns sensible defaults for code review.
// Defaults are: enabled, codex provider, 1 fix iteration, verbose output.
func DefaultCodeReviewConfig() CodeReviewConfig {
    return CodeReviewConfig{
        Enabled:          true,
        Provider:         ProviderCodex,
        MaxFixIterations: 1,
        Verbose:          true, // Default on and noisy
        Command:          "",
    }
}
```

### API Surface

```go
// internal/config/config.go

// DefaultCodeReviewConfig returns the default code review configuration.
func DefaultCodeReviewConfig() CodeReviewConfig

// Validate checks that the CodeReviewConfig is valid.
func (c *CodeReviewConfig) Validate() error

// IsReviewOnlyMode returns true if fixes are disabled.
func (c *CodeReviewConfig) IsReviewOnlyMode() bool
```

```go
// internal/orchestrator/orchestrator.go

// resolveReviewer returns the appropriate reviewer based on configuration.
// Returns nil if code review is disabled.
func (o *Orchestrator) resolveReviewer() (provider.Reviewer, error)
```

### YAML Schema

```yaml
# .choo.yaml additions

code_review:
  # Enable/disable code review. Default: true
  # Review runs as part of `choo run` workflow (no separate CLI command)
  enabled: true

  # Review provider: "codex" (default) or "claude"
  provider: codex

  # Maximum fix iterations after review. Default: 1
  # Set to 0 to disable fix attempts (review-only mode)
  max_fix_iterations: 1

  # Verbose output. Default: true (noisy)
  # When true, always prints review summary to stderr
  verbose: true

  # Optional: Override CLI path for the reviewer
  # Default: uses system PATH to find "codex" or "claude"
  command: ""
```

### Orchestrator Integration

```go
// internal/orchestrator/orchestrator.go

func (o *Orchestrator) resolveReviewer() (provider.Reviewer, error) {
    cfg := o.config.CodeReview

    if !cfg.Enabled {
        return nil, nil // Review disabled
    }

    switch cfg.Provider {
    case config.ProviderCodex:
        return provider.NewCodexReviewer(cfg.Command), nil
    case config.ProviderClaude:
        return provider.NewClaudeReviewer(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown review provider: %s", cfg.Provider)
    }
}
```

## Implementation Notes

### Backwards Compatibility

When loading configuration from YAML, missing fields receive Go zero values. The config loader must apply defaults explicitly:

```go
func LoadConfig(path string) (*Config, error) {
    cfg := &Config{
        CodeReview: DefaultCodeReviewConfig(), // Pre-fill defaults
    }

    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return cfg, nil // Use defaults if no config file
        }
        return nil, err
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }

    return cfg, nil
}
```

### Validation

Provider validation should happen at config load time to fail fast:

```go
func (c *CodeReviewConfig) Validate() error {
    if c.Enabled {
        switch c.Provider {
        case ProviderCodex, ProviderClaude:
            // Valid
        default:
            return fmt.Errorf("invalid review provider: %q (must be 'codex' or 'claude')", c.Provider)
        }
    }

    if c.MaxFixIterations < 0 {
        return fmt.Errorf("max_fix_iterations cannot be negative: %d", c.MaxFixIterations)
    }

    return nil
}
```

### Review-Only Mode

When `max_fix_iterations: 0`, the system performs code review but does not attempt to apply fixes. This is useful for audit-only workflows:

```go
func (c *CodeReviewConfig) IsReviewOnlyMode() bool {
    return c.MaxFixIterations == 0
}
```

### Provider Independence

The review provider is deliberately independent from the task execution provider. A workflow might use Claude for task execution but Codex for review, or vice versa. This separation allows users to optimize each stage independently.

## Testing Strategy

### Unit Tests

```go
// internal/config/config_test.go

func TestDefaultCodeReviewConfig(t *testing.T) {
    cfg := DefaultCodeReviewConfig()

    if !cfg.Enabled {
        t.Error("expected Enabled to be true by default")
    }
    if cfg.Provider != ProviderCodex {
        t.Errorf("expected Provider to be %q, got %q", ProviderCodex, cfg.Provider)
    }
    if cfg.MaxFixIterations != 1 {
        t.Errorf("expected MaxFixIterations to be 1, got %d", cfg.MaxFixIterations)
    }
    if !cfg.Verbose {
        t.Error("expected Verbose to be true by default (noisy)")
    }
    if cfg.Command != "" {
        t.Errorf("expected Command to be empty, got %q", cfg.Command)
    }
}

func TestCodeReviewConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        cfg     CodeReviewConfig
        wantErr bool
    }{
        {
            name: "valid codex config",
            cfg: CodeReviewConfig{
                Enabled:          true,
                Provider:         ProviderCodex,
                MaxFixIterations: 1,
            },
            wantErr: false,
        },
        {
            name: "valid claude config",
            cfg: CodeReviewConfig{
                Enabled:          true,
                Provider:         ProviderClaude,
                MaxFixIterations: 3,
            },
            wantErr: false,
        },
        {
            name: "valid disabled config",
            cfg: CodeReviewConfig{
                Enabled:  false,
                Provider: "invalid", // Invalid but ignored when disabled
            },
            wantErr: false,
        },
        {
            name: "invalid provider",
            cfg: CodeReviewConfig{
                Enabled:  true,
                Provider: "gpt4",
            },
            wantErr: true,
        },
        {
            name: "negative iterations",
            cfg: CodeReviewConfig{
                Enabled:          true,
                Provider:         ProviderCodex,
                MaxFixIterations: -1,
            },
            wantErr: true,
        },
        {
            name: "review-only mode (0 iterations)",
            cfg: CodeReviewConfig{
                Enabled:          true,
                Provider:         ProviderCodex,
                MaxFixIterations: 0,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.cfg.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestCodeReviewConfig_IsReviewOnlyMode(t *testing.T) {
    tests := []struct {
        iterations int
        want       bool
    }{
        {0, true},
        {1, false},
        {5, false},
    }

    for _, tt := range tests {
        t.Run(fmt.Sprintf("iterations=%d", tt.iterations), func(t *testing.T) {
            cfg := CodeReviewConfig{MaxFixIterations: tt.iterations}
            if got := cfg.IsReviewOnlyMode(); got != tt.want {
                t.Errorf("IsReviewOnlyMode() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

```go
// internal/config/load_test.go

func TestLoadConfig_MissingCodeReview(t *testing.T) {
    // YAML with no code_review section
    yamlContent := `
parallelism: 4
target_branch: main
`
    tmpFile := filepath.Join(t.TempDir(), ".choo.yaml")
    if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }

    cfg, err := LoadConfig(tmpFile)
    if err != nil {
        t.Fatalf("LoadConfig() error = %v", err)
    }

    // Should have defaults applied
    if !cfg.CodeReview.Enabled {
        t.Error("expected CodeReview.Enabled to default to true")
    }
    if cfg.CodeReview.Provider != ProviderCodex {
        t.Errorf("expected CodeReview.Provider to default to %q", ProviderCodex)
    }
}

func TestLoadConfig_PartialCodeReview(t *testing.T) {
    // YAML with partial code_review section
    yamlContent := `
code_review:
  provider: claude
`
    tmpFile := filepath.Join(t.TempDir(), ".choo.yaml")
    if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }

    cfg, err := LoadConfig(tmpFile)
    if err != nil {
        t.Fatalf("LoadConfig() error = %v", err)
    }

    // Explicit value should be used
    if cfg.CodeReview.Provider != ProviderClaude {
        t.Errorf("expected Provider to be %q, got %q", ProviderClaude, cfg.CodeReview.Provider)
    }

    // Other fields should have defaults
    if !cfg.CodeReview.Enabled {
        t.Error("expected Enabled to default to true")
    }
    if cfg.CodeReview.MaxFixIterations != 1 {
        t.Errorf("expected MaxFixIterations to default to 1, got %d", cfg.CodeReview.MaxFixIterations)
    }
}
```

```go
// internal/orchestrator/orchestrator_test.go

func TestOrchestrator_resolveReviewer(t *testing.T) {
    tests := []struct {
        name       string
        cfg        config.CodeReviewConfig
        wantNil    bool
        wantType   string
        wantErr    bool
    }{
        {
            name: "disabled returns nil",
            cfg: config.CodeReviewConfig{
                Enabled: false,
            },
            wantNil: true,
            wantErr: false,
        },
        {
            name: "codex provider",
            cfg: config.CodeReviewConfig{
                Enabled:  true,
                Provider: config.ProviderCodex,
            },
            wantNil:  false,
            wantType: "*provider.CodexReviewer",
            wantErr:  false,
        },
        {
            name: "claude provider",
            cfg: config.CodeReviewConfig{
                Enabled:  true,
                Provider: config.ProviderClaude,
            },
            wantNil:  false,
            wantType: "*provider.ClaudeReviewer",
            wantErr:  false,
        },
        {
            name: "unknown provider",
            cfg: config.CodeReviewConfig{
                Enabled:  true,
                Provider: "unknown",
            },
            wantNil: false,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            o := &Orchestrator{
                config: &config.Config{CodeReview: tt.cfg},
            }

            reviewer, err := o.resolveReviewer()

            if (err != nil) != tt.wantErr {
                t.Errorf("resolveReviewer() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if tt.wantNil && reviewer != nil {
                t.Error("expected nil reviewer")
            }

            if !tt.wantNil && !tt.wantErr {
                gotType := fmt.Sprintf("%T", reviewer)
                if gotType != tt.wantType {
                    t.Errorf("reviewer type = %s, want %s", gotType, tt.wantType)
                }
            }
        })
    }
}
```

### Integration Tests

| Scenario | Setup | Verification |
|----------|-------|--------------|
| Config file with code_review section | Create `.choo.yaml` with explicit values | Values loaded correctly |
| Config file without code_review section | Create `.choo.yaml` without section | Defaults applied |
| No config file | Run without `.choo.yaml` | Defaults used |
| Invalid provider in config | Create config with `provider: invalid` | Validation error at load |
| Review disabled | Set `enabled: false` | `resolveReviewer()` returns nil |

### Manual Testing

- [ ] Run `choo run` with no `.choo.yaml` - verify code review defaults to enabled with Codex, verbose output
- [ ] Add `code_review: { enabled: false }` - verify no review runs during `choo run`
- [ ] Set `provider: claude` - verify Claude reviewer is used
- [ ] Set `max_fix_iterations: 0` - verify review runs but no fix attempts
- [ ] Set `verbose: false` - verify quieter output (only issues printed)
- [ ] Set invalid provider - verify clear error message at startup
- [ ] Override command path - verify custom binary is invoked
- [ ] Verify review runs after each unit AND after all units merge (two review points)

## Design Decisions

### Why Enabled and Verbose by Default?

Code review provides value out of the box by catching issues before they reach pull requests. The "default on and noisy" philosophy ensures:

1. **Visibility**: Users immediately see that review is happening and what it finds
2. **Quality defaults**: New users get the full benefit without configuration
3. **Easy opt-out**: Users who want quiet operation can set `verbose: false`
4. **Easy disable**: Users who don't want review can set `enabled: false`

Trade-off: Some users may find the output verbose. Mitigation: Clear configuration options for both `enabled` and `verbose` settings.

### Why Codex as Default Provider?

Codex is widely available and has lower latency than Claude for code-focused tasks. Users can switch to Claude if they prefer its review style.

Trade-off: Codex may have different quality characteristics. Mitigation: Easy provider switching, provider is independent of task execution.

### Why Independent Review Provider?

The review provider serves a different purpose than the task execution provider. Task execution needs creative problem-solving; review needs critical analysis. Different models may excel at each. Keeping them independent allows users to optimize each stage.

Trade-off: Configuration complexity with two provider settings. Mitigation: Clear naming (`provider` for tasks, `code_review.provider` for review).

### Why Support Zero Iterations?

`max_fix_iterations: 0` enables review-only mode where the system reports issues but doesn't attempt fixes. This is useful for:
- Auditing existing code without modification
- Teams that want human-in-the-loop for all fixes
- Debugging the review system itself

Trade-off: Slight API complexity (0 has special meaning). Mitigation: Clear documentation, explicit helper method.

## Future Enhancements

1. **Per-unit provider override**: Allow different units to use different review providers via frontmatter
2. **Review threshold configuration**: Set minimum severity level to trigger fix attempts
3. **Provider-specific options**: Nested config for provider-specific settings (model name, temperature, etc.)
4. **Review caching**: Skip review for unchanged files to improve iteration speed
5. **Custom review prompts**: Allow users to provide domain-specific review instructions

## References

- [Code Review PRD](/Users/bennett/conductor/workspaces/choo/columbus/specs/tasks/code-review/PRD.md)
- [REVIEW-PROVIDER Spec](REVIEW-PROVIDER.md) - Provider interface and implementations
- [REVIEW-LOOP Spec](REVIEW-LOOP.md) - Review-fix iteration logic
- Go YAML library: [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
