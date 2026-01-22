---
task: 1
status: complete
backpressure: "go test ./internal/orchestrator/... -run TestResolveReviewer"
depends_on: []
---

# Reviewer Resolution and Injection

**Parent spec**: `/docs/prd/CODE-REVIEW.md`
**Task**: #1 of 2 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement `resolveReviewer()` in the orchestrator to create the appropriate Reviewer based on configuration, and inject it into Worker construction via WorkerDeps.

## Dependencies

### External Specs (must be implemented)
- **review-config** - provides `CodeReviewConfig` with Enabled, Provider, Command fields
- **codex-reviewer** - provides `NewCodexReviewer(command string) *CodexReviewer`
- **claude-reviewer** - provides `NewClaudeReviewer(command string) *ClaudeReviewer`
- **review-worker** - provides `Reviewer` field in `WorkerDeps` struct

### Task Dependencies (within this unit)
- None (this is the first task)

### Package Dependencies
- `github.com/RevCBH/choo/internal/config`
- `github.com/RevCBH/choo/internal/provider`
- `github.com/RevCBH/choo/internal/worker`

## Deliverables

### Files to Create/Modify

```
internal/orchestrator/
├── orchestrator.go       # MODIFY: Add resolveReviewer(), update createProviderFactory()
└── orchestrator_test.go  # MODIFY: Add reviewer resolution tests
```

### Types to Use

The following types are provided by dependent units:

```go
// From internal/config/config.go (review-config unit)
type CodeReviewConfig struct {
    Enabled          bool               `yaml:"enabled"`
    Provider         ReviewProviderType `yaml:"provider"`
    MaxFixIterations int                `yaml:"max_fix_iterations"`
    Command          string             `yaml:"command,omitempty"`
}

// From internal/provider/reviewer.go (codex-reviewer, claude-reviewer units)
type Reviewer interface {
    Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
    Name() ProviderType
}

// From internal/worker/worker.go (review-worker unit)
type WorkerDeps struct {
    Events    *events.Bus
    Git       *git.WorktreeManager
    GitRunner git.Runner
    GitHub    *github.PRClient
    Provider  provider.Provider
    Escalator escalate.Escalator
    MergeMu   *sync.Mutex
    Reviewer  provider.Reviewer  // Added by review-worker unit
}
```

### Functions to Implement

```go
// Add to internal/orchestrator/orchestrator.go

// resolveReviewer creates the appropriate Reviewer based on configuration.
// Returns nil if code review is disabled (not an error).
// Uses the same resolution pattern as resolveProviderForUnit but for reviewers.
func (o *Orchestrator) resolveReviewer() (provider.Reviewer, error) {
    cfg := o.cfg.CodeReview

    // If code review is disabled, return nil (no reviewer)
    if !cfg.Enabled {
        return nil, nil
    }

    // Determine reviewer type from config
    reviewerType := cfg.Provider
    if reviewerType == "" {
        reviewerType = config.ReviewProviderCodex // Default to codex
    }

    // Create the appropriate reviewer
    switch reviewerType {
    case config.ReviewProviderCodex:
        return provider.NewCodexReviewer(cfg.Command), nil
    case config.ReviewProviderClaude:
        return provider.NewClaudeReviewer(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown review provider: %s", reviewerType)
    }
}
```

### Modifications to Existing Structs

Add `CodeReview` field to orchestrator.Config struct:

```go
// In internal/orchestrator/orchestrator.go, add to Config struct:
type Config struct {
    // ... existing fields ...

    // CodeReview contains code review configuration from .choo.yaml
    CodeReview config.CodeReviewConfig
}
```

### Modifications to Worker Pool

The Pool needs to store and pass the reviewer to workers. Modify `internal/worker/pool.go`:

```go
// In Pool struct, add:
type Pool struct {
    // ... existing fields ...
    reviewer provider.Reviewer // Shared reviewer for code review (may be nil)
}

// Update NewPoolWithFactory to accept reviewer:
func NewPoolWithFactory(maxWorkers int, cfg WorkerConfig, deps WorkerDeps, factory ProviderFactory) *Pool {
    ctx, cancel := context.WithCancel(context.Background())
    return &Pool{
        // ... existing fields ...
        reviewer: deps.Reviewer, // Store reviewer from deps
    }
}

// In Pool.Submit(), add reviewer to worker deps:
worker := NewWorker(unit, p.config, WorkerDeps{
    Events:   p.events,
    Git:      p.git,
    GitHub:   p.github,
    Provider: prov,
    MergeMu:  &p.mergeMu,
    Reviewer: p.reviewer, // Pass reviewer to worker
})
```

### Modifications to Run()

Update `Run()` in orchestrator.go to resolve the reviewer:

```go
// In internal/orchestrator/orchestrator.go, within Run() method
// After creating workerCfg, before creating the pool:

// Resolve reviewer for code review (may be nil if disabled)
reviewer, err := o.resolveReviewer()
if err != nil {
    return nil, fmt.Errorf("failed to resolve reviewer: %w", err)
}

workerDeps := worker.WorkerDeps{
    Events:   o.bus,
    Git:      o.git,
    GitHub:   o.github,
    Reviewer: reviewer, // Pass reviewer to pool
    // Note: Provider is not set here - factory handles per-unit resolution
    // Note: MergeMu is managed by the Pool internally, not passed here
}
```

**Note**: The `mergeMu` mutex is already managed internally by `worker.Pool` (see pool.go:29). The orchestrator does NOT need its own merge mutex.

### Tests to Implement

```go
// Add to internal/orchestrator/orchestrator_test.go

func TestResolveReviewer_Disabled(t *testing.T) {
    orch := &Orchestrator{
        cfg: Config{
            CodeReview: config.CodeReviewConfig{
                Enabled: false,
            },
        },
    }

    reviewer, err := orch.resolveReviewer()
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if reviewer != nil {
        t.Error("expected nil reviewer when disabled")
    }
}

func TestResolveReviewer_Codex(t *testing.T) {
    orch := &Orchestrator{
        cfg: Config{
            CodeReview: config.CodeReviewConfig{
                Enabled:  true,
                Provider: config.ReviewProviderCodex,
                Command:  "/custom/codex",
            },
        },
    }

    reviewer, err := orch.resolveReviewer()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if reviewer == nil {
        t.Fatal("expected non-nil reviewer")
    }
    if reviewer.Name() != provider.ProviderCodex {
        t.Errorf("expected codex reviewer, got %s", reviewer.Name())
    }
}

func TestResolveReviewer_Claude(t *testing.T) {
    orch := &Orchestrator{
        cfg: Config{
            CodeReview: config.CodeReviewConfig{
                Enabled:  true,
                Provider: config.ReviewProviderClaude,
            },
        },
    }

    reviewer, err := orch.resolveReviewer()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if reviewer == nil {
        t.Fatal("expected non-nil reviewer")
    }
    if reviewer.Name() != provider.ProviderClaude {
        t.Errorf("expected claude reviewer, got %s", reviewer.Name())
    }
}

func TestResolveReviewer_DefaultToCodex(t *testing.T) {
    orch := &Orchestrator{
        cfg: Config{
            CodeReview: config.CodeReviewConfig{
                Enabled:  true,
                Provider: "", // Empty defaults to codex
            },
        },
    }

    reviewer, err := orch.resolveReviewer()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if reviewer == nil {
        t.Fatal("expected non-nil reviewer")
    }
    if reviewer.Name() != provider.ProviderCodex {
        t.Errorf("expected codex reviewer (default), got %s", reviewer.Name())
    }
}

func TestResolveReviewer_UnknownProvider(t *testing.T) {
    orch := &Orchestrator{
        cfg: Config{
            CodeReview: config.CodeReviewConfig{
                Enabled:  true,
                Provider: "unknown-provider",
            },
        },
    }

    _, err := orch.resolveReviewer()
    if err == nil {
        t.Error("expected error for unknown provider")
    }
    if !strings.Contains(err.Error(), "unknown review provider") {
        t.Errorf("unexpected error message: %v", err)
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/orchestrator/... -run TestResolveReviewer -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestResolveReviewer_Disabled` | Returns nil reviewer without error when disabled |
| `TestResolveReviewer_Codex` | Creates CodexReviewer with custom command |
| `TestResolveReviewer_Claude` | Creates ClaudeReviewer |
| `TestResolveReviewer_DefaultToCodex` | Defaults to codex when provider not specified |
| `TestResolveReviewer_UnknownProvider` | Returns error for unknown provider type |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The `resolveReviewer()` method follows the same pattern as `resolveProviderForUnit()` but is simpler since reviewers don't have per-unit overrides
- The reviewer is resolved once at orchestrator startup and shared across all workers (unlike providers which are resolved per-unit)
- The `mergeMu` mutex is already managed internally by `worker.Pool` - do NOT add another mutex to the orchestrator
- The Pool stores the reviewer and passes it to each Worker via WorkerDeps during Submit()
- The reviewer may be nil if code review is disabled - workers must handle this case gracefully

## NOT In Scope

- The actual `runCodeReview()` implementation (provided by review-worker unit)
- Worker integration of the review call (task #2)
- Reviewer implementations (provided by codex-reviewer, claude-reviewer units)
- Configuration types (provided by review-config unit)
