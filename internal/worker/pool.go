package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
)

// Pool manages a collection of workers executing units in parallel
type Pool struct {
	maxWorkers int
	config     WorkerConfig
	events     *events.Bus
	git        *git.WorktreeManager
	github     *github.PRClient
	claude     *ClaudeClient
	workers    map[string]*Worker
	mu         sync.Mutex
	mergeMu    sync.Mutex // Serializes merge operations to prevent conflicts
	wg         sync.WaitGroup
	sem        chan struct{} // Semaphore for concurrency control
	firstErr   error         // First error encountered
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
}

// PoolStats holds current pool statistics
type PoolStats struct {
	ActiveWorkers  int
	CompletedUnits int
	FailedUnits    int
	TotalTasks     int
	CompletedTasks int
}

// NewPool creates a worker pool with the specified parallelism
func NewPool(maxWorkers int, cfg WorkerConfig, deps WorkerDeps) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		maxWorkers: maxWorkers,
		config:     cfg,
		events:     deps.Events,
		git:        deps.Git,
		github:     deps.GitHub,
		claude:     deps.Claude,
		workers:    make(map[string]*Worker),
		sem:        make(chan struct{}, maxWorkers),
		cancelCtx:  ctx,
		cancelFunc: cancel,
	}
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

	// Create worker for unit
	worker := NewWorker(unit, p.config, WorkerDeps{
		Events:  p.events,
		Git:     p.git,
		GitHub:  p.github,
		Claude:  p.claude,
		MergeMu: &p.mergeMu,
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
		// Count active workers (those still running)
		// Since we can't easily determine if a worker is done without adding state,
		// we'll count based on semaphore capacity

		// Count tasks
		if worker.unit != nil {
			stats.TotalTasks += len(worker.unit.Tasks)
		}
	}

	// Active workers = current semaphore usage
	stats.ActiveWorkers = len(p.sem)

	// For completed/failed, we need to check if workers have finished
	// Since Worker doesn't track completion state, we'll infer from events or errors
	// For now, we'll track based on the workers map and assume completion
	// This is a simplified implementation - a full implementation would track state

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
	}
}

// Stop shuts down the worker pool (backward compatibility stub)
// Deprecated: Use Shutdown instead
func (p *Pool) Stop() error {
	p.cancelFunc()
	p.wg.Wait()
	return nil
}
