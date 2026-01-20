package web

import (
	"encoding/json"
	"sync"
	"time"
)

// Store maintains the current orchestration state.
// It is safe for concurrent access.
type Store struct {
	mu          sync.RWMutex
	connected   bool              // true when orchestrator is connected
	status      string            // "waiting", "running", "completed", "failed"
	startedAt   time.Time
	parallelism int
	graph       *GraphData
	units       map[string]*UnitState
}

// NewStore creates an empty state store in "waiting" status.
func NewStore() *Store {
	return &Store{
		status: "waiting",
		units:  make(map[string]*UnitState),
	}
}

// HandleEvent processes an event and updates state accordingly.
// Thread-safe. Event type determines state transition:
//   - orch.started: set status="running", store graph, init units
//   - unit.queued: set unit status to "ready"
//   - unit.started: set unit status to "in_progress", set startedAt
//   - task.started: increment currentTask
//   - unit.completed: set unit status to "complete"
//   - unit.failed: set unit status to "failed", store error
//   - unit.blocked: set unit status to "blocked"
//   - orch.completed: set status="completed"
//   - orch.failed: set status="failed"
func (s *Store) HandleEvent(e *Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch e.Type {
	case "orch.started":
		var payload OrchestratorPayload
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return
		}
		s.status = "running"
		s.startedAt = e.Time
		s.parallelism = payload.Parallelism
		s.graph = payload.Graph

		// Initialize unit states from graph nodes
		if s.graph != nil {
			for _, node := range s.graph.Nodes {
				s.units[node.ID] = &UnitState{
					ID:         node.ID,
					Status:     "pending",
					TotalTasks: node.Tasks,
				}
			}
		}

	case "unit.queued":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "ready"
		}

	case "unit.started":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "in_progress"
			unit.StartedAt = e.Time
			// Extract total_tasks from payload if available
			if e.Payload != nil {
				var payload struct {
					TotalTasks     int `json:"total_tasks"`
					CompletedTasks int `json:"completed_tasks"`
				}
				if err := json.Unmarshal(e.Payload, &payload); err == nil {
					unit.TotalTasks = payload.TotalTasks
					unit.CurrentTask = payload.CompletedTasks
				}
			}
		}

	case "task.started":
		if unit, ok := s.units[e.Unit]; ok {
			// Use task number from event if provided (convert 1-indexed to 0-indexed)
			if e.Task != nil {
				unit.CurrentTask = *e.Task - 1
			} else {
				unit.CurrentTask++
			}
		}

	case "unit.completed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "complete"
		}

	case "unit.failed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "failed"
			unit.Error = e.Error
		}

	case "unit.blocked":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "blocked"
		}

	case "orch.completed":
		s.status = "completed"

	case "orch.failed":
		s.status = "failed"
	}
}

// Snapshot returns the current state as a StateSnapshot.
// Thread-safe for concurrent reads.
func (s *Store) Snapshot() *StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy of units slice
	units := make([]*UnitState, 0, len(s.units))
	for _, unit := range s.units {
		// Copy the unit state
		unitCopy := &UnitState{
			ID:          unit.ID,
			Status:      unit.Status,
			CurrentTask: unit.CurrentTask,
			TotalTasks:  unit.TotalTasks,
			Error:       unit.Error,
			StartedAt:   unit.StartedAt,
		}
		units = append(units, unitCopy)
	}

	// Calculate summary
	summary := StateSummary{
		Total: len(units),
	}
	for _, unit := range s.units {
		switch unit.Status {
		case "pending":
			summary.Pending++
		case "in_progress":
			summary.InProgress++
		case "complete":
			summary.Complete++
		case "failed":
			summary.Failed++
		case "blocked":
			summary.Blocked++
		}
	}

	snapshot := &StateSnapshot{
		Connected:   s.connected,
		Status:      s.status,
		Parallelism: s.parallelism,
		Units:       units,
		Summary:     summary,
	}

	// Only set StartedAt if it's not zero
	if !s.startedAt.IsZero() {
		snapshot.StartedAt = &s.startedAt
	}

	return snapshot
}

// Graph returns the dependency graph, or nil if not yet received.
// Thread-safe.
func (s *Store) Graph() *GraphData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

// SetConnected updates the connection status.
// Called when orchestrator connects/disconnects from socket.
func (s *Store) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = connected
}

// Reset clears all state for a new run.
// Returns store to "waiting" status with no units.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = false
	s.status = "waiting"
	s.startedAt = time.Time{}
	s.parallelism = 0
	s.graph = nil
	s.units = make(map[string]*UnitState)
}
