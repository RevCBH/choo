package orchestrator

import (
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
