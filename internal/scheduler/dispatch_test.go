package scheduler

import (
	"testing"
	"time"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
)

func TestDispatch_Success(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	result := s.Dispatch()

	if !result.Dispatched {
		t.Errorf("Expected Dispatched=true, got false")
	}
	if result.Unit != "unit1" {
		t.Errorf("Expected Unit=unit1, got %q", result.Unit)
	}
	if result.Reason != ReasonNone {
		t.Errorf("Expected Reason=ReasonNone, got %q", result.Reason)
	}
}

func TestDispatch_SetsStartedAt(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	s.Dispatch()

	state, ok := s.GetState("unit1")
	if !ok {
		t.Fatal("Unit state not found")
	}

	if state.StartedAt == nil {
		t.Error("Expected StartedAt to be set, got nil")
	}
}

func TestDispatch_TransitionsToInProgress(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	s.Dispatch()

	state, ok := s.GetState("unit1")
	if !ok {
		t.Fatal("Unit state not found")
	}

	if state.Status != StatusInProgress {
		t.Errorf("Expected status=in_progress, got %s", state.Status)
	}
}

func TestDispatch_EmitsEvent(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	eventChan := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventChan <- e
	})

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Drain events from Schedule
	for len(eventChan) > 0 {
		<-eventChan
	}

	s.Dispatch()

	// Wait for the event to be processed
	found := false
	timeout := false
	for !found && !timeout {
		select {
		case e := <-eventChan:
			if e.Type == events.UnitStarted && e.Unit == "unit1" {
				found = true
			}
		case <-time.After(100 * time.Millisecond):
			timeout = true
		}
	}

	if !found {
		t.Error("Expected UnitStarted event for unit1")
	}
}

func TestDispatch_RespectsParallelism(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
		{ID: "unit2", DependsOn: []string{}},
		{ID: "unit3", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch first two units
	s.Dispatch()
	s.Dispatch()

	// Third dispatch should be blocked by parallelism
	result := s.Dispatch()

	if result.Dispatched {
		t.Error("Expected Dispatched=false when at capacity")
	}
	if result.Reason != ReasonAtCapacity {
		t.Errorf("Expected Reason=at_capacity, got %q", result.Reason)
	}
}

func TestDispatch_NoReady(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
		{ID: "unit2", DependsOn: []string{"unit1"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch unit1
	s.Dispatch()

	// unit2 is still pending (waiting for unit1 to complete)
	// Ready queue should be empty but work remains
	result := s.Dispatch()

	if result.Dispatched {
		t.Error("Expected Dispatched=false when no ready units")
	}
	if result.Reason != ReasonNoReady {
		t.Errorf("Expected Reason=no_ready_units, got %q", result.Reason)
	}
}

func TestDispatch_AllComplete(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch and complete unit1
	s.Dispatch()
	err = s.Transition("unit1", StatusComplete)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// All units are complete
	result := s.Dispatch()

	if result.Dispatched {
		t.Error("Expected Dispatched=false when all complete")
	}
	if result.Reason != ReasonAllComplete {
		t.Errorf("Expected Reason=all_complete, got %q", result.Reason)
	}
}

func TestDispatch_AllBlocked(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
		{ID: "unit2", DependsOn: []string{"unit1"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch and fail unit1
	s.Dispatch()
	err = s.Transition("unit1", StatusFailed)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Manually block unit2 (in real scenario, this would happen when evaluating deps)
	err = s.Transition("unit2", StatusBlocked)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// All units are either failed or blocked
	result := s.Dispatch()

	if result.Dispatched {
		t.Error("Expected Dispatched=false when all blocked")
	}
	if result.Reason != ReasonAllBlocked {
		t.Errorf("Expected Reason=all_blocked, got %q", result.Reason)
	}
}

func TestDispatch_ParallelismIncludesPRPhase(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 2)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
		{ID: "unit2", DependsOn: []string{}},
		{ID: "unit3", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch unit1 and transition to pr_open
	s.Dispatch()
	err = s.Transition("unit1", StatusPROpen)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Dispatch unit2 and transition through pr_open to in_review
	s.Dispatch()
	err = s.Transition("unit2", StatusPROpen)
	if err != nil {
		t.Fatalf("Transition to pr_open failed: %v", err)
	}
	err = s.Transition("unit2", StatusInReview)
	if err != nil {
		t.Fatalf("Transition to in_review failed: %v", err)
	}

	// Both units are in PR phases (pr_open, in_review)
	// These should count against parallelism limit
	result := s.Dispatch()

	if result.Dispatched {
		t.Error("Expected Dispatched=false when PR phases consume parallelism")
	}
	if result.Reason != ReasonAtCapacity {
		t.Errorf("Expected Reason=at_capacity, got %q", result.Reason)
	}
}

func TestDispatch_Sequential(t *testing.T) {
	bus := events.NewBus(10)
	defer bus.Close()

	s := New(bus, 3)

	units := []*discovery.Unit{
		{ID: "unit1", DependsOn: []string{}},
		{ID: "unit2", DependsOn: []string{}},
		{ID: "unit3", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Dispatch multiple units
	result1 := s.Dispatch()
	result2 := s.Dispatch()
	result3 := s.Dispatch()

	// All should be dispatched successfully
	if !result1.Dispatched || !result2.Dispatched || !result3.Dispatched {
		t.Error("Expected all dispatches to succeed")
	}

	// All units should be different
	units_dispatched := map[string]bool{
		result1.Unit: true,
		result2.Unit: true,
		result3.Unit: true,
	}

	if len(units_dispatched) != 3 {
		t.Errorf("Expected 3 different units, got: %v", units_dispatched)
	}

	// Verify all expected units were dispatched
	for _, unitID := range []string{"unit1", "unit2", "unit3"} {
		if !units_dispatched[unitID] {
			t.Errorf("Expected unit %q to be dispatched", unitID)
		}
	}
}
