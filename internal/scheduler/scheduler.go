package scheduler

import (
	"fmt"
	"sort"
	"sync"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
)

// Scheduler manages unit execution order and dispatch
type Scheduler struct {
	maxParallelism int
	graph          *Graph
	states         map[string]*UnitState
	ready          *ReadyQueue
	events         *events.Bus
	mu             sync.RWMutex
}

// Schedule represents the execution plan
type Schedule struct {
	TopologicalOrder []string
	Levels           [][]string
	MaxParallelism   int
}

// New creates a new Scheduler with the given event bus and parallelism limit
func New(events *events.Bus, maxParallelism int) *Scheduler {
	return &Scheduler{
		maxParallelism: maxParallelism,
		events:         events,
		states:         make(map[string]*UnitState),
		ready:          NewReadyQueue(),
	}
}

// Schedule builds the execution plan from discovered units
// Returns error if dependencies are invalid (cycles, missing refs)
// Initializes all units as pending and evaluates initial ready set
func (s *Scheduler) Schedule(units []*discovery.Unit) (*Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	missing := filterMissingUnitDeps(units)
	if len(missing) > 0 && s.events != nil {
		for unitID, deps := range missing {
			payload := map[string]any{"missing": deps}
			s.events.Emit(events.NewEvent(events.UnitDependencyMissing, unitID).WithPayload(payload))
		}
	}

	// Build dependency graph
	graph, err := NewGraph(units)
	if err != nil {
		return nil, err
	}
	s.graph = graph

	// Get topological order and levels
	topoOrder, err := graph.TopologicalSort()
	if err != nil {
		return nil, err
	}
	levels := graph.GetLevels()

	// Initialize all units as pending
	for _, unit := range units {
		s.states[unit.ID] = NewUnitState(unit.ID)
	}

	// Evaluate initial ready set (units with no dependencies)
	for _, unit := range units {
		if len(unit.DependsOn) == 0 {
			s.evaluateReady(unit.ID)
		}
	}

	return &Schedule{
		TopologicalOrder: topoOrder,
		Levels:           levels,
		MaxParallelism:   s.maxParallelism,
	}, nil
}

func filterMissingUnitDeps(units []*discovery.Unit) map[string][]string {
	valid := make(map[string]bool, len(units))
	for _, unit := range units {
		valid[unit.ID] = true
	}

	missing := make(map[string][]string)
	for _, unit := range units {
		var filtered []string
		var missingDeps []string
		for _, dep := range unit.DependsOn {
			if !valid[dep] {
				missingDeps = append(missingDeps, dep)
				continue
			}
			filtered = append(filtered, dep)
		}
		if len(missingDeps) > 0 {
			sort.Strings(missingDeps)
			missing[unit.ID] = missingDeps
		}
		if len(filtered) != len(unit.DependsOn) {
			unit.DependsOn = filtered
		}
	}

	return missing
}

// Transition moves a unit to a new status
// Returns error if the transition is invalid
// Emits appropriate event on successful transition
func (s *Scheduler) Transition(unitID string, to UnitStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.states[unitID]
	if !exists {
		return fmt.Errorf("unit %q not found", unitID)
	}

	if !CanTransition(state.Status, to) {
		return fmt.Errorf("invalid transition from %s to %s", state.Status, to)
	}

	from := state.Status
	state.Status = to

	// Update ready queue based on transition
	if to == StatusReady {
		s.ready.Push(unitID)
	} else if from == StatusReady && to != StatusReady {
		s.ready.Remove(unitID)
	}

	// Emit appropriate event
	var eventType events.EventType
	switch to {
	case StatusReady:
		eventType = events.UnitQueued
	case StatusInProgress:
		eventType = events.UnitStarted
	case StatusComplete:
		eventType = events.UnitCompleted
	case StatusFailed:
		eventType = events.UnitFailed
	}

	if eventType != "" {
		s.events.Emit(events.NewEvent(eventType, unitID))
	}

	return nil
}

// GetState returns a snapshot of a unit's current state
// Returns a copy to prevent data races with concurrent modifications
func (s *Scheduler) GetState(unitID string) (*UnitState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[unitID]
	if !ok {
		return nil, false
	}

	// Return a copy to prevent external mutation and data races
	stateCopy := *state
	if state.BlockedBy != nil {
		stateCopy.BlockedBy = make([]string, len(state.BlockedBy))
		copy(stateCopy.BlockedBy, state.BlockedBy)
	}
	return &stateCopy, true
}

// GetAllStates returns a snapshot of all unit states
func (s *Scheduler) GetAllStates() map[string]*UnitState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a deep copy to prevent external mutation
	result := make(map[string]*UnitState)
	for id, state := range s.states {
		stateCopy := *state
		// Copy slices
		if state.BlockedBy != nil {
			stateCopy.BlockedBy = make([]string, len(state.BlockedBy))
			copy(stateCopy.BlockedBy, state.BlockedBy)
		}
		result[id] = &stateCopy
	}
	return result
}

// ReadyQueue returns the current list of ready unit IDs
func (s *Scheduler) ReadyQueue() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ready.List()
}

// ActiveCount returns the number of units consuming parallelism slots
func (s *Scheduler) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, state := range s.states {
		if state.Status.IsActive() {
			count++
		}
	}
	return count
}

// IsComplete returns true if all units have reached terminal states
func (s *Scheduler) IsComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, state := range s.states {
		if !state.Status.IsTerminal() {
			return false
		}
	}
	return true
}

// HasFailures returns true if any units failed or are blocked
func (s *Scheduler) HasFailures() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, state := range s.states {
		if state.Status == StatusFailed || state.Status == StatusBlocked {
			return true
		}
	}
	return false
}

// evaluateReady checks if a unit's deps are satisfied and moves to ready
// Called with lock held
func (s *Scheduler) evaluateReady(unitID string) {
	state, exists := s.states[unitID]
	if !exists {
		return
	}

	// Only evaluate pending units
	if state.Status != StatusPending {
		return
	}

	// Check if all dependencies are complete
	deps := s.graph.GetDependencies(unitID)
	for _, depID := range deps {
		depState, ok := s.states[depID]
		if !ok || depState.Status != StatusComplete {
			return
		}
	}

	// All dependencies satisfied, move to ready
	state.Status = StatusReady
	s.ready.Push(unitID)

	// Emit UnitQueued event
	s.events.Emit(events.NewEvent(events.UnitQueued, unitID))
}
