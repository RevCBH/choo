---
task: 1
status: complete
backpressure: "go test ./internal/scheduler/... -run TestGraph"
depends_on: []
---

# Graph Types and Construction

**Parent spec**: `/specs/SCHEDULER.md`
**Task**: #1 of 6 in implementation plan

## Objective

Implement the dependency graph data structure with construction, cycle detection, and topological sorting.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `*discovery.Unit` type with `ID` and `DependsOn` fields

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- Standard library only (`fmt`, `strings`)

## Deliverables

### Files to Create/Modify

```
internal/
└── scheduler/
    ├── graph.go       # CREATE: Graph type and algorithms
    └── graph_test.go  # CREATE: Graph unit tests
```

### Types to Implement

```go
// internal/scheduler/graph.go

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

func (e *CycleError) Error() string

// MissingDependencyError indicates a referenced dependency doesn't exist
type MissingDependencyError struct {
    Unit       string
    Dependency string
}

func (e *MissingDependencyError) Error() string
```

### Functions to Implement

```go
// NewGraph constructs a dependency graph from units
// Returns error if cycles or missing dependencies are detected
func NewGraph(units []*discovery.Unit) (*Graph, error)

// TopologicalSort returns unit IDs in valid execution order
// Uses Kahn's algorithm for cycle detection
func (g *Graph) TopologicalSort() ([]string, error)

// GetDependencies returns the direct dependencies of a unit
func (g *Graph) GetDependencies(unitID string) []string

// GetDependents returns units that depend on the given unit
func (g *Graph) GetDependents(unitID string) []string

// GetLevels returns units grouped by dependency depth
// Level 0 contains units with no dependencies
func (g *Graph) GetLevels() [][]string

// findCycle locates and returns a cycle path (internal helper)
func (g *Graph) findCycle() []string
```

## Backpressure

### Validation Command

```bash
go test ./internal/scheduler/... -run TestGraph -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGraph_NewGraph_SimpleChain` | Graph builds for a -> b -> c chain |
| `TestGraph_NewGraph_CycleDetected` | Returns `*CycleError` for a -> b -> c -> a |
| `TestGraph_NewGraph_MissingDependency` | Returns `*MissingDependencyError` for unknown ref |
| `TestGraph_TopologicalSort_Chain` | Returns [a, b, c] for chain |
| `TestGraph_TopologicalSort_Diamond` | a before b,c; b,c before d |
| `TestGraph_GetDependencies` | Returns correct direct deps |
| `TestGraph_GetDependents` | Returns correct reverse deps |
| `TestGraph_GetLevels` | Groups by depth correctly |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Tests use inline unit construction |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use Kahn's algorithm for topological sort (provides cycle detection for free)
- If TopologicalSort visits fewer nodes than exist, there's a cycle
- The `findCycle` helper uses DFS with coloring to locate the actual cycle path
- Sort nodes deterministically (alphabetically) when multiple have same in-degree for reproducibility

## NOT In Scope

- Thread safety (Graph is built once and not mutated)
- State tracking (Task #2)
- Ready queue (Task #3)
- Event emission (Task #4)
