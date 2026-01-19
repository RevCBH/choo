package scheduler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
)

// Graph represents the unit dependency DAG
type Graph struct {
	// nodes are unit IDs
	nodes map[string]bool

	// edges map from unit ID to its dependencies
	// edges["app-shell"] = ["project-setup", "config"]
	edges map[string][]string

	// dependents is reverse edges for dependent lookup
	// dependents["config"] = ["app-shell", "deck-list"]
	dependents map[string][]string
}

// CycleError indicates a circular dependency was detected
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("circular dependency detected: %s", strings.Join(e.Cycle, " -> "))
}

// MissingDependencyError indicates a referenced dependency doesn't exist
type MissingDependencyError struct {
	Unit       string
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("unit %q depends on non-existent unit %q", e.Unit, e.Dependency)
}

// NewGraph constructs a dependency graph from units
// Returns error if cycles or missing dependencies are detected
func NewGraph(units []*discovery.Unit) (*Graph, error) {
	g := &Graph{
		nodes:      make(map[string]bool),
		edges:      make(map[string][]string),
		dependents: make(map[string][]string),
	}

	// First pass: register all nodes
	for _, unit := range units {
		g.nodes[unit.ID] = true
	}

	// Second pass: build edges and check for missing dependencies
	for _, unit := range units {
		// Initialize edge lists
		g.edges[unit.ID] = make([]string, len(unit.DependsOn))
		copy(g.edges[unit.ID], unit.DependsOn)

		// Check for missing dependencies
		for _, dep := range unit.DependsOn {
			if !g.nodes[dep] {
				return nil, &MissingDependencyError{
					Unit:       unit.ID,
					Dependency: dep,
				}
			}

			// Build reverse edges
			g.dependents[dep] = append(g.dependents[dep], unit.ID)
		}
	}

	// Check for cycles using topological sort
	_, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	return g, nil
}

// TopologicalSort returns unit IDs in valid execution order
// Uses Kahn's algorithm for cycle detection
func (g *Graph) TopologicalSort() ([]string, error) {
	// Calculate in-degrees (number of dependencies each node has)
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = len(g.edges[node])
	}

	// Find all nodes with in-degree 0
	var queue []string
	for node := range g.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	// Sort queue for deterministic ordering
	sort.Strings(queue)

	var result []string

	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Get dependents of current node
		dependents := g.dependents[current]
		// Sort for deterministic ordering
		sortedDependents := make([]string, len(dependents))
		copy(sortedDependents, dependents)
		sort.Strings(sortedDependents)

		for _, dependent := range sortedDependents {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}

		// Re-sort queue to maintain deterministic ordering
		sort.Strings(queue)
	}

	// If we didn't visit all nodes, there's a cycle
	if len(result) != len(g.nodes) {
		cycle := g.findCycle()
		return nil, &CycleError{Cycle: cycle}
	}

	return result, nil
}

// GetDependencies returns the direct dependencies of a unit
func (g *Graph) GetDependencies(unitID string) []string {
	deps := g.edges[unitID]
	if deps == nil {
		return []string{}
	}
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// GetDependents returns units that depend on the given unit
func (g *Graph) GetDependents(unitID string) []string {
	deps := g.dependents[unitID]
	if deps == nil {
		return []string{}
	}
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// GetLevels returns units grouped by dependency depth
// Level 0 contains units with no dependencies
func (g *Graph) GetLevels() [][]string {
	// Calculate in-degrees (number of dependencies)
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = len(g.edges[node])
	}

	var levels [][]string
	visited := make(map[string]bool)

	for len(visited) < len(g.nodes) {
		var currentLevel []string

		// Find all unvisited nodes whose dependencies have been visited
		for node := range g.nodes {
			if visited[node] {
				continue
			}

			allDepsVisited := true
			for _, dep := range g.edges[node] {
				if !visited[dep] {
					allDepsVisited = false
					break
				}
			}

			if allDepsVisited {
				currentLevel = append(currentLevel, node)
			}
		}

		// Sort for deterministic ordering
		sort.Strings(currentLevel)

		// Mark as visited
		for _, node := range currentLevel {
			visited[node] = true
		}

		levels = append(levels, currentLevel)
	}

	return levels
}

// findCycle locates and returns a cycle path (internal helper)
func (g *Graph) findCycle() []string {
	const (
		white = 0 // unvisited
		gray  = 1 // visiting
		black = 2 // visited
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	for node := range g.nodes {
		color[node] = white
	}

	var cycle []string
	var dfs func(string) bool

	dfs = func(node string) bool {
		color[node] = gray

		// Visit dependents (reversed direction for cycle finding)
		dependents := g.dependents[node]
		sortedDependents := make([]string, len(dependents))
		copy(sortedDependents, dependents)
		sort.Strings(sortedDependents)

		for _, dep := range sortedDependents {
			if color[dep] == gray {
				// Found cycle, reconstruct path
				cycle = []string{dep}
				current := node
				for current != dep {
					cycle = append([]string{current}, cycle...)
					current = parent[current]
				}
				cycle = append(cycle, dep) // close the cycle
				return true
			}

			if color[dep] == white {
				parent[dep] = node
				if dfs(dep) {
					return true
				}
			}
		}

		color[node] = black
		return false
	}

	// Try DFS from each unvisited node
	var sortedNodes []string
	for node := range g.nodes {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)

	for _, node := range sortedNodes {
		if color[node] == white {
			if dfs(node) {
				return cycle
			}
		}
	}

	return nil
}
