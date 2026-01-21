---
task: 3
status: complete
backpressure: "go test ./internal/worker/... -run Pool"
depends_on: [1, 2]
---

# Add Provider Factory to Pool

**Parent spec**: `/specs/PROVIDER-INTEGRATION.md`
**Task**: #3 of 4 in implementation plan

## Objective

Add a provider factory function to the worker Pool that creates provider instances per-unit. This enables the orchestrator to inject provider resolution logic while keeping the Pool generic.

## Dependencies

### External Specs (must be implemented)
- PROVIDER - provides `Provider` interface

### Task Dependencies (within this unit)
- Task #1 (01-worker-types.md) - Worker accepts Provider in deps
- Task #2 (02-invoke-provider.md) - Worker uses provider for invocation

### Package Dependencies
- `github.com/RevCBH/choo/internal/provider`
- `github.com/RevCBH/choo/internal/discovery`

## Deliverables

### Files to Modify

```
internal/worker/
└── pool.go    # MODIFY: Add providerFactory field and NewPoolWithFactory
```

### Types to Update

```go
// internal/worker/pool.go

package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/provider"
)

// ProviderFactory creates a provider for a given unit
// This allows the orchestrator to inject provider resolution logic
type ProviderFactory func(*discovery.Unit) (provider.Provider, error)

// Pool manages a collection of workers executing units in parallel
type Pool struct {
	maxWorkers      int
	config          WorkerConfig
	events          *events.Bus
	git             *git.WorktreeManager
	github          *github.PRClient
	providerFactory ProviderFactory   // NEW: factory for creating providers per-unit
	workers         map[string]*Worker
	mu              sync.Mutex
	wg              sync.WaitGroup
	sem             chan struct{} // Semaphore for concurrency control
	firstErr        error         // First error encountered
	cancelCtx       context.Context
	cancelFunc      context.CancelFunc
}

// PoolStats holds current pool statistics
type PoolStats struct {
	ActiveWorkers  int
	CompletedUnits int
	FailedUnits    int
	TotalTasks     int
	CompletedTasks int
}
```

### Functions to Add/Update

```go
// internal/worker/pool.go

// NewPoolWithFactory creates a worker pool with a custom provider factory
func NewPoolWithFactory(maxWorkers int, cfg WorkerConfig, deps WorkerDeps, factory ProviderFactory) *Pool {
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

// NewPool creates a worker pool with the specified parallelism
// Uses a default provider factory that creates Claude providers
func NewPool(maxWorkers int, cfg WorkerConfig, deps WorkerDeps) *Pool {
	// Default factory returns the provider from deps, or creates Claude if nil
	defaultFactory := func(unit *discovery.Unit) (provider.Provider, error) {
		if deps.Provider != nil {
			return deps.Provider, nil
		}
		// Default to Claude provider for backward compatibility
		return provider.FromConfig(provider.Config{
			Type: provider.ProviderClaude,
		})
	}

	return NewPoolWithFactory(maxWorkers, cfg, deps, defaultFactory)
}

// Submit queues a unit for execution
// Blocks if pool is at capacity until a slot opens
func (p *Pool) Submit(unit *discovery.Unit) error {
	// Check for duplicate unit ID
	p.mu.Lock()
	if _, exists := p.workers[unit.ID]; exists {
		p.mu.Unlock()
		return fmt.Errorf("unit %s already submitted", unit.ID)
	}

	// Resolve provider for this unit via factory
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

	// Add to workers map
	p.workers[unit.ID] = worker
	p.mu.Unlock()

	// Increment WaitGroup before acquiring semaphore
	p.wg.Add(1)

	// Acquire semaphore slot (blocks if pool at capacity)
	p.sem <- struct{}{}

	// Start worker in goroutine
	go func() {
		defer func() {
			// Release semaphore slot
			<-p.sem
			// Mark WaitGroup as done
			p.wg.Done()
		}()

		// Run worker
		err := worker.Run(p.cancelCtx)

		// Store first error encountered
		if err != nil {
			p.mu.Lock()
			if p.firstErr == nil {
				p.firstErr = err
			}
			p.mu.Unlock()
		}
	}()

	return nil
}

// Wait blocks until all submitted units complete
func (p *Pool) Wait() error {
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.firstErr
}

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{}

	for _, worker := range p.workers {
		// Count tasks
		if worker.unit != nil {
			stats.TotalTasks += len(worker.unit.Tasks)
		}
	}

	// Active workers = current semaphore usage
	stats.ActiveWorkers = len(p.sem)

	return stats
}

// Shutdown gracefully stops all workers
func (p *Pool) Shutdown(ctx context.Context) error {
	// Signal workers to stop via context cancellation
	p.cancelFunc()

	// Wait for workers to complete with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	}
}

// New creates a new worker pool (backward compatibility stub)
// Deprecated: Use NewPool instead
func New(size int, bus *events.Bus, git *git.WorktreeManager) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		maxWorkers: size,
		events:     bus,
		git:        git,
		workers:    make(map[string]*Worker),
		sem:        make(chan struct{}, size),
		cancelCtx:  ctx,
		cancelFunc: cancel,
		// No providerFactory - will use default Claude when Submit is called
		providerFactory: func(unit *discovery.Unit) (provider.Provider, error) {
			return provider.FromConfig(provider.Config{
				Type: provider.ProviderClaude,
			})
		},
	}
}

// Stop shuts down the worker pool (backward compatibility stub)
// Deprecated: Use Shutdown instead
func (p *Pool) Stop() error {
	p.cancelFunc()
	p.wg.Wait()
	return nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Pool
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| NewPoolWithFactory works | Pool created with custom factory |
| NewPool backward compatible | Pool created with default Claude factory |
| Submit calls factory | Provider created per-unit via factory |
| Factory errors handled | Submit returns error if factory fails |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Add `ProviderFactory` type alias for the factory function signature
- Add `providerFactory` field to Pool struct
- Create `NewPoolWithFactory` constructor that accepts a custom factory
- Update `NewPool` to use a default factory that falls back to Claude
- Update `Submit` to call the factory and pass the provider to the worker
- Remove the `claude` field from Pool struct (no longer needed)
- Handle factory errors in Submit by returning an error before starting the worker
- Ensure backward compatibility: existing code using NewPool should work unchanged

## NOT In Scope

- Provider resolution logic (task #4)
- Orchestrator changes (task #4)
- Unit tests for factory behavior (covered by backpressure command)
