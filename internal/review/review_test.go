package review

import (
	"context"
	"errors"
	"testing"

	"github.com/RevCBH/choo/internal/events"
)

// sequentialTaskInvoker returns responses in sequence for testing
type sequentialTaskInvoker struct {
	responses []string
	callIndex int
	err       error
}

func (m *sequentialTaskInvoker) InvokeTask(ctx context.Context, prompt string, subagentType string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.callIndex >= len(m.responses) {
		return "", errors.New("no more mock responses")
	}
	response := m.responses[m.callIndex]
	m.callIndex++
	return response, nil
}

// mockPublisher for testing
type mockPublisher struct {
	events []events.Event
}

func (m *mockPublisher) Emit(e events.Event) {
	m.events = append(m.events, e)
}

func TestReviewer_RunReviewLoop_PassOnFirstIteration(t *testing.T) {
	ctx := context.Background()

	// Mock task invoker that returns pass verdict
	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`{"verdict": "pass", "score": {"completeness": 95, "consistency": 90, "testability": 88, "architecture": 92}, "feedback": []}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	reviewer := NewReviewer(config, mockPub, mockTask)

	session, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.FinalVerdict != "pass" {
		t.Errorf("expected FinalVerdict to be 'pass', got: %s", session.FinalVerdict)
	}

	if len(session.Iterations) != 1 {
		t.Errorf("expected 1 iteration, got: %d", len(session.Iterations))
	}

	// Verify events: Started, Iteration, Passed
	if len(mockPub.events) != 3 {
		t.Errorf("expected 3 events, got: %d", len(mockPub.events))
	}

	if mockPub.events[0].Type != SpecReviewStarted {
		t.Errorf("expected first event to be SpecReviewStarted, got: %s", mockPub.events[0].Type)
	}

	if mockPub.events[1].Type != SpecReviewIteration {
		t.Errorf("expected second event to be SpecReviewIteration, got: %s", mockPub.events[1].Type)
	}

	if mockPub.events[2].Type != SpecReviewPassed {
		t.Errorf("expected third event to be SpecReviewPassed, got: %s", mockPub.events[2].Type)
	}
}

func TestReviewer_RunReviewLoop_PassAfterRevision(t *testing.T) {
	ctx := context.Background()

	// Mock task invoker that returns needs_revision then pass
	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`{"verdict": "needs_revision", "score": {"completeness": 70, "consistency": 75, "testability": 65, "architecture": 80}, "feedback": [{"section": "Types", "issue": "Missing field", "suggestion": "Add missing field"}]}`,
			// Feedback application response
			`Feedback applied`,
			// Second review returns pass
			`{"verdict": "pass", "score": {"completeness": 95, "consistency": 90, "testability": 88, "architecture": 92}, "feedback": []}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	reviewer := NewReviewer(config, mockPub, mockTask)

	session, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.FinalVerdict != "pass" {
		t.Errorf("expected FinalVerdict to be 'pass', got: %s", session.FinalVerdict)
	}

	if len(session.Iterations) != 2 {
		t.Errorf("expected 2 iterations, got: %d", len(session.Iterations))
	}

	// Verify first iteration was needs_revision
	if session.Iterations[0].Result.Verdict != "needs_revision" {
		t.Errorf("expected first iteration verdict to be 'needs_revision', got: %s", session.Iterations[0].Result.Verdict)
	}

	// Verify second iteration was pass
	if session.Iterations[1].Result.Verdict != "pass" {
		t.Errorf("expected second iteration verdict to be 'pass', got: %s", session.Iterations[1].Result.Verdict)
	}
}

func TestReviewer_RunReviewLoop_BlockedAfterMaxIterations(t *testing.T) {
	ctx := context.Background()

	// Mock task invoker that always returns needs_revision
	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`{"verdict": "needs_revision", "score": {"completeness": 70, "consistency": 75, "testability": 65, "architecture": 80}, "feedback": [{"section": "Types", "issue": "Issue 1", "suggestion": "Fix 1"}]}`,
			`Feedback applied`, // First feedback application
			`{"verdict": "needs_revision", "score": {"completeness": 72, "consistency": 76, "testability": 66, "architecture": 81}, "feedback": [{"section": "Types", "issue": "Issue 2", "suggestion": "Fix 2"}]}`,
			`Feedback applied`, // Second feedback application
			`{"verdict": "needs_revision", "score": {"completeness": 73, "consistency": 77, "testability": 67, "architecture": 82}, "feedback": [{"section": "Types", "issue": "Issue 3", "suggestion": "Fix 3"}]}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	config.MaxIterations = 3
	reviewer := NewReviewer(config, mockPub, mockTask)

	session, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.FinalVerdict != "blocked" {
		t.Errorf("expected FinalVerdict to be 'blocked', got: %s", session.FinalVerdict)
	}

	if len(session.Iterations) != 3 {
		t.Errorf("expected 3 iterations, got: %d", len(session.Iterations))
	}

	if session.BlockReason == "" {
		t.Error("expected BlockReason to be set")
	}

	// Verify blocked event was emitted
	lastEvent := mockPub.events[len(mockPub.events)-1]
	if lastEvent.Type != SpecReviewBlocked {
		t.Errorf("expected last event to be SpecReviewBlocked, got: %s", lastEvent.Type)
	}
}

func TestReviewer_RunReviewLoop_BlockedOnMalformedOutput(t *testing.T) {
	ctx := context.Background()

	// Mock task invoker that returns malformed output
	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`This is not valid JSON`,
			`Still not valid JSON`, // Retry
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	config.RetryOnMalformed = 1
	reviewer := NewReviewer(config, mockPub, mockTask)

	session, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.FinalVerdict != "blocked" {
		t.Errorf("expected FinalVerdict to be 'blocked', got: %s", session.FinalVerdict)
	}

	if session.BlockReason == "" {
		t.Error("expected BlockReason to be set")
	}

	// Should have 0 iterations since output was malformed
	if len(session.Iterations) != 0 {
		t.Errorf("expected 0 iterations, got: %d", len(session.Iterations))
	}

	// Verify blocked event was emitted
	lastEvent := mockPub.events[len(mockPub.events)-1]
	if lastEvent.Type != SpecReviewBlocked {
		t.Errorf("expected last event to be SpecReviewBlocked, got: %s", lastEvent.Type)
	}
}

func TestReviewer_RetryOnMalformedThenSuccess(t *testing.T) {
	ctx := context.Background()

	// Mock task invoker that returns malformed then valid output
	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`This is not valid JSON`,
			`{"verdict": "pass", "score": {"completeness": 95, "consistency": 90, "testability": 88, "architecture": 92}, "feedback": []}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	config.RetryOnMalformed = 1
	reviewer := NewReviewer(config, mockPub, mockTask)

	session, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if session.FinalVerdict != "pass" {
		t.Errorf("expected FinalVerdict to be 'pass', got: %s", session.FinalVerdict)
	}

	if len(session.Iterations) != 1 {
		t.Errorf("expected 1 iteration, got: %d", len(session.Iterations))
	}

	// Verify malformed event was emitted
	var foundMalformed bool
	for _, e := range mockPub.events {
		if e.Type == SpecReviewMalformed {
			foundMalformed = true
			break
		}
	}

	if !foundMalformed {
		t.Error("expected SpecReviewMalformed event to be emitted")
	}
}

func TestReviewer_ReviewSpecs_ValidOutput(t *testing.T) {
	ctx := context.Background()

	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`{"verdict": "pass", "score": {"completeness": 95, "consistency": 90, "testability": 88, "architecture": 92}, "feedback": []}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	reviewer := NewReviewer(config, mockPub, mockTask)

	result, err := reviewer.ReviewSpecs(ctx, "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Verdict != "pass" {
		t.Errorf("expected verdict to be 'pass', got: %s", result.Verdict)
	}

	if result.Score["completeness"] != 95 {
		t.Errorf("expected completeness score to be 95, got: %d", result.Score["completeness"])
	}
}

func TestReviewer_ReviewSpecs_MalformedOutput(t *testing.T) {
	ctx := context.Background()

	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`This is not valid JSON`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	reviewer := NewReviewer(config, mockPub, mockTask)

	result, err := reviewer.ReviewSpecs(ctx, "/path/to/prd", "/path/to/specs")

	if err == nil {
		t.Fatal("expected error for malformed output")
	}

	// Result should be returned with RawOutput preserved
	if result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if result.RawOutput == "" {
		t.Error("expected RawOutput to be preserved")
	}
}

func TestReviewer_EmitsCorrectEvents_Pass(t *testing.T) {
	ctx := context.Background()

	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`{"verdict": "pass", "score": {"completeness": 95, "consistency": 90, "testability": 88, "architecture": 92}, "feedback": []}`,
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	reviewer := NewReviewer(config, mockPub, mockTask)

	_, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify events in order: Started, Iteration, Passed
	expectedEvents := []events.EventType{
		SpecReviewStarted,
		SpecReviewIteration,
		SpecReviewPassed,
	}

	if len(mockPub.events) != len(expectedEvents) {
		t.Fatalf("expected %d events, got: %d", len(expectedEvents), len(mockPub.events))
	}

	for i, expected := range expectedEvents {
		if mockPub.events[i].Type != expected {
			t.Errorf("event %d: expected %s, got: %s", i, expected, mockPub.events[i].Type)
		}
	}
}

func TestReviewer_EmitsCorrectEvents_Blocked(t *testing.T) {
	ctx := context.Background()

	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`This is not valid JSON`,
			`Still not valid JSON`, // Retry
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	config.RetryOnMalformed = 1
	reviewer := NewReviewer(config, mockPub, mockTask)

	_, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify events include: Started, Malformed (x2), Blocked
	var hasStarted, hasBlocked bool
	var malformedCount int

	for _, e := range mockPub.events {
		switch e.Type {
		case SpecReviewStarted:
			hasStarted = true
		case SpecReviewMalformed:
			malformedCount++
		case SpecReviewBlocked:
			hasBlocked = true
		}
	}

	if !hasStarted {
		t.Error("expected SpecReviewStarted event")
	}

	if malformedCount != 2 {
		t.Errorf("expected 2 SpecReviewMalformed events, got: %d", malformedCount)
	}

	if !hasBlocked {
		t.Error("expected SpecReviewBlocked event")
	}
}

func TestReviewer_BlockedPayload_ContainsRecovery(t *testing.T) {
	ctx := context.Background()

	mockTask := &sequentialTaskInvoker{
		responses: []string{
			`This is not valid JSON`,
			`Still not valid JSON`, // Retry
		},
	}

	mockPub := &mockPublisher{}

	config := DefaultReviewConfig()
	config.RetryOnMalformed = 1
	reviewer := NewReviewer(config, mockPub, mockTask)

	_, err := reviewer.RunReviewLoop(ctx, "test-feature", "/path/to/prd", "/path/to/specs")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Find blocked event
	var blockedEvent *events.Event
	for i := range mockPub.events {
		if mockPub.events[i].Type == SpecReviewBlocked {
			blockedEvent = &mockPub.events[i]
			break
		}
	}

	if blockedEvent == nil {
		t.Fatal("expected to find SpecReviewBlocked event")
	}

	// Verify payload contains recovery actions
	payload, ok := blockedEvent.Payload.(ReviewBlockedPayload)
	if !ok {
		t.Fatal("expected payload to be ReviewBlockedPayload")
	}

	if len(payload.Recovery) == 0 {
		t.Error("expected recovery actions to be present")
	}

	if payload.Feature != "test-feature" {
		t.Errorf("expected feature to be 'test-feature', got: %s", payload.Feature)
	}

	if payload.Reason == "" {
		t.Error("expected reason to be set")
	}
}
