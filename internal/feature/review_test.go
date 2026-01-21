package feature

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// mockReviewer simulates the review.Reviewer interface
type mockReviewer struct {
	responses     []ReviewResult
	responseIndex int
	err           error
}

func (m *mockReviewer) Review(ctx context.Context, specs []Spec) (*ReviewResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.responseIndex >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}
	result := m.responses[m.responseIndex]
	m.responseIndex++
	return &result, nil
}

// loadReviewResult loads a review result from testdata
func loadReviewResult(t *testing.T, filename string) ReviewResult {
	t.Helper()
	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", filename, err)
	}
	var result ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal fixture %s: %v", filename, err)
	}
	return result
}

// TestReviewCycle_PassFirstIteration tests that review cycle transitions to validating_specs on first pass
func TestReviewCycle_PassFirstIteration(t *testing.T) {
	passResult := loadReviewResult(t, "review_pass.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{passResult},
	}

	var lastTransition FeatureStatus
	transitionFn := func(status FeatureStatus) error {
		lastTransition = status
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn

	err := cycle.Run(context.Background(), []Spec{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if lastTransition != StatusValidatingSpecs {
		t.Errorf("Expected transition to %v, got %v", StatusValidatingSpecs, lastTransition)
	}
}

// TestReviewCycle_PassAfterRevisions tests multiple iterations then pass
func TestReviewCycle_PassAfterRevisions(t *testing.T) {
	needsRevision := loadReviewResult(t, "review_needs_revision.json")
	passResult := loadReviewResult(t, "review_pass.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{needsRevision, needsRevision, passResult},
	}

	var transitions []FeatureStatus
	transitionFn := func(status FeatureStatus) error {
		transitions = append(transitions, status)
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn

	err := cycle.Run(context.Background(), []Spec{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should see: updating, reviewing, updating, reviewing, validating
	expectedTransitions := []FeatureStatus{
		StatusUpdatingSpecs,
		StatusReviewingSpecs,
		StatusUpdatingSpecs,
		StatusReviewingSpecs,
		StatusValidatingSpecs,
	}

	if len(transitions) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions, got %d", len(expectedTransitions), len(transitions))
	}

	for i, expected := range expectedTransitions {
		if transitions[i] != expected {
			t.Errorf("Transition %d: expected %v, got %v", i, expected, transitions[i])
		}
	}
}

// TestReviewCycle_MaxIterationsBlocked tests transition to review_blocked when max iterations reached
func TestReviewCycle_MaxIterationsBlocked(t *testing.T) {
	needsRevision := loadReviewResult(t, "review_needs_revision.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{needsRevision, needsRevision, needsRevision},
	}

	var lastTransition FeatureStatus
	var escalated bool
	transitionFn := func(status FeatureStatus) error {
		lastTransition = status
		return nil
	}
	escalateFn := func(msg string, err error) error {
		escalated = true
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn
	cycle.escalateFn = escalateFn

	err := cycle.Run(context.Background(), []Spec{})
	if err != nil {
		t.Fatalf("Expected no error from Run, got: %v", err)
	}

	if lastTransition != StatusReviewBlocked {
		t.Errorf("Expected transition to %v, got %v", StatusReviewBlocked, lastTransition)
	}

	if !escalated {
		t.Error("Expected escalation to be called")
	}
}

// TestReviewCycle_MalformedOutputBlocked tests transition to review_blocked on malformed output
func TestReviewCycle_MalformedOutputBlocked(t *testing.T) {
	reviewer := &mockReviewer{
		err: &MalformedReviewError{msg: "invalid JSON"},
	}

	var lastTransition FeatureStatus
	var escalated bool
	transitionFn := func(status FeatureStatus) error {
		lastTransition = status
		return nil
	}
	escalateFn := func(msg string, err error) error {
		escalated = true
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn
	cycle.escalateFn = escalateFn

	err := cycle.Run(context.Background(), []Spec{})
	if err != nil {
		t.Fatalf("Expected no error from Run, got: %v", err)
	}

	if lastTransition != StatusReviewBlocked {
		t.Errorf("Expected transition to %v, got %v", StatusReviewBlocked, lastTransition)
	}

	if !escalated {
		t.Error("Expected escalation to be called")
	}
}

// TestReviewCycle_StateTransitions tests correct transition sequence: reviewing -> updating -> reviewing
func TestReviewCycle_StateTransitions(t *testing.T) {
	needsRevision := loadReviewResult(t, "review_needs_revision.json")
	passResult := loadReviewResult(t, "review_pass.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{needsRevision, passResult},
	}

	var transitions []FeatureStatus
	transitionFn := func(status FeatureStatus) error {
		transitions = append(transitions, status)
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn

	err := cycle.Run(context.Background(), []Spec{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should see: updating, reviewing, validating
	expectedTransitions := []FeatureStatus{
		StatusUpdatingSpecs,
		StatusReviewingSpecs,
		StatusValidatingSpecs,
	}

	if len(transitions) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions, got %d", len(expectedTransitions), len(transitions))
	}

	for i, expected := range expectedTransitions {
		if transitions[i] != expected {
			t.Errorf("Transition %d: expected %v, got %v", i, expected, transitions[i])
		}
	}
}

// TestReviewCycle_ResumeFromBlocked tests that Resume transitions back to reviewing_specs
func TestReviewCycle_ResumeFromBlocked(t *testing.T) {
	passResult := loadReviewResult(t, "review_pass.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{passResult},
	}

	var transitions []FeatureStatus
	transitionFn := func(status FeatureStatus) error {
		transitions = append(transitions, status)
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn

	err := cycle.Resume(context.Background(), ResumeOptions{SkipReview: false})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should see: reviewing, validating
	expectedTransitions := []FeatureStatus{
		StatusReviewingSpecs,
		StatusValidatingSpecs,
	}

	if len(transitions) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions, got %d", len(expectedTransitions), len(transitions))
	}

	for i, expected := range expectedTransitions {
		if transitions[i] != expected {
			t.Errorf("Transition %d: expected %v, got %v", i, expected, transitions[i])
		}
	}
}

// TestReviewCycle_ResumeSkipReview tests that Resume with skip goes directly to validating_specs
func TestReviewCycle_ResumeSkipReview(t *testing.T) {
	// No reviewer responses needed since we're skipping review
	reviewer := &mockReviewer{}

	var lastTransition FeatureStatus
	transitionFn := func(status FeatureStatus) error {
		lastTransition = status
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.transitionFn = transitionFn

	err := cycle.Resume(context.Background(), ResumeOptions{SkipReview: true})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if lastTransition != StatusValidatingSpecs {
		t.Errorf("Expected transition to %v, got %v", StatusValidatingSpecs, lastTransition)
	}
}

// TestReviewCycle_EscalationOnBlock tests that escalate function is called when blocked
func TestReviewCycle_EscalationOnBlock(t *testing.T) {
	needsRevision := loadReviewResult(t, "review_needs_revision.json")

	reviewer := &mockReviewer{
		responses: []ReviewResult{needsRevision, needsRevision, needsRevision},
	}

	var escalateMsg string
	escalateFn := func(msg string, err error) error {
		escalateMsg = msg
		return nil
	}

	cycle := NewReviewCycle(reviewer, ReviewCycleConfig{MaxIterations: 3})
	cycle.escalateFn = escalateFn

	cycle.Run(context.Background(), []Spec{})

	if escalateMsg == "" {
		t.Error("Expected escalation message to be set")
	}
}
