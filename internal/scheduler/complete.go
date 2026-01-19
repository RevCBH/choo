package scheduler

import (
	"time"

	"github.com/anthropics/choo/internal/events"
)

// Complete marks a unit as complete and re-evaluates pending units
// Emits UnitCompleted event and potentially UnitQueued for dependents
func (s *Scheduler) Complete(unitID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[unitID]
	if !exists {
		return
	}

	// Set status to complete
	now := time.Now()
	state.Status = StatusComplete
	state.CompletedAt = &now

	// Emit UnitCompleted event
	s.events.Emit(events.NewEvent(events.UnitCompleted, unitID))

	// Re-evaluate all dependents to see if they can become ready
	for _, depID := range s.graph.GetDependents(unitID) {
		s.evaluateReady(depID)
	}
}

// Fail marks a unit as failed and propagates blocked status to dependents
// Emits UnitFailed event and UnitBlocked for affected dependents
func (s *Scheduler) Fail(unitID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[unitID]
	if !exists {
		return
	}

	// Set status to failed
	now := time.Now()
	state.Status = StatusFailed
	state.CompletedAt = &now
	state.Error = err

	// Remove from ready queue if present (unit may fail before dispatch)
	s.ready.Remove(unitID)

	// Emit UnitFailed event
	s.events.Emit(events.NewEvent(events.UnitFailed, unitID).WithError(err))

	// Propagate blocked status to all dependents
	s.propagateBlocked(unitID, unitID)
}

// propagateBlocked recursively marks dependents as blocked
// Called with lock held
func (s *Scheduler) propagateBlocked(failedID, currentID string) {
	for _, depID := range s.graph.GetDependents(currentID) {
		state, exists := s.states[depID]
		if !exists {
			continue
		}

		// Skip units that are already in a terminal state
		if state.Status.IsTerminal() {
			continue
		}

		// Remove from ready queue if present
		s.ready.Remove(depID)

		// Mark as blocked
		state.Status = StatusBlocked
		state.BlockedBy = append(state.BlockedBy, failedID)
		now := time.Now()
		state.CompletedAt = &now

		// Emit UnitBlocked event with the original failed unit ID
		evt := events.NewEvent(events.UnitBlocked, depID).WithPayload(map[string]any{
			"blocked_by": failedID,
		})
		s.events.Emit(evt)

		// Recursively propagate to dependents of this unit
		s.propagateBlocked(failedID, depID)
	}
}
