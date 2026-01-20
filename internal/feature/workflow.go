package feature

import (
	"context"
	"fmt"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
)

// Workflow manages the feature development lifecycle
type Workflow struct {
	prd                 *PRD
	branchMgr           *BranchManager
	reviewer            Reviewer
	discovery           *discovery.Discovery
	events              *events.Bus
	git                 *git.Client
	github              *github.PRClient
	drift               *DriftDetector
	completion          *CompletionChecker
	reviewCycle         *ReviewCycle
	maxReviewIterations int
	status              FeatureStatus
}

// WorkflowConfig holds configuration for the workflow
type WorkflowConfig struct {
	MaxReviewIterations int
	PushRetries         int
	DriftCheckInterval  time.Duration
}

// WorkflowDeps holds external dependencies
type WorkflowDeps struct {
	BranchMgr  *BranchManager
	Reviewer   Reviewer
	Discovery  *discovery.Discovery
	Events     *events.Bus
	Git        *git.Client
	GitHub     *github.PRClient
	Claude     ClaudeClient
}

// NewWorkflow creates a new feature workflow manager
func NewWorkflow(prd *PRD, cfg WorkflowConfig, deps WorkflowDeps) *Workflow {
	if cfg.MaxReviewIterations == 0 {
		cfg.MaxReviewIterations = 3
	}

	w := &Workflow{
		prd:                 prd,
		branchMgr:           deps.BranchMgr,
		reviewer:            deps.Reviewer,
		discovery:           deps.Discovery,
		events:              deps.Events,
		git:                 deps.Git,
		github:              deps.GitHub,
		maxReviewIterations: cfg.MaxReviewIterations,
		status:              StatusPending,
	}

	// Initialize drift detector
	w.drift = NewDriftDetectorFromPRD(prd, deps.Claude)

	// Initialize completion checker
	if deps.GitHub != nil {
		w.completion = NewCompletionChecker(prd, deps.Git, deps.GitHub, deps.Events)
	}

	// Initialize review cycle
	w.reviewCycle = NewReviewCycle(deps.Reviewer, ReviewCycleConfig{
		MaxIterations: cfg.MaxReviewIterations,
	})

	// Set transition and escalate callbacks
	w.reviewCycle.transitionFn = w.transitionTo
	w.reviewCycle.escalateFn = w.escalate

	return w
}

// Start initiates the feature workflow from pending state
func (w *Workflow) Start(ctx context.Context) error {
	if w.status != StatusPending {
		return fmt.Errorf("cannot start: workflow is in %s state, must be pending", w.status)
	}

	return w.transitionTo(StatusGeneratingSpecs)
}

// GenerateSpecs transitions to spec generation
func (w *Workflow) GenerateSpecs(ctx context.Context) error {
	if w.status != StatusGeneratingSpecs {
		return fmt.Errorf("cannot generate specs: workflow is in %s state", w.status)
	}

	// Spec generation logic would go here (delegated to Claude or other service)
	// For now, just transition to reviewing
	return w.transitionTo(StatusReviewingSpecs)
}

// ReviewSpecs runs the spec review cycle
func (w *Workflow) ReviewSpecs(ctx context.Context) error {
	if w.status != StatusReviewingSpecs {
		return fmt.Errorf("cannot review specs: workflow is in %s state", w.status)
	}

	// Delegate to review cycle
	// The review cycle will handle transitions internally
	return w.reviewCycle.Run(ctx, []Spec{})
}

// ValidateSpecs validates specs before task generation
func (w *Workflow) ValidateSpecs(ctx context.Context) error {
	if w.status != StatusValidatingSpecs {
		return fmt.Errorf("cannot validate specs: workflow is in %s state", w.status)
	}

	// Validation logic would go here
	// For now, just transition to generating tasks
	return w.transitionTo(StatusGeneratingTasks)
}

// GenerateTasks generates implementation tasks from specs
func (w *Workflow) GenerateTasks(ctx context.Context) error {
	if w.status != StatusGeneratingTasks {
		return fmt.Errorf("cannot generate tasks: workflow is in %s state", w.status)
	}

	// Task generation logic would go here
	// For now, just transition to specs_committed
	return w.transitionTo(StatusSpecsCommitted)
}

// CommitSpecs commits generated specs to feature branch
func (w *Workflow) CommitSpecs(ctx context.Context) error {
	if w.status != StatusSpecsCommitted {
		return fmt.Errorf("cannot commit specs: workflow is in %s state", w.status)
	}

	// Commit the specs
	prdID := w.prd.Body // Using body as ID for now
	_, err := CommitSpecs(ctx, w.git, prdID)
	if err != nil {
		return fmt.Errorf("failed to commit specs: %w", err)
	}

	// Transition to in_progress
	return w.transitionTo(StatusInProgress)
}

// Resume continues workflow from review_blocked state
func (w *Workflow) Resume(ctx context.Context, opts ResumeOptions) error {
	if !w.CanResume() {
		return fmt.Errorf("cannot resume: workflow is in %s state, must be review_blocked", w.status)
	}

	// Delegate to review cycle
	return w.reviewCycle.Resume(ctx, opts)
}

// CurrentStatus returns the current workflow state
func (w *Workflow) CurrentStatus() FeatureStatus {
	return w.status
}

// CanResume returns true if workflow can be resumed
func (w *Workflow) CanResume() bool {
	return w.status.CanResume()
}

// transitionTo changes state with validation
func (w *Workflow) transitionTo(status FeatureStatus) error {
	if !CanTransition(w.status, status) {
		return fmt.Errorf("invalid transition from %s to %s", w.status, status)
	}

	oldStatus := w.status
	w.status = status

	// Emit state change event
	if w.events != nil {
		event := events.NewEvent(events.EventType(fmt.Sprintf("workflow.%s", status)), w.prd.Body)
		event.Payload = map[string]string{
			"from": string(oldStatus),
			"to":   string(status),
		}
		w.events.Emit(event)
	}

	return nil
}

// escalate notifies user of unrecoverable issue
func (w *Workflow) escalate(msg string, err error) error {
	// Emit escalation event
	if w.events != nil {
		event := events.NewEvent(events.EventType("workflow.escalation"), w.prd.Body)
		event.Payload = msg
		if err != nil {
			event = event.WithError(err)
		}
		w.events.Emit(event)
	}

	// Return the error to caller
	if err != nil {
		return fmt.Errorf("escalation: %s: %w", msg, err)
	}
	return fmt.Errorf("escalation: %s", msg)
}
