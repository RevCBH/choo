---
task: 1
status: pending
backpressure: "go build ./internal/discovery/..."
depends_on: []
---

# Core Types

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md`
**Task**: #1 of 4 in implementation plan

## Objective

Define the core data structures for the Discovery package: Unit, Task, and their status enums.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- Standard library only (`time` package)

## Deliverables

### Files to Create/Modify

```
internal/
└── discovery/
    └── types.go    # CREATE: Core types and status enums
```

### Types to Implement

```go
// Unit represents a discovered unit of work with its tasks
type Unit struct {
    // Parsed from directory structure
    ID       string   // directory name, e.g., "app-shell"
    Path     string   // absolute path to unit directory

    // Parsed from IMPLEMENTATION_PLAN.md frontmatter
    DependsOn []string // other unit IDs this unit depends on

    // Orchestrator state (from frontmatter, updated at runtime)
    Status      UnitStatus
    Branch      string     // orch_branch from frontmatter
    Worktree    string     // orch_worktree from frontmatter
    PRNumber    int        // orch_pr_number from frontmatter
    StartedAt   *time.Time // orch_started_at from frontmatter
    CompletedAt *time.Time // orch_completed_at from frontmatter

    // Parsed from task files
    Tasks []*Task
}

// UnitStatus represents the lifecycle state of a unit
type UnitStatus string

const (
    UnitStatusPending    UnitStatus = "pending"
    UnitStatusInProgress UnitStatus = "in_progress"
    UnitStatusPROpen     UnitStatus = "pr_open"
    UnitStatusInReview   UnitStatus = "in_review"
    UnitStatusMerging    UnitStatus = "merging"
    UnitStatusComplete   UnitStatus = "complete"
    UnitStatusFailed     UnitStatus = "failed"
)

// Task represents a single task within a unit
type Task struct {
    // Parsed from frontmatter
    Number       int        // task field from frontmatter
    Status       TaskStatus // status field from frontmatter
    Backpressure string     // backpressure field from frontmatter
    DependsOn    []int      // depends_on field (task numbers within unit)

    // Parsed from file
    FilePath string // relative to unit dir, e.g., "01-nav-types.md"
    Title    string // extracted from first H1 heading
    Content  string // full markdown content (including frontmatter)
}

// TaskStatus represents the lifecycle state of a task
type TaskStatus string

const (
    TaskStatusPending    TaskStatus = "pending"
    TaskStatusInProgress TaskStatus = "in_progress"
    TaskStatusComplete   TaskStatus = "complete"
    TaskStatusFailed     TaskStatus = "failed"
)
```

### Functions to Implement

```go
// parseUnitStatus converts a string to UnitStatus with validation
func parseUnitStatus(s string) (UnitStatus, error)

// parseTaskStatus converts a string to TaskStatus with validation
func parseTaskStatus(s string) (TaskStatus, error)
```

## Backpressure

### Validation Command

```bash
go build ./internal/discovery/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `parseUnitStatus("")` | Returns `UnitStatusPending, nil` |
| `parseUnitStatus("pending")` | Returns `UnitStatusPending, nil` |
| `parseUnitStatus("in_progress")` | Returns `UnitStatusInProgress, nil` |
| `parseUnitStatus("invalid")` | Returns error |
| `parseTaskStatus("")` | Returns `TaskStatusPending, nil` |
| `parseTaskStatus("complete")` | Returns `TaskStatusComplete, nil` |

### Test Fixtures

None required - pure type definitions and simple parsing functions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Empty string for status should default to `pending` (units/tasks start as pending)
- Status parsing functions should be case-sensitive (match spec exactly)
- Use `fmt.Errorf` for invalid status errors with the invalid value in message

## NOT In Scope

- Frontmatter parsing (Task #2)
- File discovery logic (Task #3)
- Validation logic (Task #4)
