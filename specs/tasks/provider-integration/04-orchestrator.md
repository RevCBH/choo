---
task: 4
status: pending
backpressure: "go test ./internal/orchestrator/... -run Provider"
depends_on: [3]
---

# Add resolveProviderForUnit to Orchestrator

**Parent spec**: `/specs/PROVIDER-INTEGRATION.md`
**Task**: #4 of 4 in implementation plan

## Objective

Add provider configuration fields to the Orchestrator Config and implement `resolveProviderForUnit` with the five-level precedence chain. Wire the provider factory to the worker pool during initialization.

## Dependencies

### External Specs (must be implemented)
- PROVIDER - provides `Provider` interface, `ProviderType`, `FromConfig`
- CONFIG - provides `ProviderConfig` type

### Task Dependencies (within this unit)
- Task #3 (03-pool-factory.md) - Pool accepts ProviderFactory

### Package Dependencies
- `github.com/RevCBH/choo/internal/provider`
- `github.com/RevCBH/choo/internal/config`

## Deliverables

### Files to Modify

```
internal/orchestrator/
└── orchestrator.go    # MODIFY: Add provider config and resolution
```

### Types to Update

```go
// internal/orchestrator/orchestrator.go

// Add to imports:
import (
	// ... existing imports ...
	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/provider"
)

// Config holds orchestrator-specific configuration
type Config struct {
	// ... existing fields ...

	// Parallelism is the maximum concurrent units
	Parallelism int

	// TargetBranch is the branch PRs target
	TargetBranch string

	// TasksDir is the path to the specs/tasks directory
	TasksDir string

	// RepoRoot is the path to the repository root
	RepoRoot string

	// WorktreeBase is the path where worktrees are created
	WorktreeBase string

	// NoPR skips PR creation when true
	NoPR bool

	// SkipReview auto-merges without waiting for review
	SkipReview bool

	// SingleUnit limits execution to one unit when non-empty
	SingleUnit string

	// DryRun shows execution plan without running
	DryRun bool

	// ShutdownTimeout is the grace period for worker cleanup
	ShutdownTimeout time.Duration

	// SuppressOutput disables stdout/stderr tee in workers (TUI mode)
	SuppressOutput bool

	// DefaultProvider is the provider type from --provider flag
	// Used when unit frontmatter doesn't specify a provider
	DefaultProvider string

	// ForceTaskProvider overrides all provider selection when non-empty
	// Set via --force-task-provider CLI flag
	ForceTaskProvider string

	// ProviderConfig contains provider-specific settings from .choo.yaml
	// Includes provider type default and per-provider command overrides
	ProviderConfig config.ProviderConfig
}
```

### Functions to Add

```go
// internal/orchestrator/orchestrator.go

// resolveProviderForUnit determines which provider to use for a unit
// Precedence (highest to lowest):
// 1. --force-task-provider flag
// 2. Unit frontmatter provider field
// 3. --provider CLI flag (stored in DefaultProvider)
// 4. RALPH_PROVIDER env var (merged into ProviderConfig during config loading)
// 5. .choo.yaml provider.type (in ProviderConfig)
// 6. Default: claude
func (o *Orchestrator) resolveProviderForUnit(unit *discovery.Unit) (provider.Provider, error) {
	var providerType provider.ProviderType

	// 1. --force-task-provider overrides everything
	if o.cfg.ForceTaskProvider != "" {
		providerType = provider.ProviderType(o.cfg.ForceTaskProvider)
	} else if unit.Frontmatter.Provider != "" {
		// 2. Per-unit frontmatter
		providerType = provider.ProviderType(unit.Frontmatter.Provider)
	} else if o.cfg.DefaultProvider != "" {
		// 3. --provider CLI flag
		providerType = provider.ProviderType(o.cfg.DefaultProvider)
	} else if o.cfg.ProviderConfig.Type != "" {
		// 4-5. Env var and .choo.yaml (merged during config loading)
		providerType = provider.ProviderType(o.cfg.ProviderConfig.Type)
	} else {
		// 6. Default to claude
		providerType = provider.ProviderClaude
	}

	// Get provider-specific command override if configured
	command := ""
	if o.cfg.ProviderConfig.Providers != nil {
		if settings, ok := o.cfg.ProviderConfig.Providers[string(providerType)]; ok {
			command = settings.Command
		}
	}

	return provider.FromConfig(provider.Config{
		Type:    providerType,
		Command: command,
	})
}

// createProviderFactory returns a factory function that resolves providers for units
func (o *Orchestrator) createProviderFactory() worker.ProviderFactory {
	return func(unit *discovery.Unit) (provider.Provider, error) {
		return o.resolveProviderForUnit(unit)
	}
}
```

### Update Run Method

Update the Run method to use the provider factory:

```go
// internal/orchestrator/orchestrator.go

// Run executes the orchestration loop until completion or cancellation
func (o *Orchestrator) Run(ctx context.Context) (*Result, error) {
	// ... existing discovery and scheduling code ...

	// 3. Initialize worker pool with provider factory
	workerCfg := worker.WorkerConfig{
		RepoRoot:            o.cfg.RepoRoot,
		TargetBranch:        o.cfg.TargetBranch,
		WorktreeBase:        o.cfg.WorktreeBase,
		NoPR:                o.cfg.NoPR,
		BackpressureTimeout: 5 * time.Minute,
		MaxClaudeRetries:    3,
		SuppressOutput:      o.cfg.SuppressOutput,
	}

	workerDeps := worker.WorkerDeps{
		Events: o.bus,
		Git:    o.git,
		GitHub: o.github,
		// Note: Provider is not set here - factory handles per-unit resolution
	}

	// Use factory-based pool construction for per-unit provider resolution
	o.pool = worker.NewPoolWithFactory(
		o.cfg.Parallelism,
		workerCfg,
		workerDeps,
		o.createProviderFactory(),
	)

	// ... rest of Run method unchanged ...
}
```

### Add Test

```go
// internal/orchestrator/orchestrator_test.go

func TestResolveProviderForUnit_Precedence(t *testing.T) {
	tests := []struct {
		name           string
		cfg            Config
		unitProvider   string
		expectedType   provider.ProviderType
	}{
		{
			name: "force_overrides_all",
			cfg: Config{
				ForceTaskProvider: "codex",
				DefaultProvider:   "claude",
				ProviderConfig: config.ProviderConfig{
					Type: "claude",
				},
			},
			unitProvider: "claude",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "frontmatter_overrides_default",
			cfg: Config{
				DefaultProvider: "claude",
			},
			unitProvider: "codex",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "default_used_when_no_frontmatter",
			cfg: Config{
				DefaultProvider: "codex",
			},
			unitProvider: "",
			expectedType: provider.ProviderCodex,
		},
		{
			name: "config_used_when_no_default",
			cfg: Config{
				ProviderConfig: config.ProviderConfig{
					Type: "codex",
				},
			},
			unitProvider: "",
			expectedType: provider.ProviderCodex,
		},
		{
			name:         "claude_is_fallback",
			cfg:          Config{},
			unitProvider: "",
			expectedType: provider.ProviderClaude,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := Dependencies{Bus: events.NewBus(100)}
			o := New(tt.cfg, deps)

			unit := &discovery.Unit{
				ID: "test-unit",
				Frontmatter: discovery.UnitFrontmatter{
					Provider: tt.unitProvider,
				},
			}

			prov, err := o.resolveProviderForUnit(unit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prov.Name() != tt.expectedType {
				t.Errorf("expected %s, got %s", tt.expectedType, prov.Name())
			}
		})
	}
}

func TestResolveProviderForUnit_CommandOverride(t *testing.T) {
	cfg := Config{
		DefaultProvider: "codex",
		ProviderConfig: config.ProviderConfig{
			Providers: map[string]config.ProviderSettings{
				"codex": {
					Command: "/custom/path/to/codex",
				},
			},
		},
	}
	deps := Dependencies{Bus: events.NewBus(100)}
	o := New(cfg, deps)

	unit := &discovery.Unit{ID: "test-unit"}

	prov, err := o.resolveProviderForUnit(unit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Provider should be created successfully with custom command
	if prov.Name() != provider.ProviderCodex {
		t.Errorf("expected codex, got %s", prov.Name())
	}
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/orchestrator/... -run Provider
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Precedence test passes | Force > frontmatter > default > config > claude |
| Command override works | Custom command passed to provider |
| Factory creation works | createProviderFactory returns valid factory |
| Pool uses factory | Run method uses NewPoolWithFactory |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Add three new fields to Config: `DefaultProvider`, `ForceTaskProvider`, `ProviderConfig`
- Import the `config` package for `ProviderConfig` type
- Import the `provider` package for `Provider`, `ProviderType`, `FromConfig`
- Implement `resolveProviderForUnit` with the five-level precedence chain
- Create `createProviderFactory` helper that returns a closure over `resolveProviderForUnit`
- Update the `Run` method to use `NewPoolWithFactory` instead of `NewPool`
- Provider resolution happens once per unit at dispatch time, not per task
- Provider errors fail fast at Submit time rather than during task execution

## NOT In Scope

- CLI flag parsing (handled by CLI package)
- Config file loading (handled by config package)
- Provider implementation (handled by provider package)
- Event bus changes
