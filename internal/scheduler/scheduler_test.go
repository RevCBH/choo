package scheduler

import (
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
)

func TestScheduler_New(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	if s.maxParallelism != 5 {
		t.Errorf("maxParallelism = %d, want 5", s.maxParallelism)
	}
	if s.events != bus {
		t.Error("events bus not set correctly")
	}
	if s.states == nil {
		t.Error("states map not initialized")
	}
	if s.ready == nil {
		t.Error("ready queue not initialized")
	}
}

func TestScheduler_Schedule_Simple(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
	}

	schedule, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v, want nil", err)
	}

	if schedule.MaxParallelism != 5 {
		t.Errorf("schedule.MaxParallelism = %d, want 5", schedule.MaxParallelism)
	}

	if len(schedule.TopologicalOrder) != 3 {
		t.Errorf("len(TopologicalOrder) = %d, want 3", len(schedule.TopologicalOrder))
	}

	if len(schedule.Levels) != 2 {
		t.Errorf("len(Levels) = %d, want 2", len(schedule.Levels))
	}
}

func TestScheduler_Schedule_CycleError(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"c"}},
		{ID: "c", DependsOn: []string{"a"}},
	}

	_, err := s.Schedule(units)
	if err == nil {
		t.Fatal("Schedule() error = nil, want CycleError")
	}

	if _, ok := err.(*CycleError); !ok {
		t.Errorf("Schedule() error type = %T, want *CycleError", err)
	}
}

func TestScheduler_Schedule_MissingDep(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{"nonexistent"}},
	}

	_, err := s.Schedule(units)
	if err == nil {
		t.Fatal("Schedule() error = nil, want MissingDependencyError")
	}

	if _, ok := err.(*MissingDependencyError); !ok {
		t.Errorf("Schedule() error type = %T, want *MissingDependencyError", err)
	}
}

func TestScheduler_Schedule_InitialReady(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{"a"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v, want nil", err)
	}

	ready := s.ReadyQueue()
	if len(ready) != 2 {
		t.Errorf("len(ReadyQueue) = %d, want 2", len(ready))
	}

	// Check that a and b are in ready queue
	hasA := false
	hasB := false
	for _, id := range ready {
		if id == "a" {
			hasA = true
		}
		if id == "b" {
			hasB = true
		}
	}

	if !hasA {
		t.Error("unit 'a' not in ready queue")
	}
	if !hasB {
		t.Error("unit 'b' not in ready queue")
	}

	// Check states
	stateA, ok := s.GetState("a")
	if !ok {
		t.Fatal("unit 'a' state not found")
	}
	if stateA.Status != StatusReady {
		t.Errorf("unit 'a' status = %s, want %s", stateA.Status, StatusReady)
	}

	stateC, ok := s.GetState("c")
	if !ok {
		t.Fatal("unit 'c' state not found")
	}
	if stateC.Status != StatusPending {
		t.Errorf("unit 'c' status = %s, want %s", stateC.Status, StatusPending)
	}
}

func TestScheduler_Schedule_PendingWithDeps(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v, want nil", err)
	}

	// Only 'a' should be ready
	ready := s.ReadyQueue()
	if len(ready) != 1 || ready[0] != "a" {
		t.Errorf("ReadyQueue = %v, want [a]", ready)
	}

	// b and c should be pending
	stateB, _ := s.GetState("b")
	if stateB.Status != StatusPending {
		t.Errorf("unit 'b' status = %s, want %s", stateB.Status, StatusPending)
	}

	stateC, _ := s.GetState("c")
	if stateC.Status != StatusPending {
		t.Errorf("unit 'c' status = %s, want %s", stateC.Status, StatusPending)
	}
}

func TestScheduler_Transition_Valid(t *testing.T) {
	bus := events.NewBus(100)

	eventChan := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventChan <- e
	})

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Drain any events from scheduling (with timeout to ensure async events are delivered)
	timeout := time.After(50 * time.Millisecond)
drainLoop:
	for {
		select {
		case <-eventChan:
		case <-timeout:
			break drainLoop
		default:
			// Small yield to let event loop process
			time.Sleep(1 * time.Millisecond)
			select {
			case <-eventChan:
			default:
				break drainLoop
			}
		}
	}

	// Transition from ready to in_progress
	err = s.Transition("a", StatusInProgress)
	if err != nil {
		t.Errorf("Transition() error = %v, want nil", err)
	}

	state, _ := s.GetState("a")
	if state.Status != StatusInProgress {
		t.Errorf("status after transition = %s, want %s", state.Status, StatusInProgress)
	}

	// Wait for event (should arrive quickly since bus is buffered)
	e := <-eventChan
	if e.Type != events.UnitStarted {
		t.Errorf("event type = %s, want %s", e.Type, events.UnitStarted)
	}
	if e.Unit != "a" {
		t.Errorf("event unit = %s, want 'a'", e.Unit)
	}

	bus.Close()
}

func TestScheduler_Transition_Invalid(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Try invalid transition from ready to complete (must go through in_progress)
	err = s.Transition("a", StatusComplete)
	if err == nil {
		t.Error("Transition() error = nil, want error for invalid transition")
	}
}

func TestScheduler_GetState(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	state, ok := s.GetState("a")
	if !ok {
		t.Fatal("GetState('a') returned false, want true")
	}

	if state.UnitID != "a" {
		t.Errorf("state.UnitID = %s, want 'a'", state.UnitID)
	}
	if state.Status != StatusReady {
		t.Errorf("state.Status = %s, want %s", state.Status, StatusReady)
	}
}

func TestScheduler_GetState_NotFound(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	state, ok := s.GetState("nonexistent")
	if ok {
		t.Error("GetState('nonexistent') returned true, want false")
	}
	if state != nil {
		t.Error("GetState('nonexistent') returned non-nil state")
	}
}

func TestScheduler_GetAllStates(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	states := s.GetAllStates()

	if len(states) != 2 {
		t.Errorf("len(GetAllStates()) = %d, want 2", len(states))
	}

	if states["a"] == nil {
		t.Error("states['a'] is nil")
	}
	if states["b"] == nil {
		t.Error("states['b'] is nil")
	}

	// Verify it's a deep copy by modifying the returned map
	states["a"].Status = StatusFailed
	originalState, _ := s.GetState("a")
	if originalState.Status == StatusFailed {
		t.Error("modifying GetAllStates() result affected original state")
	}
}

func TestScheduler_ActiveCount(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Initially no active units
	if count := s.ActiveCount(); count != 0 {
		t.Errorf("initial ActiveCount() = %d, want 0", count)
	}

	// Transition a to in_progress
	s.Transition("a", StatusInProgress)
	if count := s.ActiveCount(); count != 1 {
		t.Errorf("ActiveCount() after 1 in_progress = %d, want 1", count)
	}

	// Transition b to pr_open (also active)
	s.Transition("b", StatusInProgress)
	s.Transition("b", StatusPROpen)
	if count := s.ActiveCount(); count != 2 {
		t.Errorf("ActiveCount() with in_progress + pr_open = %d, want 2", count)
	}

	// Transition a to complete (no longer active)
	s.Transition("a", StatusComplete)
	if count := s.ActiveCount(); count != 1 {
		t.Errorf("ActiveCount() after one complete = %d, want 1", count)
	}
}

func TestScheduler_IsComplete_AllTerminal(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Not complete initially
	if s.IsComplete() {
		t.Error("IsComplete() = true initially, want false")
	}

	// Transition both to terminal states
	s.Transition("a", StatusInProgress)
	s.Transition("a", StatusComplete)
	s.Transition("b", StatusInProgress)
	s.Transition("b", StatusComplete)

	if !s.IsComplete() {
		t.Error("IsComplete() = false with all complete, want true")
	}
}

func TestScheduler_IsComplete_SomePending(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Complete only 'a'
	s.Transition("a", StatusInProgress)
	s.Transition("a", StatusComplete)

	// 'b' is still pending
	if s.IsComplete() {
		t.Error("IsComplete() = true with pending units, want false")
	}
}

func TestScheduler_HasFailures(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)

	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// No failures initially
	if s.HasFailures() {
		t.Error("HasFailures() = true initially, want false")
	}

	// Transition one to failed
	s.Transition("a", StatusInProgress)
	s.Transition("a", StatusFailed)

	if !s.HasFailures() {
		t.Error("HasFailures() = false with failed unit, want true")
	}

	// Test with blocked
	s2 := New(bus, 5)
	_, _ = s2.Schedule(units)

	s2.Transition("b", StatusBlocked)

	if !s2.HasFailures() {
		t.Error("HasFailures() = false with blocked unit, want true")
	}
}
