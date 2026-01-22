package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/RevCBH/choo/internal/scheduler"
	"github.com/RevCBH/choo/internal/worker"
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
	// completedUnits tracks unit IDs for PR description
	completedUnits []string

	// Synchronization for background goroutines
	escalateMu     sync.Mutex
	escalateWg     sync.WaitGroup
	escalateCtx    context.Context
	escalateCancel context.CancelFunc
	closing        bool

	// Bounded semaphore for escalation goroutines (prevents goroutine leak)
	escalateSem chan struct{}
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

	// SuppressOutput disables stdout/stderr tee in workers (TUI mode)
	SuppressOutput bool

	// FeatureBranch is the feature branch name when in feature mode
	FeatureBranch string

	// FeatureTitle is the title used for feature PRs
	FeatureTitle string

	// FeatureDescription is the summary used for feature PRs
	FeatureDescription string

	// FeatureMode is true when operating in feature mode
	FeatureMode bool

	// DefaultProvider is the provider type from --provider flag
	// Used when unit frontmatter doesn't specify a provider
	DefaultProvider string

	// ForceTaskProvider overrides all provider selection when non-empty
	// Set via --force-task-provider CLI flag
	ForceTaskProvider string

	// ProviderConfig contains provider-specific settings from .choo.yaml
	// Includes provider type default and per-provider command overrides
	ProviderConfig config.ProviderConfig

	// ClaudeCommand is the Claude CLI command for non-task operations
	// (conflict resolution, PR creation, etc.) - always uses Claude regardless of task provider
	ClaudeCommand string

	// CodeReview contains code review configuration from .choo.yaml
	CodeReview config.CodeReviewConfig
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

// DefaultShutdownTimeout is the default grace period for shutdown
const DefaultShutdownTimeout = 30 * time.Second

// MaxConcurrentEscalations limits concurrent escalation goroutines to prevent resource exhaustion
const MaxConcurrentEscalations = 100

// New creates an orchestrator with the given configuration and dependencies
func New(cfg Config, deps Dependencies) *Orchestrator {
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	escalateCtx, escalateCancel := context.WithCancel(context.Background())
	return &Orchestrator{
		cfg:            cfg,
		bus:            deps.Bus,
		escalator:      deps.Escalator,
		git:            deps.Git,
		github:         deps.GitHub,
		unitMap:        make(map[string]*discovery.Unit),
		escalateCtx:    escalateCtx,
		escalateCancel: escalateCancel,
		escalateSem:    make(chan struct{}, MaxConcurrentEscalations),
	}
}

// Close releases all resources held by the orchestrator
// Note: Does not close the event bus as it is owned by the caller
func (o *Orchestrator) Close() error {
	// Mark as closing to prevent new escalations
	o.escalateMu.Lock()
	o.closing = true
	o.escalateMu.Unlock()

	// Stop worker pool if initialized
	if o.pool != nil {
		if err := o.pool.Stop(); err != nil {
			return fmt.Errorf("stopping worker pool: %w", err)
		}
	}

	// Cancel escalation context and wait for pending escalation goroutines
	if o.escalateCancel != nil {
		o.escalateCancel()
	}
	o.escalateWg.Wait()

	return nil
}

// shutdown gracefully stops the orchestrator with timeout
// Note: Does not close the event bus as it is owned by the caller
func (o *Orchestrator) shutdown(ctx context.Context) error {
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, o.cfg.ShutdownTimeout)
	defer cancel()

	var errs []error

	// Stop accepting new work
	// (dispatch loop exits via context cancellation in Run())

	// Wait for in-progress workers to complete
	if o.pool != nil {
		if err := o.pool.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("worker shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
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

// filterOutCompleteUnits removes units that are already complete
// and updates dependency lists to remove references to complete units
func filterOutCompleteUnits(units []*discovery.Unit) []*discovery.Unit {
	// First, collect IDs of complete units
	completeIDs := make(map[string]bool)
	for _, unit := range units {
		if unit.Status == discovery.UnitStatusComplete {
			completeIDs[unit.ID] = true
		}
	}

	// Filter out complete units and update dependency lists
	var result []*discovery.Unit
	for _, unit := range units {
		if unit.Status != discovery.UnitStatusComplete {
			// Remove completed dependencies from the unit's DependsOn list
			var filteredDeps []string
			for _, depID := range unit.DependsOn {
				if !completeIDs[depID] {
					filteredDeps = append(filteredDeps, depID)
				}
			}
			unit.DependsOn = filteredDeps
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

	// Store all discovered units before filtering (for PR creation)
	allDiscoveredUnits := units

	// Filter out already-complete units (they don't need to be re-executed)
	units = filterOutCompleteUnits(units)
	if len(units) == 0 {
		// All units already complete - create PR if in feature mode
		if o.cfg.FeatureMode && !o.cfg.NoPR {
			// Collect unit IDs from the original list
			var unitIDs []string
			for _, u := range allDiscoveredUnits {
				unitIDs = append(unitIDs, u.ID)
			}
			prURL, err := o.createFeaturePR(ctx, unitIDs...)
			if err != nil {
				return nil, fmt.Errorf("failed to create PR: %w", err)
			}
			if prURL != "" {
				fmt.Printf("\nPR created: %s\n", prURL)
			}
		}
		return &Result{
			TotalUnits:     len(allDiscoveredUnits),
			CompletedUnits: len(allDiscoveredUnits),
		}, nil
	}

	o.units = units
	o.unitMap = buildUnitMap(units)

	// Handle dry-run mode
	if o.cfg.DryRun {
		return o.dryRun(units)
	}

	// 2. Build schedule (before emitting event so we can include the graph)
	o.scheduler = scheduler.New(o.bus, o.cfg.Parallelism)
	schedule, err := o.scheduler.Schedule(units)
	if err != nil {
		return nil, fmt.Errorf("scheduling failed: %w", err)
	}

	// Emit orchestrator started event with graph for web UI
	o.bus.Emit(events.NewEvent(events.OrchStarted, "").WithPayload(map[string]any{
		"unit_count":  len(units),
		"parallelism": o.cfg.Parallelism,
		"graph":       buildGraphData(units, schedule.Levels),
	}))

	// 3. Initialize worker pool
	workerCfg := worker.WorkerConfig{
		RepoRoot:            o.cfg.RepoRoot,
		TargetBranch:        o.cfg.TargetBranch,
		WorktreeBase:        o.cfg.WorktreeBase,
		NoPR:                o.cfg.NoPR,
		BackpressureTimeout: 5 * time.Minute,
		MaxClaudeRetries:    3,
		SuppressOutput:      o.cfg.SuppressOutput,
		ClaudeCommand:       o.cfg.ClaudeCommand,
	}

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

	// Use factory-based pool construction for per-unit provider resolution
	o.pool = worker.NewPoolWithFactory(
		o.cfg.Parallelism,
		workerCfg,
		workerDeps,
		o.createProviderFactory(),
	)

	// Subscribe to worker completion events
	o.bus.Subscribe(o.handleEvent)

	// 4. Main dispatch loop
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown
			if err := o.shutdown(context.Background()); err != nil {
				// Log but don't fail - we're already shutting down
				o.bus.Emit(events.NewEvent(events.OrchFailed, "").
					WithError(fmt.Errorf("shutdown error: %w", err)))
			}
			return o.buildResult(startTime, ctx.Err()), ctx.Err()
		default:
			// Continue - not shutting down
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
			// Create PR if in feature mode
			if o.cfg.FeatureMode && !o.cfg.NoPR {
				prURL, err := o.createFeaturePR(ctx)
				if err != nil {
					o.bus.Emit(events.NewEvent(events.OrchFailed, "").
						WithError(fmt.Errorf("failed to create PR: %w", err)))
					return o.buildResult(startTime, err), err
				}
				if prURL != "" {
					fmt.Printf("\nPR created: %s\n", prURL)
				}
			}

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

// handleEvent processes events from the event bus
func (o *Orchestrator) handleEvent(e events.Event) {
	switch e.Type {
	case events.UnitCompleted:
		o.scheduler.Complete(e.Unit)

	case events.UnitFailed:
		var err error
		if e.Error != "" {
			err = fmt.Errorf("%s", e.Error)
		}
		o.scheduler.Fail(e.Unit, err)

		// Check if escalation is needed
		if o.escalator != nil {
			issue := escalate.Escalation{
				Severity: categorizeErrorSeverity(err),
				Unit:     e.Unit,
				Title:    fmt.Sprintf("Unit %s failed", e.Unit),
				Message:  e.Error,
				Context:  make(map[string]string),
			}

			// Add error type to context
			errType := categorizeErrorType(err)
			if errType != "" {
				issue.Context["error_type"] = errType
			}

			// Escalate asynchronously to avoid blocking event dispatch
			// Use semaphore to limit concurrent escalations and prevent goroutine leak
			o.escalateMu.Lock()
			if o.closing {
				o.escalateMu.Unlock()
				return
			}

			// If semaphore is initialized, use it to limit concurrency
			if o.escalateSem != nil {
				// Try to acquire semaphore (non-blocking)
				select {
				case o.escalateSem <- struct{}{}:
					// Acquired semaphore, spawn goroutine
					o.escalateWg.Add(1)
					o.escalateMu.Unlock()
					go func() {
						defer func() {
							<-o.escalateSem // Release semaphore
							o.escalateWg.Done()
						}()
						// Use escalateCtx if available, otherwise background context
						parentCtx := o.escalateCtx
						if parentCtx == nil {
							parentCtx = context.Background()
						}
						ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
						defer cancel()
						_ = o.escalator.Escalate(ctx, issue)
					}()
				default:
					// Semaphore full, drop this escalation to prevent goroutine explosion
					o.escalateMu.Unlock()
				}
			} else {
				// No semaphore (legacy/test mode), spawn goroutine directly
				o.escalateWg.Add(1)
				o.escalateMu.Unlock()
				go func() {
					defer o.escalateWg.Done()
					parentCtx := o.escalateCtx
					if parentCtx == nil {
						parentCtx = context.Background()
					}
					ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
					defer cancel()
					_ = o.escalator.Escalate(ctx, issue)
				}()
			}
		}
	}
}

// categorizeErrorSeverity determines escalation severity from error
func categorizeErrorSeverity(err error) escalate.Severity {
	if err == nil {
		return escalate.SeverityInfo
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "merge conflict"):
		return escalate.SeverityBlocking
	case strings.Contains(errStr, "review timeout"):
		return escalate.SeverityWarning
	case strings.Contains(errStr, "baseline"):
		return escalate.SeverityCritical
	default:
		return escalate.SeverityCritical
	}
}

// categorizeErrorType returns a short error type string for context
func categorizeErrorType(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "merge conflict"):
		return "merge_conflict"
	case strings.Contains(errStr, "review timeout"):
		return "review_timeout"
	case strings.Contains(errStr, "baseline"):
		return "baseline_failure"
	default:
		return "claude_failure"
	}
}

// buildGraphData creates graph data for the web UI from units and levels
func buildGraphData(units []*discovery.Unit, levels [][]string) map[string]any {
	// Build level lookup for nodes
	levelMap := make(map[string]int)
	for i, level := range levels {
		for _, unitID := range level {
			levelMap[unitID] = i
		}
	}

	// Build nodes with status and completed task count for resume support
	nodes := make([]map[string]any, 0, len(units))
	for _, unit := range units {
		// Count completed tasks for resume scenarios
		completedTasks := 0
		for _, task := range unit.Tasks {
			if task != nil && task.Status == discovery.TaskStatusComplete {
				completedTasks++
			}
		}

		// Map discovery status to web status
		status := "pending"
		if unit.Status != "" {
			status = string(unit.Status)
		}

		nodes = append(nodes, map[string]any{
			"id":              unit.ID,
			"level":           levelMap[unit.ID],
			"tasks":           len(unit.Tasks),
			"status":          status,
			"completed_tasks": completedTasks,
		})
	}

	// Build dependency map for transitive reduction
	depMap := make(map[string][]string)
	for _, unit := range units {
		depMap[unit.ID] = unit.DependsOn
	}

	// Build edges with transitive reduction
	// For each unit, only include edges to dependencies that are not
	// transitively reachable through other direct dependencies
	var edges []map[string]any
	for _, unit := range units {
		directDeps := unit.DependsOn
		if len(directDeps) <= 1 {
			// No transitive edges possible with 0 or 1 dependency
			for _, dep := range directDeps {
				edges = append(edges, map[string]any{
					"from": unit.ID,
					"to":   dep,
				})
			}
			continue
		}

		// For each direct dependency, check if it's reachable through another
		for _, dep := range directDeps {
			isTransitive := false
			for _, otherDep := range directDeps {
				if otherDep == dep {
					continue
				}
				// Check if dep is reachable from otherDep
				if isReachable(otherDep, dep, depMap) {
					isTransitive = true
					break
				}
			}
			if !isTransitive {
				edges = append(edges, map[string]any{
					"from": unit.ID,
					"to":   dep,
				})
			}
		}
	}

	// Build levels array
	levelsData := make([][]string, len(levels))
	for i, level := range levels {
		levelsData[i] = make([]string, len(level))
		copy(levelsData[i], level)
	}

	return map[string]any{
		"nodes":  nodes,
		"edges":  edges,
		"levels": levelsData,
	}
}

// isReachable checks if target is reachable from start by following dependencies
func isReachable(start, target string, depMap map[string][]string) bool {
	visited := make(map[string]bool)
	queue := []string{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, dep := range depMap[current] {
			if dep == target {
				return true
			}
			if !visited[dep] {
				queue = append(queue, dep)
			}
		}
	}

	return false
}

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
	} else if unit.Provider != "" {
		// 2. Per-unit frontmatter
		providerType = provider.ProviderType(unit.Provider)
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
		if settings, ok := o.cfg.ProviderConfig.Providers[config.ProviderType(providerType)]; ok {
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

// dryRun prints the execution plan without running workers
func (o *Orchestrator) dryRun(units []*discovery.Unit) (*Result, error) {
	// Build schedule without executing
	sched := scheduler.New(o.bus, o.cfg.Parallelism)
	schedule, err := sched.Schedule(units)
	if err != nil {
		return nil, err
	}

	// Build unit map for task counts
	unitMap := buildUnitMap(units)

	// Emit dry-run started event with graph for web UI
	o.bus.Emit(events.NewEvent(events.OrchDryRunStarted, "").WithPayload(map[string]any{
		"unit_count":  len(units),
		"parallelism": o.cfg.Parallelism,
		"graph":       buildGraphData(units, schedule.Levels),
	}))

	// Print execution plan
	fmt.Printf("Execution Plan\n")
	fmt.Printf("==============\n\n")
	fmt.Printf("Units to execute: %d\n", len(units))
	fmt.Printf("Max parallelism: %d\n", o.cfg.Parallelism)
	fmt.Printf("Execution levels: %d\n\n", len(schedule.Levels))

	for i, level := range schedule.Levels {
		fmt.Printf("Level %d (parallel):\n", i+1)
		for _, unitID := range level {
			unit := unitMap[unitID]
			taskCount := 0
			if unit != nil {
				taskCount = len(unit.Tasks)
			}
			fmt.Printf("  - %s (%d tasks)\n", unitID, taskCount)
		}
		fmt.Println()
	}

	fmt.Printf("Topological order:\n")
	for i, unitID := range schedule.TopologicalOrder {
		fmt.Printf("  %d. %s\n", i+1, unitID)
	}

	// Emit dry-run completed event
	o.bus.Emit(events.NewEvent(events.OrchDryRunCompleted, ""))

	// Brief delay to allow event bus subscribers to process events
	// This is important for web UI to receive events before program exits
	time.Sleep(100 * time.Millisecond)

	return &Result{
		TotalUnits: len(units),
	}, nil
}

// prURLPattern matches GitHub PR URLs
var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

// createFeaturePR creates a pull request for the feature branch.
func (o *Orchestrator) createFeaturePR(ctx context.Context, completedUnitIDs ...string) (string, error) {
	if !o.cfg.FeatureMode || o.cfg.FeatureBranch == "" {
		return "", nil // Not in feature mode, nothing to do
	}

	if o.cfg.NoPR {
		return "", nil // PR creation disabled
	}

	// Push the feature branch first
	if err := o.pushFeatureBranch(ctx); err != nil {
		return "", fmt.Errorf("failed to push feature branch: %w", err)
	}

	if len(completedUnitIDs) > 0 {
		o.completedUnits = append([]string(nil), completedUnitIDs...)
	} else if len(o.completedUnits) == 0 {
		for _, unit := range o.units {
			o.completedUnits = append(o.completedUnits, unit.ID)
		}
	}

	title := fmt.Sprintf("feat: %s", o.cfg.FeatureTitle)
	body := o.buildPRBody()

	prCmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--base", o.cfg.TargetBranch,
		"--head", o.cfg.FeatureBranch,
		"--title", title,
		"--body", body,
	)
	prCmd.Dir = o.cfg.RepoRoot

	output, err := prCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w", err)
	}

	prURL := prURLPattern.FindString(string(output))
	if prURL == "" {
		prURL = strings.TrimSpace(string(output))
	}
	if prURL == "" {
		return "", fmt.Errorf("could not find PR URL in gh output")
	}

	if o.bus != nil {
		o.bus.Emit(events.NewEvent(events.PRCreated, "").
			WithPayload(map[string]any{
				"url":    prURL,
				"branch": o.cfg.FeatureBranch,
				"target": o.cfg.TargetBranch,
			}))
	}

	return prURL, nil
}

// pushFeatureBranch pushes the feature branch to remote
func (o *Orchestrator) pushFeatureBranch(ctx context.Context) error {
	return git.PushBranch(ctx, o.cfg.RepoRoot, o.cfg.FeatureBranch)
}
