package feature

import (
	"context"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
)

// newTestWorkflow creates a workflow for testing
func newTestWorkflow() *Workflow {
	prd := &PRD{
		Body:  "test-feature",
		Units: []Unit{},
	}

	bus := events.NewBus(100)

	cfg := WorkflowConfig{
		MaxReviewIterations: 3,
	}

	deps := WorkflowDeps{
		BranchMgr: &BranchManager{},
		Reviewer: &mockReviewer{
			responses: []ReviewResult{
				{Verdict: "pass"},
			},
		},
		Discovery: &discovery.Discovery{},
		Events:    bus,
		Git:       &git.Client{WorktreePath: "/tmp/test"},
		GitHub:    &github.PRClient{},
		Claude:    &mockClaudeClient{},
	}

	return NewWorkflow(prd, cfg, deps)
}

func TestWorkflow_StartFromPending(t *testing.T) {
	w := newTestWorkflow()

	if w.CurrentStatus() != StatusPending {
		t.Fatalf("expected initial status to be pending, got %s", w.CurrentStatus())
	}

	err := w.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if w.CurrentStatus() != StatusGeneratingSpecs {
		t.Errorf("expected status to be generating_specs, got %s", w.CurrentStatus())
	}
}

func TestWorkflow_InvalidStartState(t *testing.T) {
	w := newTestWorkflow()

	// Move to a non-pending state
	w.status = StatusReviewingSpecs

	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting from non-pending state")
	}
}

func TestWorkflow_FullCycle(t *testing.T) {
	w := newTestWorkflow()

	ctx := context.Background()

	// Start: pending -> generating_specs
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if w.CurrentStatus() != StatusGeneratingSpecs {
		t.Errorf("expected generating_specs, got %s", w.CurrentStatus())
	}

	// GenerateSpecs: generating_specs -> reviewing_specs
	if err := w.GenerateSpecs(ctx); err != nil {
		t.Fatalf("GenerateSpecs failed: %v", err)
	}
	if w.CurrentStatus() != StatusReviewingSpecs {
		t.Errorf("expected reviewing_specs, got %s", w.CurrentStatus())
	}

	// ReviewSpecs: reviewing_specs -> validating_specs (via review cycle)
	if err := w.ReviewSpecs(ctx); err != nil {
		t.Fatalf("ReviewSpecs failed: %v", err)
	}
	if w.CurrentStatus() != StatusValidatingSpecs {
		t.Errorf("expected validating_specs, got %s", w.CurrentStatus())
	}

	// ValidateSpecs: validating_specs -> generating_tasks
	if err := w.ValidateSpecs(ctx); err != nil {
		t.Fatalf("ValidateSpecs failed: %v", err)
	}
	if w.CurrentStatus() != StatusGeneratingTasks {
		t.Errorf("expected generating_tasks, got %s", w.CurrentStatus())
	}

	// GenerateTasks: generating_tasks -> specs_committed
	if err := w.GenerateTasks(ctx); err != nil {
		t.Fatalf("GenerateTasks failed: %v", err)
	}
	if w.CurrentStatus() != StatusSpecsCommitted {
		t.Errorf("expected specs_committed, got %s", w.CurrentStatus())
	}
}

func TestWorkflow_TransitionValidation(t *testing.T) {
	w := newTestWorkflow()

	// Try invalid transition
	err := w.transitionTo(StatusComplete)
	if err == nil {
		t.Fatal("expected error for invalid transition from pending to complete")
	}

	// Try valid transition
	err = w.transitionTo(StatusGeneratingSpecs)
	if err != nil {
		t.Fatalf("unexpected error for valid transition: %v", err)
	}
}

func TestWorkflow_Resume(t *testing.T) {
	w := newTestWorkflow()

	// Set to review_blocked state
	w.status = StatusReviewBlocked

	if !w.CanResume() {
		t.Fatal("expected workflow to be resumable from review_blocked state")
	}

	// Resume should transition back to reviewing_specs
	err := w.Resume(context.Background(), ResumeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After review cycle runs with pass verdict, should reach validating_specs
	if w.CurrentStatus() != StatusValidatingSpecs {
		t.Errorf("expected validating_specs after resume, got %s", w.CurrentStatus())
	}
}

func TestWorkflow_ResumeInvalidState(t *testing.T) {
	w := newTestWorkflow()

	// Try to resume from pending state
	err := w.Resume(context.Background(), ResumeOptions{})
	if err == nil {
		t.Fatal("expected error when resuming from non-resumable state")
	}

	if w.CanResume() {
		t.Error("expected CanResume to return false for pending state")
	}
}

func TestWorkflow_CurrentStatus(t *testing.T) {
	w := newTestWorkflow()

	if w.CurrentStatus() != StatusPending {
		t.Errorf("expected pending, got %s", w.CurrentStatus())
	}

	w.status = StatusReviewingSpecs
	if w.CurrentStatus() != StatusReviewingSpecs {
		t.Errorf("expected reviewing_specs, got %s", w.CurrentStatus())
	}
}

func TestWorkflow_Escalation(t *testing.T) {
	w := newTestWorkflow()

	// Track events with a channel for synchronization
	eventReceived := make(chan events.Event, 1)
	w.events.Subscribe(func(e events.Event) {
		if e.Type == "workflow.escalation" {
			select {
			case eventReceived <- e:
			default:
			}
		}
	})

	err := w.escalate("test escalation", nil)
	if err == nil {
		t.Fatal("expected error from escalate")
	}

	// Wait for the event with a timeout
	select {
	case escalationEvent := <-eventReceived:
		if escalationEvent.Payload != "test escalation" {
			t.Errorf("expected payload 'test escalation', got %v", escalationEvent.Payload)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected escalation event to be emitted")
	}

	// Clean up
	w.events.Close()
}

func TestWorkflow_DriftCheckIntegration(t *testing.T) {
	w := newTestWorkflow()

	ctx := context.Background()

	// Check for drift - should return no drift
	result, err := w.drift.CheckDrift(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HasDrift {
		t.Error("expected no drift for unchanged PRD")
	}

	// Modify PRD body to trigger drift
	w.prd.Body = "modified-feature"

	result, err = w.drift.CheckDrift(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.HasDrift {
		t.Error("expected drift after PRD modification")
	}
}

func TestWorkflow_CompletionCheckIntegration(t *testing.T) {
	w := newTestWorkflow()

	if w.completion == nil {
		t.Skip("completion checker not initialized")
	}

	ctx := context.Background()

	// Check completion status
	status, err := w.completion.Check(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not be ready (no units complete)
	if status.ReadyForPR {
		t.Error("expected not ready for PR with no units")
	}
}
