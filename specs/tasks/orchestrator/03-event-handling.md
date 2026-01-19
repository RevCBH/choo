---
task: 3
status: pending
backpressure: "go test ./internal/orchestrator/... -run TestOrchestrator_HandleEvent"
depends_on: [1, 2]
---

# Orchestrator Event Handling

**Parent spec**: `/specs/ORCHESTRATOR.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement event subscription and the handleEvent() method that updates scheduler state based on worker completion events.

## Dependencies

### External Specs (must be implemented)
- EVENTS - provides Bus and Event types
- SCHEDULER - provides Complete() and Fail() methods
- ESCALATION - provides Escalator interface

### Task Dependencies (within this unit)
- Task #1 - Core types must be defined
- Task #2 - Run() method must exist for integration

### Package Dependencies
- `github.com/anthropics/choo/internal/events`
- `github.com/anthropics/choo/internal/scheduler`
- `github.com/anthropics/choo/internal/escalate`

## Deliverables

### Files to Create/Modify
```
internal/orchestrator/
├── orchestrator.go      # MODIFY: Add handleEvent() method
└── orchestrator_test.go # MODIFY: Add event handling tests
```

### Functions to Implement

```go
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
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = o.escalator.Escalate(ctx, issue)
			}()
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
```

### Tests to Implement

```go
// Add to internal/orchestrator/orchestrator_test.go

func TestOrchestrator_HandleEvent_UnitCompleted(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	// Create a minimal unit for scheduling
	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
	}
	sched.Schedule(units)

	// Dispatch the unit
	result := sched.Dispatch()
	if result.Unit != "unit-a" {
		t.Fatalf("expected unit-a to be dispatched")
	}

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		unitMap:   buildUnitMap(units),
	}

	// Subscribe to events
	bus.Subscribe(orch.handleEvent)

	// Emit completion event
	bus.Emit(events.NewEvent(events.UnitCompleted, "unit-a"))

	// Give event time to process
	time.Sleep(50 * time.Millisecond)

	// Check scheduler state
	state, ok := sched.GetState("unit-a")
	if !ok {
		t.Fatal("unit-a state not found")
	}
	if state.Status != scheduler.StatusComplete {
		t.Errorf("expected StatusComplete, got %v", state.Status)
	}
}

func TestOrchestrator_HandleEvent_UnitFailed(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
		{ID: "unit-b", DependsOn: []string{"unit-a"}},
	}
	sched.Schedule(units)

	// Dispatch unit-a
	sched.Dispatch()

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		unitMap:   buildUnitMap(units),
	}

	bus.Subscribe(orch.handleEvent)

	// Emit failure event
	bus.Emit(events.NewEvent(events.UnitFailed, "unit-a").WithError(fmt.Errorf("test error")))

	time.Sleep(50 * time.Millisecond)

	// Check unit-a is failed
	stateA, _ := sched.GetState("unit-a")
	if stateA.Status != scheduler.StatusFailed {
		t.Errorf("expected unit-a StatusFailed, got %v", stateA.Status)
	}

	// Check unit-b is blocked
	stateB, _ := sched.GetState("unit-b")
	if stateB.Status != scheduler.StatusBlocked {
		t.Errorf("expected unit-b StatusBlocked, got %v", stateB.Status)
	}
}

func TestOrchestrator_HandleEvent_WithEscalator(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	sched := scheduler.New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit-a", DependsOn: []string{}},
	}
	sched.Schedule(units)
	sched.Dispatch()

	// Track escalations
	escalated := make(chan escalate.Escalation, 1)
	mockEscalator := &mockEscalator{
		escalateFn: func(ctx context.Context, e escalate.Escalation) error {
			escalated <- e
			return nil
		},
	}

	orch := &Orchestrator{
		bus:       bus,
		scheduler: sched,
		escalator: mockEscalator,
		unitMap:   buildUnitMap(units),
	}

	bus.Subscribe(orch.handleEvent)

	// Emit failure with specific error type
	bus.Emit(events.NewEvent(events.UnitFailed, "unit-a").
		WithError(fmt.Errorf("merge conflict detected")))

	// Wait for escalation
	select {
	case e := <-escalated:
		if e.Unit != "unit-a" {
			t.Errorf("expected unit-a, got %s", e.Unit)
		}
		if e.Severity != escalate.SeverityBlocking {
			t.Errorf("expected blocking severity for merge conflict")
		}
		if e.Context["error_type"] != "merge_conflict" {
			t.Errorf("expected merge_conflict error type")
		}
	case <-time.After(time.Second):
		t.Error("escalation not received")
	}
}

func TestCategorizeErrorSeverity(t *testing.T) {
	tests := []struct {
		err      error
		expected escalate.Severity
	}{
		{nil, escalate.SeverityInfo},
		{fmt.Errorf("merge conflict"), escalate.SeverityBlocking},
		{fmt.Errorf("review timeout"), escalate.SeverityWarning},
		{fmt.Errorf("baseline checks failed"), escalate.SeverityCritical},
		{fmt.Errorf("unknown error"), escalate.SeverityCritical},
	}

	for _, tc := range tests {
		got := categorizeErrorSeverity(tc.err)
		if got != tc.expected {
			t.Errorf("categorizeErrorSeverity(%v) = %v, want %v", tc.err, got, tc.expected)
		}
	}
}

// mockEscalator for testing
type mockEscalator struct {
	escalateFn func(ctx context.Context, e escalate.Escalation) error
}

func (m *mockEscalator) Escalate(ctx context.Context, e escalate.Escalation) error {
	if m.escalateFn != nil {
		return m.escalateFn(ctx, e)
	}
	return nil
}

func (m *mockEscalator) Name() string {
	return "mock"
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/orchestrator/... -run TestOrchestrator_HandleEvent
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestOrchestrator_HandleEvent_UnitCompleted | Scheduler state transitions to Complete |
| TestOrchestrator_HandleEvent_UnitFailed | Scheduler state transitions to Failed |
| TestOrchestrator_HandleEvent_WithEscalator | Escalator called with correct severity |
| TestCategorizeErrorSeverity | Error types map to correct severities |

## NOT In Scope
- Shutdown logic (task #4)
- Dry-run mode (task #5)
- CLI integration (task #6)
