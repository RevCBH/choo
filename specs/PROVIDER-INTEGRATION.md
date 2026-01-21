# PROVIDER-INTEGRATION — Worker and Orchestrator Changes for Provider Abstraction

## Overview

The PROVIDER-INTEGRATION spec covers how the worker loop and orchestrator integrate with the new Provider abstraction for task execution. This replaces the hardcoded Claude CLI invocation with a configurable provider system.

Currently, the worker directly invokes Claude via subprocess in `invokeClaudeForTask()`. This spec introduces a provider resolution flow where the orchestrator determines which provider to use for each unit based on a five-level precedence chain, then passes that provider to the worker for task execution.

The scope is intentionally limited to task execution inner loops. Other LLM operations (merge conflict resolution, branch naming, PR creation) remain explicitly tied to Claude and are out of scope for this spec.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Provider Integration Flow                            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Configuration Resolution (highest to lowest precedence):                    │
│                                                                              │
│   ┌─────────────────────────┐                                                │
│   │ --force-task-provider   │ ──► Overrides everything                       │
│   └───────────┬─────────────┘                                                │
│               ▼                                                               │
│   ┌─────────────────────────┐                                                │
│   │ Per-unit frontmatter    │ ──► Unit author specifies provider             │
│   └───────────┬─────────────┘                                                │
│               ▼                                                               │
│   ┌─────────────────────────┐                                                │
│   │ --provider CLI flag     │ ──► Default for units without override         │
│   └───────────┬─────────────┘                                                │
│               ▼                                                               │
│   ┌─────────────────────────┐                                                │
│   │ RALPH_PROVIDER env var  │ ──► Environment-level default                  │
│   └───────────┬─────────────┘                                                │
│               ▼                                                               │
│   ┌─────────────────────────┐                                                │
│   │ .choo.yaml provider     │ ──► Config file fallback                       │
│   └───────────┬─────────────┘                                                │
│               ▼                                                               │
│   ┌─────────────────────────┐                                                │
│   │ Default: claude         │ ──► Backward compatible default                │
│   └─────────────────────────┘                                                │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Orchestrator │────▶│   Worker     │────▶│   Provider   │
│              │     │              │     │              │
│ resolves     │     │ receives     │     │ invokes      │
│ provider for │     │ provider     │     │ CLI tool     │
│ each unit    │     │ instance     │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
```

## Requirements

### Functional Requirements

1. Orchestrator resolves provider for each unit before dispatching to worker pool
2. Resolution follows five-level precedence: `--force-task-provider` > frontmatter > `--provider` > env var > config
3. Worker receives a `Provider` instance instead of using hardcoded Claude invocation
4. `invokeClaudeForTask()` is renamed to `invokeProvider()` and delegates to the Provider interface
5. Worker pool passes provider configuration to workers during submission
6. Empty or missing provider configuration defaults to `claude` provider
7. Existing behavior remains unchanged when no provider configuration is specified
8. Provider resolution occurs once per unit at dispatch time, not per-task

### Performance Requirements

| Metric | Target |
|--------|--------|
| Provider resolution overhead | <1ms per unit |
| Memory overhead per Provider instance | <1KB |
| Provider factory instantiation | <100us |

### Constraints

- Depends on PROVIDER spec (interface and implementations)
- Depends on CONFIG spec (ProviderConfig type and loading)
- Depends on CLI spec (--provider and --force-task-provider flags)
- Must maintain backward compatibility with existing worker invocation patterns
- Provider instances must be safe for concurrent use (though typically one per worker)

## Design

### Module Structure

```
internal/
├── orchestrator/
│   └── orchestrator.go    # Add provider resolution, pass to workers
├── worker/
│   ├── worker.go          # Replace ClaudeClient with Provider
│   ├── loop.go            # Rename invokeClaudeForTask → invokeProvider
│   └── pool.go            # Pass provider configuration to workers
└── provider/              # (from PROVIDER spec)
    ├── provider.go        # Provider interface
    ├── claude.go          # Claude implementation
    ├── codex.go           # Codex implementation
    └── factory.go         # FromConfig factory
```

### Core Types

```go
// internal/orchestrator/orchestrator.go

// Config holds orchestrator-specific configuration
type Config struct {
    // ... existing fields ...

    // DefaultProvider is the provider type from --provider flag or config
    DefaultProvider string

    // ForceTaskProvider overrides all provider selection when non-empty
    ForceTaskProvider string

    // ProviderConfig contains provider-specific settings from .choo.yaml
    ProviderConfig config.ProviderConfig
}
```

```go
// internal/worker/worker.go

// Worker executes a single unit in an isolated worktree
type Worker struct {
    unit         *discovery.Unit
    config       WorkerConfig
    events       *events.Bus
    git          *git.WorktreeManager
    gitRunner    git.Runner
    github       *github.PRClient
    provider     provider.Provider  // NEW: replaces ClaudeClient
    escalator    escalate.Escalator
    worktreePath string
    branch       string
    currentTask  *discovery.Task
    prNumber     int
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
    Events    *events.Bus
    Git       *git.WorktreeManager
    GitRunner git.Runner
    GitHub    *github.PRClient
    Provider  provider.Provider  // NEW: replaces Claude
    Escalator escalate.Escalator
}
```

```go
// internal/worker/pool.go

// Pool manages a collection of workers executing units in parallel
type Pool struct {
    maxWorkers      int
    config          WorkerConfig
    events          *events.Bus
    git             *git.WorktreeManager
    github          *github.PRClient
    providerFactory func(*discovery.Unit) (provider.Provider, error)  // NEW
    workers         map[string]*Worker
    mu              sync.Mutex
    wg              sync.WaitGroup
    sem             chan struct{}
    firstErr        error
    cancelCtx       context.Context
    cancelFunc      context.CancelFunc
}

// PoolConfig configures the worker pool
type PoolConfig struct {
    WorkerConfig    WorkerConfig
    ProviderFactory func(*discovery.Unit) (provider.Provider, error)
}
```

### API Surface

```go
// internal/orchestrator/orchestrator.go

// resolveProviderForUnit determines which provider to use for a unit
func (o *Orchestrator) resolveProviderForUnit(unit *discovery.Unit) (provider.Provider, error)

// New creates an orchestrator with the given configuration and dependencies
// (signature unchanged, but Config struct has new fields)
func New(cfg Config, deps Dependencies) *Orchestrator
```

```go
// internal/worker/worker.go

// NewWorker creates a worker for executing a unit
// Provider is now passed via WorkerDeps instead of being created internally
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker
```

```go
// internal/worker/loop.go

// invokeProvider runs the configured provider with the task prompt
// Replaces invokeClaudeForTask
func (w *Worker) invokeProvider(ctx context.Context, prompt TaskPrompt) error
```

```go
// internal/worker/pool.go

// NewPoolWithFactory creates a worker pool with a custom provider factory
func NewPoolWithFactory(maxWorkers int, cfg WorkerConfig, deps WorkerDeps, factory func(*discovery.Unit) (provider.Provider, error)) *Pool

// Submit queues a unit for execution, resolving its provider via the factory
func (p *Pool) Submit(unit *discovery.Unit) error
```

### Provider Resolution Algorithm

```go
// internal/orchestrator/orchestrator.go

func (o *Orchestrator) resolveProviderForUnit(unit *discovery.Unit) (provider.Provider, error) {
    var providerType provider.ProviderType

    // 1. --force-task-provider overrides everything
    if o.cfg.ForceTaskProvider != "" {
        providerType = provider.ProviderType(o.cfg.ForceTaskProvider)
    } else if unit.Frontmatter.Provider != "" {
        // 2. Per-unit frontmatter
        providerType = provider.ProviderType(unit.Frontmatter.Provider)
    } else if o.cfg.DefaultProvider != "" {
        // 3. --provider CLI flag (already merged with env var in config loading)
        providerType = provider.ProviderType(o.cfg.DefaultProvider)
    } else if o.cfg.ProviderConfig.Type != "" {
        // 4-5. Env var and .choo.yaml (handled during config loading)
        providerType = provider.ProviderType(o.cfg.ProviderConfig.Type)
    } else {
        // 6. Default to claude
        providerType = provider.ProviderClaude
    }

    // Get provider-specific command override if configured
    command := ""
    if settings, ok := o.cfg.ProviderConfig.Providers[string(providerType)]; ok {
        command = settings.Command
    }

    return provider.FromConfig(provider.Config{
        Type:    providerType,
        Command: command,
    })
}
```

### Worker Integration

```go
// internal/worker/loop.go

// invokeProvider runs the configured provider with the task prompt
// CRITICAL: Uses subprocess, NEVER direct API calls
func (w *Worker) invokeProvider(ctx context.Context, prompt TaskPrompt) error {
    // Create log file for provider output
    logDir := filepath.Join(w.config.WorktreeBase, "logs")
    if err := os.MkdirAll(logDir, 0755); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
    }

    logFile, err := os.Create(filepath.Join(logDir,
        fmt.Sprintf("%s-%s-%d.log", w.provider.Name(), w.unit.ID, time.Now().Unix())))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to create log file: %v\n", err)
        // Fall back to stdout/stderr
        if !w.config.SuppressOutput {
            return w.provider.Invoke(ctx, prompt.Content, w.worktreePath, os.Stdout, os.Stderr)
        }
        return w.provider.Invoke(ctx, prompt.Content, w.worktreePath, io.Discard, io.Discard)
    }
    defer logFile.Close()

    // Write prompt to log file
    fmt.Fprintf(logFile, "=== PROMPT ===\n%s\n=== END PROMPT ===\n\n", prompt.Content)
    fmt.Fprintf(logFile, "=== PROVIDER: %s ===\n", w.provider.Name())

    // Configure output writers
    var stdout, stderr io.Writer
    if w.config.SuppressOutput {
        stdout = logFile
        stderr = logFile
    } else {
        stdout = io.MultiWriter(os.Stdout, logFile)
        stderr = io.MultiWriter(os.Stderr, logFile)
        fmt.Fprintf(os.Stderr, "Provider output logging to: %s\n", logFile.Name())
    }

    // Emit TaskClaudeInvoke event (name unchanged for backward compatibility)
    if w.events != nil {
        evt := events.NewEvent(events.TaskClaudeInvoke, w.unit.ID)
        if w.currentTask != nil {
            evt = evt.WithTask(w.currentTask.Number).WithPayload(map[string]any{
                "title":    w.currentTask.Title,
                "provider": string(w.provider.Name()),
            })
        }
        w.events.Emit(evt)
    }

    // Invoke provider
    runErr := w.provider.Invoke(ctx, prompt.Content, w.worktreePath, stdout, stderr)

    // Write completion status to log
    fmt.Fprintf(logFile, "\n=== END PROVIDER OUTPUT ===\n")
    if runErr != nil {
        fmt.Fprintf(logFile, "Exit error: %v\n", runErr)
    } else {
        fmt.Fprintf(logFile, "Exit: success\n")
    }

    // Emit TaskClaudeDone event (name unchanged for backward compatibility)
    if w.events != nil {
        evt := events.NewEvent(events.TaskClaudeDone, w.unit.ID)
        if w.currentTask != nil {
            evt = evt.WithTask(w.currentTask.Number)
        }
        if runErr != nil {
            evt = evt.WithError(runErr)
        }
        w.events.Emit(evt)
    }

    return runErr
}
```

### Pool Factory Pattern

```go
// internal/worker/pool.go

// NewPoolWithFactory creates a worker pool with a custom provider factory
func NewPoolWithFactory(maxWorkers int, cfg WorkerConfig, deps WorkerDeps,
    factory func(*discovery.Unit) (provider.Provider, error)) *Pool {

    ctx, cancel := context.WithCancel(context.Background())
    return &Pool{
        maxWorkers:      maxWorkers,
        config:          cfg,
        events:          deps.Events,
        git:             deps.Git,
        github:          deps.GitHub,
        providerFactory: factory,
        workers:         make(map[string]*Worker),
        sem:             make(chan struct{}, maxWorkers),
        cancelCtx:       ctx,
        cancelFunc:      cancel,
    }
}

// Submit queues a unit for execution
func (p *Pool) Submit(unit *discovery.Unit) error {
    p.mu.Lock()
    if _, exists := p.workers[unit.ID]; exists {
        p.mu.Unlock()
        return fmt.Errorf("unit %s already submitted", unit.ID)
    }

    // Resolve provider for this unit
    prov, err := p.providerFactory(unit)
    if err != nil {
        p.mu.Unlock()
        return fmt.Errorf("failed to create provider for unit %s: %w", unit.ID, err)
    }

    // Create worker with resolved provider
    worker := NewWorker(unit, p.config, WorkerDeps{
        Events:   p.events,
        Git:      p.git,
        GitHub:   p.github,
        Provider: prov,
    })

    p.workers[unit.ID] = worker
    p.mu.Unlock()

    // ... rest of submission logic unchanged ...
}
```

## Implementation Notes

### Backward Compatibility

The integration maintains backward compatibility through several mechanisms:

1. **Default provider is Claude** - When no provider configuration exists, the system defaults to Claude, preserving existing behavior.

2. **Event names unchanged** - `TaskClaudeInvoke` and `TaskClaudeDone` event types are preserved to avoid breaking event subscribers. The payload includes a `provider` field for consumers that need to distinguish.

3. **ClaudeConfig preserved** - The existing `ClaudeConfig` in the config package remains for any code that depends on it. The new `ProviderConfig` is additive.

4. **Empty config handling** - Empty `provider.type` in `.choo.yaml` falls back to Claude without error.

### Provider Instance Lifecycle

- Provider instances are created once per unit at dispatch time
- The same Provider instance is used for all tasks within a unit
- Providers should be stateless (no shared mutable state between invocations)
- Provider instances are not shared between workers

### Error Handling

Provider resolution errors should fail fast:

```go
func (o *Orchestrator) dispatchUnit(unit *discovery.Unit) error {
    prov, err := o.resolveProviderForUnit(unit)
    if err != nil {
        // Emit failure event and mark unit as failed
        o.bus.Emit(events.NewEvent(events.UnitFailed, unit.ID).WithError(err))
        o.scheduler.Fail(unit.ID, err)
        return err
    }
    // ... continue with dispatch
}
```

### Thread Safety

- Provider resolution in orchestrator is called from the main dispatch loop (single-threaded)
- Provider factory is called in pool.Submit which holds mutex
- Provider.Invoke is called from worker goroutines (one per unit, no sharing)

## Testing Strategy

### Unit Tests

```go
// internal/orchestrator/orchestrator_test.go

func TestResolveProviderForUnit_ForceTakesPrecedence(t *testing.T) {
    cfg := Config{
        ForceTaskProvider: "codex",
        DefaultProvider:   "claude",
        ProviderConfig: config.ProviderConfig{
            Type: "claude",
        },
    }
    deps := Dependencies{Bus: events.NewBus(100)}
    o := New(cfg, deps)

    unit := &discovery.Unit{
        ID: "test-unit",
        Frontmatter: discovery.UnitFrontmatter{
            Provider: "claude",  // Should be overridden
        },
    }

    prov, err := o.resolveProviderForUnit(unit)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if prov.Name() != provider.ProviderCodex {
        t.Errorf("expected codex, got %s", prov.Name())
    }
}

func TestResolveProviderForUnit_FrontmatterOverridesDefault(t *testing.T) {
    cfg := Config{
        DefaultProvider: "claude",
    }
    deps := Dependencies{Bus: events.NewBus(100)}
    o := New(cfg, deps)

    unit := &discovery.Unit{
        ID: "test-unit",
        Frontmatter: discovery.UnitFrontmatter{
            Provider: "codex",
        },
    }

    prov, err := o.resolveProviderForUnit(unit)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if prov.Name() != provider.ProviderCodex {
        t.Errorf("expected codex, got %s", prov.Name())
    }
}

func TestResolveProviderForUnit_DefaultsToClaudeWhenEmpty(t *testing.T) {
    cfg := Config{}  // No provider config
    deps := Dependencies{Bus: events.NewBus(100)}
    o := New(cfg, deps)

    unit := &discovery.Unit{ID: "test-unit"}

    prov, err := o.resolveProviderForUnit(unit)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if prov.Name() != provider.ProviderClaude {
        t.Errorf("expected claude, got %s", prov.Name())
    }
}
```

```go
// internal/worker/loop_test.go

func TestInvokeProvider_DelegatesToProvider(t *testing.T) {
    // Create mock provider
    invoked := false
    var capturedPrompt string
    var capturedWorkdir string

    mockProvider := &mockProvider{
        invokeFunc: func(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error {
            invoked = true
            capturedPrompt = prompt
            capturedWorkdir = workdir
            return nil
        },
        name: provider.ProviderClaude,
    }

    unit := &discovery.Unit{ID: "test-unit"}
    cfg := WorkerConfig{
        WorktreeBase:   t.TempDir(),
        SuppressOutput: true,
    }

    worker := NewWorker(unit, cfg, WorkerDeps{
        Provider: mockProvider,
    })
    worker.worktreePath = "/tmp/worktree"

    prompt := TaskPrompt{Content: "test prompt"}
    err := worker.invokeProvider(context.Background(), prompt)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if !invoked {
        t.Error("provider was not invoked")
    }

    if capturedPrompt != "test prompt" {
        t.Errorf("expected prompt 'test prompt', got %q", capturedPrompt)
    }

    if capturedWorkdir != "/tmp/worktree" {
        t.Errorf("expected workdir '/tmp/worktree', got %q", capturedWorkdir)
    }
}

type mockProvider struct {
    invokeFunc func(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error
    name       provider.ProviderType
}

func (m *mockProvider) Invoke(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error {
    return m.invokeFunc(ctx, prompt, workdir, stdout, stderr)
}

func (m *mockProvider) Name() provider.ProviderType {
    return m.name
}
```

```go
// internal/worker/pool_test.go

func TestPoolWithFactory_ResolvesProviderPerUnit(t *testing.T) {
    resolved := make(map[string]provider.ProviderType)
    var mu sync.Mutex

    factory := func(unit *discovery.Unit) (provider.Provider, error) {
        mu.Lock()
        defer mu.Unlock()
        provType := provider.ProviderClaude
        if unit.Frontmatter.Provider != "" {
            provType = provider.ProviderType(unit.Frontmatter.Provider)
        }
        resolved[unit.ID] = provType
        return &mockProvider{name: provType}, nil
    }

    cfg := WorkerConfig{WorktreeBase: t.TempDir()}
    deps := WorkerDeps{Events: events.NewBus(100)}

    pool := NewPoolWithFactory(2, cfg, deps, factory)

    // Submit units with different provider preferences
    unit1 := &discovery.Unit{ID: "unit1"}
    unit2 := &discovery.Unit{
        ID: "unit2",
        Frontmatter: discovery.UnitFrontmatter{
            Provider: "codex",
        },
    }

    _ = pool.Submit(unit1)
    _ = pool.Submit(unit2)

    // Verify factory was called with correct units
    mu.Lock()
    defer mu.Unlock()

    if resolved["unit1"] != provider.ProviderClaude {
        t.Errorf("unit1 expected claude, got %s", resolved["unit1"])
    }

    if resolved["unit2"] != provider.ProviderCodex {
        t.Errorf("unit2 expected codex, got %s", resolved["unit2"])
    }
}
```

### Integration Tests

| Scenario | Setup | Expected Behavior |
|----------|-------|-------------------|
| Default provider | No config, no flags | Claude is used for all units |
| CLI flag override | `--provider=codex` | Codex used for units without frontmatter |
| Force flag override | `--force-task-provider=codex` | Codex used for all units, ignoring frontmatter |
| Per-unit frontmatter | Unit with `provider: codex` | That unit uses Codex, others use default |
| Mixed providers | Multiple units with different providers | Each unit uses its resolved provider |
| Invalid provider | `--provider=invalid` | Error at startup, not per-unit |

### Manual Testing

- [ ] Run `choo run` with no provider config, verify Claude is invoked
- [ ] Run `choo run --provider=codex`, verify Codex is invoked for tasks
- [ ] Add `provider: codex` to a unit's IMPLEMENTATION_PLAN.md frontmatter
- [ ] Run without flags, verify that unit uses Codex while others use Claude
- [ ] Run `choo run --force-task-provider=claude`, verify it overrides per-unit setting
- [ ] Check log files include correct provider name in header
- [ ] Verify events include `provider` field in payload

## Design Decisions

### Why factory pattern for provider resolution?

The orchestrator needs to resolve providers per-unit, but the worker pool handles worker creation. A factory function allows the pool to defer provider creation while the orchestrator maintains control over resolution logic.

Alternatives considered:
- Pass resolved providers map to pool: Would require orchestrator to know all units upfront
- Resolve in worker constructor: Would duplicate resolution logic or require passing full config

### Why keep event names unchanged?

Changing `TaskClaudeInvoke` to `TaskProviderInvoke` would break existing event subscribers (TUI, web dashboard, logs). The provider name is included in the event payload for consumers that need it.

The trade-off is slightly misleading event names when using non-Claude providers. This is documented as a known limitation and can be addressed in a future UX update.

### Why resolve provider once per unit, not per task?

Provider switching mid-unit would add complexity and has unclear semantics. If a unit starts with Claude, it should complete with Claude. This also matches the mental model that a unit is an atomic work item.

### Why use Provider interface instead of config-based switching?

An interface allows:
- Clean testing with mock providers
- Easy addition of new providers without modifying worker code
- Provider-specific behavior encapsulation

Config-based switching would require the worker to know about all provider types and their invocation patterns.

## Future Enhancements

1. **Provider health checks** - Verify provider CLI is available before starting execution
2. **Provider fallback chain** - Try alternative provider if primary fails
3. **Per-task provider selection** - Allow tasks within a unit to use different providers
4. **Provider metrics** - Track success rates and latency per provider
5. **Dynamic provider loading** - Plugin system for custom providers

## References

- [Multi-Provider PRD](/Users/bennett/conductor/workspaces/choo/san-jose/docs/MULTI-PROVIDER-PRD.md) - Product requirements document
- [PROVIDER spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/PROVIDER.md) - Provider interface and implementations (dependency)
- [CONFIG spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/completed/CONFIG.md) - Configuration loading
- [WORKER spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/completed/WORKER.md) - Worker implementation details
- [ORCHESTRATOR spec](/Users/bennett/conductor/workspaces/choo/san-jose/specs/completed/ORCHESTRATOR.md) - Orchestrator implementation details
