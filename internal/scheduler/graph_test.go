package scheduler

import (
	"testing"

	"github.com/RevCBH/choo/internal/discovery"
)

func TestGraph_NewGraph_SimpleChain(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if graph == nil {
		t.Fatal("expected graph to be non-nil")
	}

	// Verify nodes
	if !graph.nodes["a"] || !graph.nodes["b"] || !graph.nodes["c"] {
		t.Errorf("expected all nodes to be registered")
	}

	// Verify edges
	if len(graph.edges["a"]) != 0 {
		t.Errorf("expected a to have 0 dependencies, got %d", len(graph.edges["a"]))
	}
	if len(graph.edges["b"]) != 1 || graph.edges["b"][0] != "a" {
		t.Errorf("expected b to depend on a")
	}
	if len(graph.edges["c"]) != 1 || graph.edges["c"][0] != "b" {
		t.Errorf("expected c to depend on b")
	}
}

func TestGraph_NewGraph_CycleDetected(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	graph, err := NewGraph(units)
	if graph != nil {
		t.Errorf("expected graph to be nil when cycle detected")
	}

	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	cycleErr, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected *CycleError, got %T", err)
	}

	if len(cycleErr.Cycle) == 0 {
		t.Errorf("expected cycle path to be non-empty")
	}
}

func TestGraph_NewGraph_MissingDependency(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{"nonexistent"}},
	}

	graph, err := NewGraph(units)
	if graph != nil {
		t.Errorf("expected graph to be nil when missing dependency")
	}

	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}

	missingErr, ok := err.(*MissingDependencyError)
	if !ok {
		t.Fatalf("expected *MissingDependencyError, got %T", err)
	}

	if missingErr.Unit != "a" {
		t.Errorf("expected unit to be 'a', got %q", missingErr.Unit)
	}
	if missingErr.Dependency != "nonexistent" {
		t.Errorf("expected dependency to be 'nonexistent', got %q", missingErr.Dependency)
	}
}

func TestGraph_TopologicalSort_Chain(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"a", "b", "c"}
	if len(sorted) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(sorted))
	}

	for i, id := range expected {
		if sorted[i] != id {
			t.Errorf("expected sorted[%d] = %q, got %q", i, id, sorted[i])
		}
	}
}

func TestGraph_TopologicalSort_Diamond(t *testing.T) {
	// Diamond: a -> b, a -> c, b -> d, c -> d
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(sorted))
	}

	// a must come before b and c
	aIdx := indexOf(sorted, "a")
	bIdx := indexOf(sorted, "b")
	cIdx := indexOf(sorted, "c")
	dIdx := indexOf(sorted, "d")

	if aIdx == -1 || bIdx == -1 || cIdx == -1 || dIdx == -1 {
		t.Fatalf("expected all nodes in result, got %v", sorted)
	}

	if aIdx >= bIdx {
		t.Errorf("expected a before b")
	}
	if aIdx >= cIdx {
		t.Errorf("expected a before c")
	}
	if bIdx >= dIdx {
		t.Errorf("expected b before d")
	}
	if cIdx >= dIdx {
		t.Errorf("expected c before d")
	}
}

func TestGraph_GetDependencies(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test a (no dependencies)
	aDeps := graph.GetDependencies("a")
	if len(aDeps) != 0 {
		t.Errorf("expected a to have 0 dependencies, got %d", len(aDeps))
	}

	// Test b (depends on a)
	bDeps := graph.GetDependencies("b")
	if len(bDeps) != 1 || bDeps[0] != "a" {
		t.Errorf("expected b to depend on [a], got %v", bDeps)
	}

	// Test c (depends on a and b)
	cDeps := graph.GetDependencies("c")
	if len(cDeps) != 2 {
		t.Errorf("expected c to have 2 dependencies, got %d", len(cDeps))
	}
	if !contains(cDeps, "a") || !contains(cDeps, "b") {
		t.Errorf("expected c to depend on a and b, got %v", cDeps)
	}

	// Test non-existent unit
	noDeps := graph.GetDependencies("nonexistent")
	if len(noDeps) != 0 {
		t.Errorf("expected empty slice for non-existent unit, got %v", noDeps)
	}
}

func TestGraph_GetDependents(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test a (b and c depend on it)
	aDeps := graph.GetDependents("a")
	if len(aDeps) != 2 {
		t.Errorf("expected a to have 2 dependents, got %d", len(aDeps))
	}
	if !contains(aDeps, "b") || !contains(aDeps, "c") {
		t.Errorf("expected a's dependents to be b and c, got %v", aDeps)
	}

	// Test b (d depends on it)
	bDeps := graph.GetDependents("b")
	if len(bDeps) != 1 || bDeps[0] != "d" {
		t.Errorf("expected b's dependents to be [d], got %v", bDeps)
	}

	// Test d (nothing depends on it)
	dDeps := graph.GetDependents("d")
	if len(dDeps) != 0 {
		t.Errorf("expected d to have 0 dependents, got %d", len(dDeps))
	}

	// Test non-existent unit
	noDeps := graph.GetDependents("nonexistent")
	if len(noDeps) != 0 {
		t.Errorf("expected empty slice for non-existent unit, got %v", noDeps)
	}
}

func TestGraph_GetLevels(t *testing.T) {
	units := []*discovery.Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"a", "b"}},
		{ID: "e", DependsOn: []string{"c", "d"}},
	}

	graph, err := NewGraph(units)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	levels := graph.GetLevels()

	// Expected:
	// Level 0: a, b (no dependencies)
	// Level 1: c (depends on a), d (depends on a, b)
	// Level 2: e (depends on c, d)

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}

	// Level 0
	if len(levels[0]) != 2 {
		t.Errorf("expected level 0 to have 2 units, got %d", len(levels[0]))
	}
	if !contains(levels[0], "a") || !contains(levels[0], "b") {
		t.Errorf("expected level 0 to contain a and b, got %v", levels[0])
	}

	// Level 1
	if len(levels[1]) != 2 {
		t.Errorf("expected level 1 to have 2 units, got %d", len(levels[1]))
	}
	if !contains(levels[1], "c") || !contains(levels[1], "d") {
		t.Errorf("expected level 1 to contain c and d, got %v", levels[1])
	}

	// Level 2
	if len(levels[2]) != 1 {
		t.Errorf("expected level 2 to have 1 unit, got %d", len(levels[2]))
	}
	if levels[2][0] != "e" {
		t.Errorf("expected level 2 to contain e, got %v", levels[2])
	}
}

// Helper functions

func indexOf(slice []string, value string) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

func contains(slice []string, value string) bool {
	return indexOf(slice, value) != -1
}
