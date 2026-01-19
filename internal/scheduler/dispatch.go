package scheduler

import (
	"time"

	"github.com/RevCBH/choo/internal/events"
)

// DispatchResult represents the outcome of a dispatch attempt
type DispatchResult struct {
	Unit       string
	Dispatched bool
	Reason     DispatchBlockReason
}

// DispatchBlockReason explains why dispatch didn't occur
type DispatchBlockReason string

const (
	ReasonNone        DispatchBlockReason = ""
	ReasonNoReady     DispatchBlockReason = "no_ready_units"
	ReasonAtCapacity  DispatchBlockReason = "at_capacity"
	ReasonAllComplete DispatchBlockReason = "all_complete"
	ReasonAllBlocked  DispatchBlockReason = "all_blocked"
)

// Dispatch attempts to dispatch the next ready unit
// Returns the dispatched unit ID or empty if none available
// Respects parallelism limit and emits UnitStarted event
func (s *Scheduler) Dispatch() DispatchResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check parallelism limit first
	activeCount := 0
	for _, state := range s.states {
		if state.Status.IsActive() {
			activeCount++
		}
	}

	if activeCount >= s.maxParallelism {
		return DispatchResult{
			Dispatched: false,
			Reason:     ReasonAtCapacity,
		}
	}

	// Try to pop from ready queue
	unitID := s.ready.Pop()
	if unitID == "" {
		// No ready units, check why
		if s.allBlockedOrComplete() {
			// Check if all complete or all blocked
			allComplete := true
			for _, state := range s.states {
				if state.Status != StatusComplete {
					allComplete = false
					break
				}
			}
			if allComplete {
				return DispatchResult{
					Dispatched: false,
					Reason:     ReasonAllComplete,
				}
			}
			return DispatchResult{
				Dispatched: false,
				Reason:     ReasonAllBlocked,
			}
		}
		return DispatchResult{
			Dispatched: false,
			Reason:     ReasonNoReady,
		}
	}

	// Dispatch the unit
	state := s.states[unitID]

	// Set StartedAt timestamp
	now := time.Now()
	state.StartedAt = &now

	// Transition to in_progress (this will emit UnitStarted event and update ready queue)
	state.Status = StatusInProgress

	// Remove from ready queue (already popped above)
	// Emit UnitStarted event
	s.events.Emit(events.NewEvent(events.UnitStarted, unitID))

	return DispatchResult{
		Unit:       unitID,
		Dispatched: true,
		Reason:     ReasonNone,
	}
}

// allBlockedOrComplete checks if remaining units are all blocked/complete
// Called with lock held
func (s *Scheduler) allBlockedOrComplete() bool {
	for _, state := range s.states {
		// If any unit is pending or ready, we're not all blocked/complete
		if state.Status == StatusPending || state.Status == StatusReady {
			return false
		}
		// If any unit is active, we're not all blocked/complete
		if state.Status.IsActive() {
			return false
		}
	}
	return true
}
