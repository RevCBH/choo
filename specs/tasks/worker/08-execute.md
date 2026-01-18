---
task: 8
status: pending
backpressure: "go test ./internal/worker/..."
depends_on: [6, 7]
---

# Public Execute Entry Point

**Parent spec**: `/specs/WORKER.md`
**Task**: #8 of 8 in implementation plan

## Objective

Implement the public Execute() function that serves as the primary entry point for single-unit execution, wrapping the Worker creation and run phases.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Unit`
- All worker dependencies (events, git, github, claude)

### Task Dependencies (within this unit)
- Task #6 (worker) - provides `NewWorker`, `Worker.Run()`
- Task #7 (pool) - ensures pool is available for parallel scenarios

### Package Dependencies
- `context` - for cancellation

## Deliverables

### Files to Create

```
internal/worker/
└── execute.go      # Public Execute() entry point
```

### Functions to Implement

```go
// Execute runs a single unit to completion (convenience wrapper)
// This is the primary entry point for single-unit execution
func Execute(ctx context.Context, unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) error {
    // 1. Validate inputs
    // 2. Create worker
    // 3. Run worker
    // 4. Return result
}

// ExecuteWithDefaults runs a unit with sensible default configuration
func ExecuteWithDefaults(ctx context.Context, unit *discovery.Unit, deps WorkerDeps) error {
    // 1. Create default config
    // 2. Call Execute with defaults
}

// DefaultConfig returns a WorkerConfig with sensible defaults
func DefaultConfig() WorkerConfig {
    return WorkerConfig{
        MaxClaudeRetries:    3,
        MaxBaselineRetries:  3,
        BackpressureTimeout: 5 * time.Minute,
        BaselineTimeout:     10 * time.Minute,
        WorktreeBase:        "/tmp/ralph-worktrees",
        TargetBranch:        "main",
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestExecute_Success` | Returns nil on successful execution |
| `TestExecute_NilUnit` | Returns error for nil unit |
| `TestExecute_PropagatesError` | Returns worker error |
| `TestExecute_RespectsContext` | Cancellation stops execution |
| `TestExecuteWithDefaults` | Uses default config values |
| `TestDefaultConfig` | All fields have sensible values |

### Test Implementation

```go
func TestExecute_Success(t *testing.T) {
    deps := mockDeps(t)
    unit := &discovery.Unit{
        ID:    "test",
        Tasks: []*discovery.Task{},
    }

    err := Execute(context.Background(), unit, DefaultConfig(), deps)

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}

func TestExecute_NilUnit(t *testing.T) {
    deps := mockDeps(t)

    err := Execute(context.Background(), nil, DefaultConfig(), deps)

    if err == nil {
        t.Error("expected error for nil unit")
    }
}

func TestExecute_PropagatesError(t *testing.T) {
    deps := mockDeps(t)
    deps.Git = &mockGitManager{
        createWorktreeErr: errors.New("git failed"),
    }

    unit := &discovery.Unit{ID: "test"}

    err := Execute(context.Background(), unit, DefaultConfig(), deps)

    if err == nil {
        t.Error("expected error to propagate")
    }
    if !strings.Contains(err.Error(), "git failed") {
        t.Errorf("expected git error, got: %v", err)
    }
}

func TestExecute_RespectsContext(t *testing.T) {
    deps := mockDeps(t)

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    unit := &discovery.Unit{ID: "test"}

    err := Execute(ctx, unit, DefaultConfig(), deps)

    if err == nil {
        t.Error("expected context cancellation error")
    }
}

func TestExecuteWithDefaults(t *testing.T) {
    deps := mockDeps(t)
    unit := &discovery.Unit{
        ID:    "test",
        Tasks: []*discovery.Task{},
    }

    err := ExecuteWithDefaults(context.Background(), unit, deps)

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}

func TestDefaultConfig(t *testing.T) {
    cfg := DefaultConfig()

    if cfg.MaxClaudeRetries != 3 {
        t.Errorf("expected MaxClaudeRetries=3, got %d", cfg.MaxClaudeRetries)
    }
    if cfg.MaxBaselineRetries != 3 {
        t.Errorf("expected MaxBaselineRetries=3, got %d", cfg.MaxBaselineRetries)
    }
    if cfg.BackpressureTimeout != 5*time.Minute {
        t.Errorf("expected BackpressureTimeout=5m, got %v", cfg.BackpressureTimeout)
    }
    if cfg.BaselineTimeout != 10*time.Minute {
        t.Errorf("expected BaselineTimeout=10m, got %v", cfg.BaselineTimeout)
    }
    if cfg.TargetBranch != "main" {
        t.Errorf("expected TargetBranch=main, got %s", cfg.TargetBranch)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses mocks)
- [x] Runs in <60 seconds

## Implementation Notes

- Execute() is a thin wrapper that validates inputs and delegates to Worker
- Input validation should catch common errors before worker creation
- Context cancellation should be respected throughout
- DefaultConfig values match performance requirements from spec

## NOT In Scope

- CLI argument parsing (handled by cmd package)
- Configuration file loading
- Logging setup
