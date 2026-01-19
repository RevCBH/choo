---
task: 4
status: complete
backpressure: "go test ./internal/discovery/... -run TestValidate"
depends_on: [1, 2, 3]
---

# Validation

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md`
**Task**: #4 of 4 in implementation plan

## Objective

Implement validation logic and error aggregation for discovered units and tasks.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Unit`, `Task` types)
- Task #2 must be complete (provides: frontmatter types)
- Task #3 must be complete (provides: `Discover`, `DiscoverUnit` functions)

### Package Dependencies
- Standard library (`fmt`, `strings`)

## Deliverables

### Files to Create/Modify

```
internal/
└── discovery/
    ├── validation.go       # CREATE: Validation logic
    └── validation_test.go  # CREATE: Validation tests
```

### Types to Implement

```go
// ValidationError represents a single validation failure
type ValidationError struct {
    Unit    string // unit ID (empty for cross-unit errors)
    Task    *int   // task number (nil for unit-level errors)
    File    string // file path where error occurred
    Field   string // field name that failed validation
    Message string // human-readable error description
}

// ValidationResult collects all validation errors
type ValidationResult struct {
    Errors []ValidationError
}

// IsValid returns true if no validation errors occurred
func (r *ValidationResult) IsValid() bool

// Error returns a formatted string of all validation errors
func (r *ValidationResult) Error() string

// Add appends an error to the result
func (r *ValidationResult) Add(err ValidationError)

// Merge combines another ValidationResult into this one
func (r *ValidationResult) Merge(other *ValidationResult)
```

### Functions to Implement

```go
// ValidateUnit validates a single unit's structure
func ValidateUnit(unit *Unit) *ValidationResult

// ValidateUnits validates all units including cross-unit dependencies
func ValidateUnits(units []*Unit) *ValidationResult

// ValidateTaskSequence ensures task numbers are sequential from 1
func ValidateTaskSequence(tasks []*Task) *ValidationResult

// ValidateTaskDependencies ensures task depends_on references are valid
func ValidateTaskDependencies(tasks []*Task) *ValidationResult

// ValidateUnitDependencies ensures unit depends_on references exist
func ValidateUnitDependencies(units []*Unit) *ValidationResult

// DetectCycles checks for circular dependencies in the unit graph
func DetectCycles(units []*Unit) *ValidationResult
```

## Backpressure

### Validation Command

```bash
go test ./internal/discovery/... -run TestValidate -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestValidateUnit_MissingUnitField` | Error when unit frontmatter missing `unit` field |
| `TestValidateUnit_MissingTaskField` | Error when task missing `task` number |
| `TestValidateUnit_MissingBackpressure` | Error when task missing `backpressure` |
| `TestValidateTaskSequence_Valid` | No errors for 1, 2, 3 sequence |
| `TestValidateTaskSequence_StartsFrom2` | Error for sequence starting at 2 |
| `TestValidateTaskSequence_Gap` | Error for 1, 3 (missing 2) |
| `TestValidateTaskSequence_Duplicate` | Error for duplicate task numbers |
| `TestValidateTaskDependencies_Valid` | No errors for valid references |
| `TestValidateTaskDependencies_InvalidRef` | Error for depends_on referencing non-existent task |
| `TestValidateTaskDependencies_SelfRef` | Error for task depending on itself |
| `TestValidateUnitDependencies_Valid` | No errors for valid unit references |
| `TestValidateUnitDependencies_Missing` | Error for depends_on referencing non-existent unit |
| `TestDetectCycles_NoCycle` | No errors for linear chain |
| `TestDetectCycles_SimpleCycle` | Error for A -> B -> A |
| `TestDetectCycles_ThreeNodeCycle` | Error for A -> B -> C -> A |
| `TestDetectCycles_CycleMessage` | Error message shows cycle path |
| `TestValidationResult_Error` | Formats multiple errors nicely |
| `TestValidationResult_IsValid` | Returns true when no errors |

### Test Fixtures

Test cases use in-memory Unit and Task objects:

```go
// Valid sequence
[]*Task{
    {Number: 1, FilePath: "01-a.md"},
    {Number: 2, FilePath: "02-b.md"},
    {Number: 3, FilePath: "03-c.md"},
}

// Gap in sequence
[]*Task{
    {Number: 1, FilePath: "01-a.md"},
    {Number: 3, FilePath: "03-c.md"},
}

// Simple cycle
[]*Unit{
    {ID: "a", DependsOn: []string{"b"}},
    {ID: "b", DependsOn: []string{"a"}},
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Validation Rules

1. **Unit-level**: `unit` field must be present in frontmatter
2. **Task-level**: `task`, `status`, `backpressure` fields required
3. **Task sequence**: Numbers must be sequential starting from 1
4. **Task dependencies**: `depends_on` must reference valid task numbers within unit
5. **Unit dependencies**: `depends_on` must reference existing unit IDs
6. **No cycles**: Unit dependency graph must be acyclic

### Error Aggregation

Collect ALL errors, don't fail fast:

```go
func ValidateUnit(unit *Unit) *ValidationResult {
    result := &ValidationResult{}

    // Check unit field
    if unit.ID == "" {
        result.Add(ValidationError{...})
    }

    // Check each task
    for _, task := range unit.Tasks {
        // ... validate task fields
    }

    // Validate task sequence
    result.Merge(ValidateTaskSequence(unit.Tasks))

    // Validate task dependencies
    result.Merge(ValidateTaskDependencies(unit.Tasks))

    return result
}
```

### Cycle Detection Algorithm

Use DFS with three-color marking:
- 0 = unvisited
- 1 = visiting (in current path)
- 2 = visited (fully explored)

Finding a node with state=1 indicates a cycle. Track the path to report the cycle in the error message.

### Error Formatting

```
validation failed with 3 errors:
  - unit "app-shell", task 2, file "02-nav.md": missing required 'backpressure' field
  - unit "deck-list": task sequence gap, missing task 2
  - circular dependency: app-shell -> deck-list -> app-shell
```

## NOT In Scope

- Modifying files to fix errors (orchestrator responsibility)
- Warning-level issues (only errors that prevent execution)
- Performance optimization (validation is fast enough for hundreds of units)
