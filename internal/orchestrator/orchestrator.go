package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/escalate"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
	"github.com/anthropics/choo/internal/scheduler"
	"github.com/anthropics/choo/internal/worker"
)

// Orchestrator coordinates unit execution across all subsystems
type Orchestrator struct {
	cfg       Config
	bus       *events.Bus
	escalator escalate.Escalator
	scheduler *scheduler.Scheduler
	pool      *worker.Pool
	git       *git.WorktreeManager
	github    *github.PRClient

	// Runtime state
	units   []*discovery.Unit
	unitMap map[string]*discovery.Unit // unitID -> Unit for quick lookup
}

// Config holds orchestrator-specific configuration
type Config struct {
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
}

// Dependencies bundles external dependencies for injection
type Dependencies struct {
	Bus       *events.Bus
	Escalator escalate.Escalator
	Git       *git.WorktreeManager
	GitHub    *github.PRClient
}

// Result represents the outcome of an orchestration run
type Result struct {
	TotalUnits     int
	CompletedUnits int
	FailedUnits    int
	BlockedUnits   int
	Duration       time.Duration
	Error          error
}

// New creates an orchestrator with the given configuration and dependencies
func New(cfg Config, deps Dependencies) *Orchestrator {
	return &Orchestrator{
		cfg:       cfg,
		bus:       deps.Bus,
		escalator: deps.Escalator,
		git:       deps.Git,
		github:    deps.GitHub,
		unitMap:   make(map[string]*discovery.Unit),
	}
}

// Close releases all resources held by the orchestrator
func (o *Orchestrator) Close() error {
	// Stop worker pool if initialized
	if o.pool != nil {
		o.pool.Stop()
	}

	// Close event bus
	if o.bus != nil {
		o.bus.Close()
	}

	return nil
}

// buildUnitMap creates a lookup map from unit ID to unit
func buildUnitMap(units []*discovery.Unit) map[string]*discovery.Unit {
	m := make(map[string]*discovery.Unit, len(units))
	for _, unit := range units {
		m[unit.ID] = unit
	}
	return m
}

// filterToUnit returns only the specified unit and its dependencies
func filterToUnit(units []*discovery.Unit, targetID string) []*discovery.Unit {
	// First, find the target unit
	var target *discovery.Unit
	unitMap := buildUnitMap(units)

	target = unitMap[targetID]
	if target == nil {
		return nil
	}

	// Collect target and all its dependencies recursively
	needed := make(map[string]bool)
	var collectDeps func(id string)
	collectDeps = func(id string) {
		if needed[id] {
			return
		}
		needed[id] = true
		unit := unitMap[id]
		if unit != nil {
			for _, depID := range unit.DependsOn {
				collectDeps(depID)
			}
		}
	}
	collectDeps(targetID)

	// Filter to only needed units
	var result []*discovery.Unit
	for _, unit := range units {
		if needed[unit.ID] {
			result = append(result, unit)
		}
	}
	return result
}

// Run executes the orchestration loop until completion or cancellation
// Returns Result with execution statistics
func (o *Orchestrator) Run(ctx context.Context) (*Result, error) {
	startTime := time.Now()

	// 1. Discovery phase
	units, err := discovery.Discover(o.cfg.TasksDir)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Filter to single unit if specified
	if o.cfg.SingleUnit != "" {
		units = filterToUnit(units, o.cfg.SingleUnit)
		if len(units) == 0 {
			return nil, fmt.Errorf("unit %q not found", o.cfg.SingleUnit)
		}
	}

	o.units = units
	o.unitMap = buildUnitMap(units)

	// Handle dry-run mode
	if o.cfg.DryRun {
		return o.dryRun(units)
	}

	// Emit orchestrator started event
	o.bus.Emit(events.NewEvent(events.OrchStarted, "").WithPayload(map[string]any{
		"unit_count":  len(units),
		"parallelism": o.cfg.Parallelism,
	}))

	// 2. Build schedule
	o.scheduler = scheduler.New(o.bus, o.cfg.Parallelism)
	_, err = o.scheduler.Schedule(units)
	if err != nil {
		return nil, fmt.Errorf("scheduling failed: %w", err)
	}

	// 3. Initialize worker pool
	workerCfg := worker.WorkerConfig{
		RepoRoot:     o.cfg.RepoRoot,
		TargetBranch: o.cfg.TargetBranch,
		WorktreeBase: o.cfg.WorktreeBase,
		NoPR:         o.cfg.NoPR,
	}

	workerDeps := worker.WorkerDeps{
		Events: o.bus,
		Git:    o.git,
		GitHub: o.github,
	}

	o.pool = worker.NewPool(o.cfg.Parallelism, workerCfg, workerDeps)

	// Subscribe to worker completion events
	o.bus.Subscribe(o.handleEvent)

	// 4. Main dispatch loop
	for {
		select {
		case <-ctx.Done():
			return o.buildResult(startTime, ctx.Err()), ctx.Err()
		default:
		}

		// Attempt to dispatch next ready unit
		result := o.scheduler.Dispatch()

		switch result.Reason {
		case scheduler.ReasonNone:
			// Successfully dispatched, submit to pool
			unit := o.unitMap[result.Unit]
			if err := o.pool.Submit(unit); err != nil {
				o.scheduler.Fail(result.Unit, err)
			}

		case scheduler.ReasonAllComplete:
			// All units finished successfully
			o.bus.Emit(events.NewEvent(events.OrchCompleted, ""))
			return o.buildResult(startTime, nil), nil

		case scheduler.ReasonAllBlocked:
			// All remaining units are blocked by failures
			err := fmt.Errorf("execution blocked: all remaining units depend on failed units")
			o.bus.Emit(events.NewEvent(events.OrchFailed, "").WithError(err))
			return o.buildResult(startTime, err), err

		case scheduler.ReasonAtCapacity, scheduler.ReasonNoReady:
			// Wait for workers to complete or dependencies to resolve
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// buildResult constructs the Result from current scheduler state
func (o *Orchestrator) buildResult(startTime time.Time, err error) *Result {
	result := &Result{
		TotalUnits: len(o.units),
		Duration:   time.Since(startTime),
		Error:      err,
	}

	if o.scheduler == nil {
		return result
	}

	// Count states from scheduler
	states := o.scheduler.GetAllStates()
	for _, state := range states {
		switch state.Status {
		case scheduler.StatusComplete:
			result.CompletedUnits++
		case scheduler.StatusFailed:
			result.FailedUnits++
		case scheduler.StatusBlocked:
			result.BlockedUnits++
		}
	}

	return result
}

// handleEvent is a stub for event handling (task #3)
func (o *Orchestrator) handleEvent(evt events.Event) {
	// Event handling implementation will be added in task #3
}

// dryRun is a stub for dry-run mode (task #5)
func (o *Orchestrator) dryRun(units []*discovery.Unit) (*Result, error) {
	// Dry-run implementation will be added in task #5
	return &Result{
		TotalUnits: len(units),
	}, nil
}
