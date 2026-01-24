package web

import (
	"encoding/json"
	"sync"
	"time"
)

// Store maintains the current orchestration state.
// It is safe for concurrent access.
type Store struct {
	mu             sync.RWMutex
	connectedCount int    // number of connected jobs (for concurrent job support)
	status         string // "waiting", "running", "completed", "failed"
	startedAt      time.Time
	parallelism    int
	graph          *GraphData
	units          map[string]*UnitState
	seeded         bool // true when graph/units were pre-seeded from disk
	workdir        string
	repoRoot       string
	branch         string
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
		if s.seeded {
			return
		}
		normalizeGraph(payload.Graph)
		s.graph = payload.Graph

		// Initialize unit states from graph nodes (supports resume with pre-existing statuses)
		if s.graph != nil {
			if s.units == nil {
				s.units = make(map[string]*UnitState)
			}
			for _, node := range s.graph.Nodes {
				// Use initial status from graph if provided, otherwise default to pending
				status := "pending"
				if node.Status != "" {
					status = node.Status
				}

				// For completed units, set CurrentTask to show all tasks done
				currentTask := 0
				if status == "complete" && node.Tasks > 0 {
					currentTask = node.Tasks - 1
				} else if node.CompletedTasks > 0 {
					// For in-progress resumed units, show completed task progress
					currentTask = node.CompletedTasks - 1
				} else {
					currentTask = -1
				}

				unit := &UnitState{
					ID:             node.ID,
					Status:         status,
					TotalTasks:     node.Tasks,
					CompletedTasks: node.CompletedTasks,
					CurrentTask:    currentTask,
				}
				if existing := s.units[node.ID]; existing != nil {
					unit = existing
					unit.Status = status
					unit.TotalTasks = node.Tasks
					unit.CompletedTasks = node.CompletedTasks
					unit.CurrentTask = currentTask
				}
				s.units[node.ID] = unit
			}
		}

	case "unit.queued":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "ready"
		}

	case "unit.started":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "in_progress"
			unit.Phase = ""
			unit.StartedAt = e.Time
			// Extract total_tasks from payload if available
			if e.Payload != nil {
				var payload struct {
					TotalTasks     int `json:"total_tasks"`
					CompletedTasks int `json:"completed_tasks"`
				}
				if err := json.Unmarshal(e.Payload, &payload); err == nil {
					unit.TotalTasks = payload.TotalTasks
					unit.CompletedTasks = payload.CompletedTasks
					if payload.CompletedTasks > 0 {
						unit.CurrentTask = payload.CompletedTasks - 1
					} else {
						unit.CurrentTask = -1
					}
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

	case "task.completed":
		if unit, ok := s.units[e.Unit]; ok {
			if e.Task != nil {
				if *e.Task > unit.CompletedTasks {
					unit.CompletedTasks = *e.Task
				}
			} else {
				unit.CompletedTasks++
			}
			unit.CurrentTask = -1
		}

	case "unit.completed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "complete"
			unit.Phase = ""
			// Set to last task (0-indexed) so display shows "N of N"
			if unit.TotalTasks > 0 {
				unit.CompletedTasks = unit.TotalTasks
				unit.CurrentTask = unit.TotalTasks - 1
			}
		}

	case "unit.failed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "failed"
			unit.Phase = ""
			unit.Error = e.Error
		}

	case "unit.blocked":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Status = "blocked"
			unit.Phase = ""
		}

	case "codereview.started":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "reviewing"
		}

	case "codereview.passed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "review_passed"
		}

	case "codereview.issues_found":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "review_issues"
		}

	case "codereview.fix_attempt":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "review_fixing"
		}

	case "codereview.fix_applied":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "review_fix_applied"
		}

	case "codereview.failed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "review_failed"
		}

	case "pr.created", "feature.pr.opened":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "pr_created"
		}

	case "pr.review.in_progress":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "pr_review"
		}

	case "pr.merge.queued":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "merging"
		}

	case "pr.conflict":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "merge_conflict"
		}

	case "pr.merged":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "pr_merged"
		}

	case "pr.failed":
		if unit, ok := s.units[e.Unit]; ok {
			unit.Phase = "pr_failed"
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
			ID:             unit.ID,
			Status:         unit.Status,
			Phase:          unit.Phase,
			CurrentTask:    unit.CurrentTask,
			TotalTasks:     unit.TotalTasks,
			CompletedTasks: unit.CompletedTasks,
			Error:          unit.Error,
			StartedAt:      unit.StartedAt,
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
		Connected:   s.connectedCount > 0,
		Status:      s.status,
		Parallelism: s.parallelism,
		Workdir:     s.workdir,
		RepoRoot:    s.repoRoot,
		Branch:      s.branch,
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

// SeedState pre-populates the graph and unit states before any orchestration starts.
// Used by the web server to display dependency graphs and progress without a connected run.
func (s *Store) SeedState(graph *GraphData, units []*UnitState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.graph = graph
	normalizeGraph(s.graph)
	s.seeded = (units != nil && len(units) > 0) || (graph != nil && len(graph.Nodes) > 0)

	if units != nil {
		s.units = make(map[string]*UnitState, len(units))
		for _, unit := range units {
			unitCopy := *unit
			s.units[unit.ID] = &unitCopy
		}
		return
	}

	if graph == nil {
		return
	}

	s.units = make(map[string]*UnitState, len(graph.Nodes))
	for _, node := range graph.Nodes {
		status := node.Status
		if status == "" {
			status = "pending"
		}

		currentTask := -1
		if status == "complete" && node.Tasks > 0 {
			currentTask = node.Tasks - 1
		} else if node.CompletedTasks > 0 {
			currentTask = node.CompletedTasks - 1
		}

		s.units[node.ID] = &UnitState{
			ID:             node.ID,
			Status:         status,
			CurrentTask:    currentTask,
			TotalTasks:     node.Tasks,
			CompletedTasks: node.CompletedTasks,
		}
	}
}

// SetWorkspaceInfo sets workspace metadata for display in the web UI.
func (s *Store) SetWorkspaceInfo(workdir, repoRoot, branch string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workdir = workdir
	s.repoRoot = repoRoot
	s.branch = branch
}

// SeedGraph pre-populates the graph and unit states before any orchestration starts.
// Used by the web server to display dependency graphs without a connected run.
func (s *Store) SeedGraph(graph *GraphData) {
	s.SeedState(graph, nil)
}

func normalizeGraph(graph *GraphData) {
	if graph == nil {
		return
	}
	if graph.Nodes == nil {
		graph.Nodes = []GraphNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []GraphEdge{}
	}
	if graph.Levels == nil {
		graph.Levels = [][]string{}
	}
}

func mergeGraph(existing *GraphData, incoming *GraphData) *GraphData {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	normalizeGraph(existing)
	normalizeGraph(incoming)

	nodes := make(map[string]GraphNode, len(existing.Nodes))
	for _, node := range existing.Nodes {
		nodes[node.ID] = node
	}
	for _, node := range incoming.Nodes {
		nodes[node.ID] = node
	}

	edges := make(map[string]GraphEdge, len(existing.Edges)+len(incoming.Edges))
	for _, edge := range existing.Edges {
		key := edge.From + "->" + edge.To
		edges[key] = edge
	}
	for _, edge := range incoming.Edges {
		key := edge.From + "->" + edge.To
		edges[key] = edge
	}

	mergedNodes := make([]GraphNode, 0, len(nodes))
	for _, node := range nodes {
		mergedNodes = append(mergedNodes, node)
	}

	mergedEdges := make([]GraphEdge, 0, len(edges))
	for _, edge := range edges {
		mergedEdges = append(mergedEdges, edge)
	}

	levels := existing.Levels
	if len(incoming.Levels) > len(existing.Levels) {
		levels = incoming.Levels
	}

	return &GraphData{
		Nodes:  mergedNodes,
		Edges:  mergedEdges,
		Levels: levels,
	}
}

// SetConnected updates the connection status.
// Called when orchestrator connects/disconnects from socket.
// Uses reference counting to support multiple concurrent jobs:
// - SetConnected(true) increments the count
// - SetConnected(false) decrements the count
// The store reports as "connected" when count > 0.
func (s *Store) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if connected {
		s.connectedCount++
	} else if s.connectedCount > 0 {
		s.connectedCount--
	}
}

// Reset clears all state for a new run.
// Returns store to "waiting" status with no units.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedCount = 0
	s.status = "waiting"
	s.startedAt = time.Time{}
	s.parallelism = 0
	s.graph = nil
	s.units = make(map[string]*UnitState)
}
