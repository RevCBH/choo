package web

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestStore_NewStore(t *testing.T) {
	store := NewStore()

	if store.status != "waiting" {
		t.Errorf("expected status 'waiting', got '%s'", store.status)
	}

	if store.connectedCount != 0 {
		t.Error("expected connectedCount to be 0")
	}

	if store.units == nil {
		t.Error("expected units map to be initialized")
	}

	if len(store.units) != 0 {
		t.Errorf("expected empty units, got %d units", len(store.units))
	}
}

func TestStore_HandleOrchStarted(t *testing.T) {
	store := NewStore()

	graph := &GraphData{
		Nodes: []GraphNode{
			{ID: "unit1", Level: 0},
			{ID: "unit2", Level: 1},
		},
		Edges: []GraphEdge{
			{From: "unit1", To: "unit2"},
		},
		Levels: [][]string{
			{"unit1"},
			{"unit2"},
		},
	}

	payload := OrchestratorPayload{
		UnitCount:   2,
		Parallelism: 3,
		Graph:       graph,
	}

	payloadJSON, _ := json.Marshal(payload)

	event := &Event{
		Type:    "orch.started",
		Time:    time.Now(),
		Payload: payloadJSON,
	}

	store.HandleEvent(event)

	if store.status != "running" {
		t.Errorf("expected status 'running', got '%s'", store.status)
	}

	if store.parallelism != 3 {
		t.Errorf("expected parallelism 3, got %d", store.parallelism)
	}

	if store.graph == nil {
		t.Fatal("expected graph to be stored")
	}

	if len(store.units) != 2 {
		t.Errorf("expected 2 units, got %d", len(store.units))
	}

	if unit1, ok := store.units["unit1"]; !ok {
		t.Error("expected unit1 to be initialized")
	} else if unit1.Status != "pending" {
		t.Errorf("expected unit1 status 'pending', got '%s'", unit1.Status)
	}

	if unit2, ok := store.units["unit2"]; !ok {
		t.Error("expected unit2 to be initialized")
	} else if unit2.Status != "pending" {
		t.Errorf("expected unit2 status 'pending', got '%s'", unit2.Status)
	}
}

func TestStore_HandleOrchStarted_MergesExistingGraph(t *testing.T) {
	store := NewStore()

	store.SeedState(&GraphData{
		Nodes: []GraphNode{
			{ID: "unit1", Level: 0, Status: "complete", Tasks: 2, CompletedTasks: 2},
			{ID: "unit2", Level: 1, Status: "pending", Tasks: 1},
		},
		Edges: []GraphEdge{
			{From: "unit1", To: "unit2"},
		},
		Levels: [][]string{
			{"unit1"},
			{"unit2"},
		},
	}, nil)

	incoming := OrchestratorPayload{
		UnitCount:   1,
		Parallelism: 2,
		Graph: &GraphData{
			Nodes: []GraphNode{
				{ID: "unit2", Level: 0, Status: "pending", Tasks: 1},
			},
			Edges:  []GraphEdge{},
			Levels: [][]string{{"unit2"}},
		},
	}

	payloadJSON, _ := json.Marshal(incoming)
	store.HandleEvent(&Event{
		Type:    "orch.started",
		Time:    time.Now(),
		Payload: payloadJSON,
	})

	if store.graph == nil || len(store.graph.Nodes) != 2 {
		t.Fatalf("expected seeded graph to remain with 2 nodes, got %v", store.graph)
	}
	if unit1, ok := store.units["unit1"]; !ok || unit1.Status != "complete" {
		t.Errorf("expected unit1 to remain complete, got %+v", unit1)
	}
}

func TestStore_HandleUnitLifecycle(t *testing.T) {
	store := NewStore()

	// Initialize a unit
	store.units["unit1"] = &UnitState{
		ID:     "unit1",
		Status: "pending",
	}

	// Test unit.queued
	event := &Event{
		Type: "unit.queued",
		Time: time.Now(),
		Unit: "unit1",
	}
	store.HandleEvent(event)

	if store.units["unit1"].Status != "ready" {
		t.Errorf("expected status 'ready' after queued, got '%s'", store.units["unit1"].Status)
	}

	// Test unit.started
	startTime := time.Now()
	event = &Event{
		Type: "unit.started",
		Time: startTime,
		Unit: "unit1",
	}
	store.HandleEvent(event)

	if store.units["unit1"].Status != "in_progress" {
		t.Errorf("expected status 'in_progress' after started, got '%s'", store.units["unit1"].Status)
	}

	if store.units["unit1"].StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}

	// Test unit.completed
	event = &Event{
		Type: "unit.completed",
		Time: time.Now(),
		Unit: "unit1",
	}
	store.HandleEvent(event)

	if store.units["unit1"].Status != "complete" {
		t.Errorf("expected status 'complete' after completed, got '%s'", store.units["unit1"].Status)
	}

	// Test unit.failed on another unit
	store.units["unit2"] = &UnitState{
		ID:     "unit2",
		Status: "in_progress",
	}

	event = &Event{
		Type:  "unit.failed",
		Time:  time.Now(),
		Unit:  "unit2",
		Error: "test error message",
	}
	store.HandleEvent(event)

	if store.units["unit2"].Status != "failed" {
		t.Errorf("expected status 'failed' after failed, got '%s'", store.units["unit2"].Status)
	}

	if store.units["unit2"].Error != "test error message" {
		t.Errorf("expected error 'test error message', got '%s'", store.units["unit2"].Error)
	}
}

func TestStore_HandleTaskStarted(t *testing.T) {
	store := NewStore()

	store.units["unit1"] = &UnitState{
		ID:          "unit1",
		Status:      "in_progress",
		CurrentTask: 0,
	}

	event := &Event{
		Type: "task.started",
		Time: time.Now(),
		Unit: "unit1",
	}

	store.HandleEvent(event)

	if store.units["unit1"].CurrentTask != 1 {
		t.Errorf("expected CurrentTask 1, got %d", store.units["unit1"].CurrentTask)
	}

	// Handle another task.started
	store.HandleEvent(event)

	if store.units["unit1"].CurrentTask != 2 {
		t.Errorf("expected CurrentTask 2, got %d", store.units["unit1"].CurrentTask)
	}
}

func TestStore_HandleOrchCompleted(t *testing.T) {
	store := NewStore()
	store.status = "running"

	event := &Event{
		Type: "orch.completed",
		Time: time.Now(),
	}

	store.HandleEvent(event)

	if store.status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", store.status)
	}
}

func TestStore_HandleOrchFailed(t *testing.T) {
	store := NewStore()
	store.status = "running"

	event := &Event{
		Type: "orch.failed",
		Time: time.Now(),
	}

	store.HandleEvent(event)

	if store.status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", store.status)
	}
}

func TestStore_SummaryCalculation(t *testing.T) {
	store := NewStore()

	store.units["unit1"] = &UnitState{ID: "unit1", Status: "pending"}
	store.units["unit2"] = &UnitState{ID: "unit2", Status: "in_progress"}
	store.units["unit3"] = &UnitState{ID: "unit3", Status: "complete"}
	store.units["unit4"] = &UnitState{ID: "unit4", Status: "failed"}
	store.units["unit5"] = &UnitState{ID: "unit5", Status: "blocked"}

	snapshot := store.Snapshot()

	if snapshot.Summary.Total != 5 {
		t.Errorf("expected total 5, got %d", snapshot.Summary.Total)
	}

	if snapshot.Summary.Pending != 1 {
		t.Errorf("expected pending 1, got %d", snapshot.Summary.Pending)
	}

	if snapshot.Summary.InProgress != 1 {
		t.Errorf("expected inProgress 1, got %d", snapshot.Summary.InProgress)
	}

	if snapshot.Summary.Complete != 1 {
		t.Errorf("expected complete 1, got %d", snapshot.Summary.Complete)
	}

	if snapshot.Summary.Failed != 1 {
		t.Errorf("expected failed 1, got %d", snapshot.Summary.Failed)
	}

	if snapshot.Summary.Blocked != 1 {
		t.Errorf("expected blocked 1, got %d", snapshot.Summary.Blocked)
	}
}

func TestStore_SetConnected(t *testing.T) {
	store := NewStore()

	if store.connectedCount != 0 {
		t.Error("expected initial connectedCount to be 0")
	}

	// Test reference counting - first connection
	store.SetConnected(true)

	if store.connectedCount != 1 {
		t.Errorf("expected connectedCount 1 after first SetConnected(true), got %d", store.connectedCount)
	}

	// Second connection (concurrent job)
	store.SetConnected(true)

	if store.connectedCount != 2 {
		t.Errorf("expected connectedCount 2 after second SetConnected(true), got %d", store.connectedCount)
	}

	// First job disconnects
	store.SetConnected(false)

	if store.connectedCount != 1 {
		t.Errorf("expected connectedCount 1 after first SetConnected(false), got %d", store.connectedCount)
	}

	// Verify snapshot still shows connected
	snapshot := store.Snapshot()
	if !snapshot.Connected {
		t.Error("expected snapshot.Connected to be true with connectedCount > 0")
	}

	// Second job disconnects
	store.SetConnected(false)

	if store.connectedCount != 0 {
		t.Errorf("expected connectedCount 0 after second SetConnected(false), got %d", store.connectedCount)
	}

	// Verify snapshot shows disconnected
	snapshot = store.Snapshot()
	if snapshot.Connected {
		t.Error("expected snapshot.Connected to be false with connectedCount == 0")
	}

	// Verify we don't go negative
	store.SetConnected(false)
	if store.connectedCount != 0 {
		t.Errorf("expected connectedCount to stay at 0, got %d", store.connectedCount)
	}
}

func TestStore_Reset(t *testing.T) {
	store := NewStore()

	// Set up some state
	store.connectedCount = 2
	store.status = "running"
	store.startedAt = time.Now()
	store.parallelism = 5
	store.graph = &GraphData{}
	store.units["unit1"] = &UnitState{ID: "unit1", Status: "in_progress"}

	store.Reset()

	if store.connectedCount != 0 {
		t.Errorf("expected connectedCount to be 0 after Reset, got %d", store.connectedCount)
	}

	if store.status != "waiting" {
		t.Errorf("expected status 'waiting' after Reset, got '%s'", store.status)
	}

	if !store.startedAt.IsZero() {
		t.Error("expected startedAt to be zero after Reset")
	}

	if store.parallelism != 0 {
		t.Errorf("expected parallelism 0 after Reset, got %d", store.parallelism)
	}

	if store.graph != nil {
		t.Error("expected graph to be nil after Reset")
	}

	if len(store.units) != 0 {
		t.Errorf("expected empty units after Reset, got %d units", len(store.units))
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore()

	// Initialize some units
	graph := &GraphData{
		Nodes: []GraphNode{
			{ID: "unit1", Level: 0},
			{ID: "unit2", Level: 0},
			{ID: "unit3", Level: 0},
		},
	}

	payload := OrchestratorPayload{
		UnitCount:   3,
		Parallelism: 2,
		Graph:       graph,
	}

	payloadJSON, _ := json.Marshal(payload)

	event := &Event{
		Type:    "orch.started",
		Time:    time.Now(),
		Payload: payloadJSON,
	}

	store.HandleEvent(event)

	// Spawn multiple goroutines to read and write concurrently
	var wg sync.WaitGroup
	iterations := 100

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				event := &Event{
					Type: "unit.started",
					Time: time.Now(),
					Unit: "unit1",
				}
				store.HandleEvent(event)
			}
		}()
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = store.Snapshot()
				_ = store.Graph()
			}
		}()
	}

	// SetConnected calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				store.SetConnected(i%2 == 0)
			}
		}(i)
	}

	wg.Wait()

	// Verify the store is still in a valid state
	snapshot := store.Snapshot()
	if snapshot == nil {
		t.Error("expected non-nil snapshot after concurrent access")
	}
}
