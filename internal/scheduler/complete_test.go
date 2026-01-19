package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
)

func TestComplete_SetsStatus(t *testing.T) {
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

	// Dispatch and complete the unit
	s.Dispatch()
	s.Complete("a")

	state, ok := s.GetState("a")
	if !ok {
		t.Fatal("unit 'a' not found")
	}

	if state.Status != StatusComplete {
		t.Errorf("status = %v, want %v", state.Status, StatusComplete)
	}
}

func TestComplete_SetsCompletedAt(t *testing.T) {
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

	s.Dispatch()
	s.Complete("a")

	state, ok := s.GetState("a")
	if !ok {
		t.Fatal("unit 'a' not found")
	}

	if state.CompletedAt == nil {
		t.Error("CompletedAt is nil, want non-nil timestamp")
	}
}

func TestComplete_EmitsEvent(t *testing.T) {
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

	// Subscribe to events
	eventCh := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventCh <- e
	})

	s.Dispatch()

	// Drain events from Schedule and Dispatch
	for len(eventCh) > 0 {
		<-eventCh
	}

	s.Complete("a")

	// Wait for UnitCompleted event
	found := false
	timeout := false
	for !found && !timeout {
		select {
		case evt := <-eventCh:
			if evt.Type == events.UnitCompleted && evt.Unit == "a" {
				found = true
			}
		case <-time.After(100 * time.Millisecond):
			timeout = true
		}
	}

	if !found {
		t.Error("UnitCompleted event not emitted")
	}
}

func TestComplete_UnblocksDependents(t *testing.T) {
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

	// Initially b should be pending
	stateB, _ := s.GetState("b")
	if stateB.Status != StatusPending {
		t.Errorf("b status = %v, want pending", stateB.Status)
	}

	// Dispatch and complete a
	s.Dispatch()
	s.Complete("a")

	// Now b should be ready
	stateB, _ = s.GetState("b")
	if stateB.Status != StatusReady {
		t.Errorf("b status = %v, want ready", stateB.Status)
	}

	// b should be in ready queue
	if !s.ready.Contains("b") {
		t.Error("b not in ready queue")
	}
}

func TestComplete_PartialUnblock(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Complete only a
	s.Dispatch() // dispatches a
	s.Complete("a")

	// c should still be pending (waiting for b)
	stateC, _ := s.GetState("c")
	if stateC.Status != StatusPending {
		t.Errorf("c status = %v, want pending", stateC.Status)
	}

	// Now complete b
	s.Dispatch() // dispatches b
	s.Complete("b")

	// c should now be ready
	stateC, _ = s.GetState("c")
	if stateC.Status != StatusReady {
		t.Errorf("c status = %v, want ready", stateC.Status)
	}
}

func TestComplete_EmitsQueuedEvent(t *testing.T) {
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

	// Subscribe to events
	eventCh := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventCh <- e
	})

	s.Dispatch()

	// Drain events from Schedule and Dispatch
	for len(eventCh) > 0 {
		<-eventCh
	}

	s.Complete("a")

	// Wait for UnitQueued event for b
	found := false
	timeout := false
	for !found && !timeout {
		select {
		case evt := <-eventCh:
			if evt.Type == events.UnitQueued && evt.Unit == "b" {
				found = true
			}
		case <-time.After(100 * time.Millisecond):
			timeout = true
		}
	}

	if !found {
		t.Error("UnitQueued event not emitted for dependent unit b")
	}
}

func TestFail_SetsStatus(t *testing.T) {
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

	s.Dispatch()
	s.Fail("a", errors.New("test error"))

	state, ok := s.GetState("a")
	if !ok {
		t.Fatal("unit 'a' not found")
	}

	if state.Status != StatusFailed {
		t.Errorf("status = %v, want %v", state.Status, StatusFailed)
	}
}

func TestFail_SetsError(t *testing.T) {
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

	s.Dispatch()
	testErr := errors.New("test error")
	s.Fail("a", testErr)

	state, ok := s.GetState("a")
	if !ok {
		t.Fatal("unit 'a' not found")
	}

	if state.Error == nil {
		t.Fatal("Error is nil, want non-nil")
	}

	if state.Error.Error() != testErr.Error() {
		t.Errorf("Error = %v, want %v", state.Error, testErr)
	}
}

func TestFail_EmitsEvent(t *testing.T) {
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

	// Subscribe to events
	eventCh := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventCh <- e
	})

	s.Dispatch()

	// Drain events from Schedule and Dispatch
	for len(eventCh) > 0 {
		<-eventCh
	}

	testErr := errors.New("test error")
	s.Fail("a", testErr)

	// Wait for UnitFailed event
	found := false
	timeout := false
	var failEvent events.Event
	for !found && !timeout {
		select {
		case evt := <-eventCh:
			if evt.Type == events.UnitFailed && evt.Unit == "a" {
				found = true
				failEvent = evt
			}
		case <-time.After(100 * time.Millisecond):
			timeout = true
		}
	}

	if !found {
		t.Error("UnitFailed event not emitted")
	}

	if failEvent.Error != testErr.Error() {
		t.Errorf("event error = %v, want %v", failEvent.Error, testErr.Error())
	}
}

func TestFail_BlocksDependents(t *testing.T) {
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

	// Fail a
	s.Dispatch()
	s.Fail("a", errors.New("test error"))

	// b should be blocked
	stateB, ok := s.GetState("b")
	if !ok {
		t.Fatal("unit 'b' not found")
	}

	if stateB.Status != StatusBlocked {
		t.Errorf("b status = %v, want blocked", stateB.Status)
	}
}

func TestFail_BlocksTransitive(t *testing.T) {
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
		t.Fatalf("Schedule() error = %v", err)
	}

	// Fail a
	s.Dispatch()
	s.Fail("a", errors.New("test error"))

	// Both b and c should be blocked
	stateB, _ := s.GetState("b")
	stateC, _ := s.GetState("c")

	if stateB.Status != StatusBlocked {
		t.Errorf("b status = %v, want blocked", stateB.Status)
	}

	if stateC.Status != StatusBlocked {
		t.Errorf("c status = %v, want blocked", stateC.Status)
	}
}

func TestFail_RecordsBlockedBy(t *testing.T) {
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
		t.Fatalf("Schedule() error = %v", err)
	}

	// Fail a
	s.Dispatch()
	s.Fail("a", errors.New("test error"))

	// Both b and c should have BlockedBy containing "a"
	stateB, _ := s.GetState("b")
	stateC, _ := s.GetState("c")

	if len(stateB.BlockedBy) == 0 || stateB.BlockedBy[0] != "a" {
		t.Errorf("b.BlockedBy = %v, want [a]", stateB.BlockedBy)
	}

	if len(stateC.BlockedBy) == 0 || stateC.BlockedBy[0] != "a" {
		t.Errorf("c.BlockedBy = %v, want [a]", stateC.BlockedBy)
	}
}

func TestFail_EmitsBlockedEvents(t *testing.T) {
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
		t.Fatalf("Schedule() error = %v", err)
	}

	// Subscribe to events
	eventCh := make(chan events.Event, 10)
	bus.Subscribe(func(e events.Event) {
		eventCh <- e
	})

	s.Dispatch()

	// Drain events from Schedule and Dispatch
	for len(eventCh) > 0 {
		<-eventCh
	}

	s.Fail("a", errors.New("test error"))

	// Collect UnitBlocked events for b and c (with timeout)
	blockedUnits := make(map[string]bool)
	timeout := time.After(100 * time.Millisecond)
	done := false
	for !done && len(blockedUnits) < 2 {
		select {
		case evt := <-eventCh:
			if evt.Type == events.UnitBlocked {
				blockedUnits[evt.Unit] = true
			}
			if len(blockedUnits) >= 2 {
				done = true
			}
		case <-timeout:
			// Give up if we don't get both events in time
			done = true
		}
	}

	if !blockedUnits["b"] {
		t.Error("UnitBlocked event not emitted for b")
	}

	if !blockedUnits["c"] {
		t.Error("UnitBlocked event not emitted for c")
	}
}

func TestFail_RemovesFromReadyQueue(t *testing.T) {
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
		t.Fatalf("Schedule() error = %v", err)
	}

	// a and b should be ready initially
	if !s.ready.Contains("a") {
		t.Error("a not in ready queue")
	}
	if !s.ready.Contains("b") {
		t.Error("b not in ready queue")
	}

	// Fail a while it's still in ready queue
	s.Fail("a", errors.New("test error"))

	// c should be blocked and not in ready queue
	if s.ready.Contains("c") {
		t.Error("c should not be in ready queue after being blocked")
	}

	stateC, _ := s.GetState("c")
	if stateC.Status != StatusBlocked {
		t.Errorf("c status = %v, want blocked", stateC.Status)
	}
}

func TestFail_SkipsTerminalUnits(t *testing.T) {
	bus := events.NewBus(100)
	defer bus.Close()

	s := New(bus, 5)
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}

	_, err := s.Schedule(units)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}

	// Complete b first
	s.Dispatch() // dispatches a
	s.Dispatch() // dispatches b
	s.Complete("b")

	// Now fail a
	s.Fail("a", errors.New("test error"))

	// b should remain complete, not be marked as blocked
	stateB, _ := s.GetState("b")
	if stateB.Status != StatusComplete {
		t.Errorf("b status = %v, want complete (should not be affected by a's failure)", stateB.Status)
	}

	// c should be blocked
	stateC, _ := s.GetState("c")
	if stateC.Status != StatusBlocked {
		t.Errorf("c status = %v, want blocked", stateC.Status)
	}
}
