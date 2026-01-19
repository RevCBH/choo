---
task: 3
status: complete
backpressure: "go test ./internal/discovery/... -run TestDiscover"
depends_on: [1, 2]
---

# File Discovery

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md`
**Task**: #3 of 4 in implementation plan

## Objective

Implement directory walking and glob-based file discovery to find units and tasks.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Unit`, `Task` types)
- Task #2 must be complete (provides: frontmatter parsing functions)

### Package Dependencies
- Standard library (`os`, `path/filepath`, `sort`)

## Deliverables

### Files to Create/Modify

```
internal/
└── discovery/
    ├── discovery.go       # CREATE: Main discovery logic
    └── discovery_test.go  # CREATE: Tests with temp directory fixtures
```

### Functions to Implement

```go
// Discover finds all units and tasks in the given tasks directory
// Returns an error if the directory doesn't exist or validation fails
func Discover(tasksDir string) ([]*Unit, error)

// DiscoverUnit discovers a single unit by directory path
// Useful for targeted re-discovery after file changes
func DiscoverUnit(unitDir string) (*Unit, error)

// discoverTaskFiles finds all task files matching [0-9][0-9]-*.md pattern
func discoverTaskFiles(unitDir string) ([]string, error)

// discoverUnitDirs finds all subdirectories of tasksDir
func discoverUnitDirs(tasksDir string) ([]string, error)
```

## Backpressure

### Validation Command

```bash
go test ./internal/discovery/... -run TestDiscover -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDiscover_SingleUnit` | Discovers one unit with tasks |
| `TestDiscover_MultipleUnits` | Discovers multiple units |
| `TestDiscover_SkipsNoImplPlan` | Skips directories without IMPLEMENTATION_PLAN.md |
| `TestDiscover_SkipsNoTasks` | Skips directories with IMPLEMENTATION_PLAN.md but no task files |
| `TestDiscover_TaskOrdering` | Tasks are ordered by filename (01, 02, 03...) |
| `TestDiscover_TaskContent` | Task.Content contains full file content |
| `TestDiscover_TaskTitle` | Task.Title extracted from H1 |
| `TestDiscover_UnitDependsOn` | Unit.DependsOn populated from frontmatter |
| `TestDiscover_OrchFields` | Orchestrator fields populated when present |
| `TestDiscoverTaskFiles_Pattern` | Only matches `[0-9][0-9]-*.md` |
| `TestDiscoverTaskFiles_Sorting` | Returns files sorted by name |
| `TestDiscoverUnit_NotExists` | Returns error for non-existent directory |

### Test Fixtures

Create temporary directories in tests using `t.TempDir()`:

```go
// Test structure:
// tmpDir/
//   specs/tasks/
//     app-shell/
//       IMPLEMENTATION_PLAN.md
//       01-nav-types.md
//       02-navigation.md
//     deck-list/
//       IMPLEMENTATION_PLAN.md
//       01-deck-card.md
//     no-impl-plan/
//       01-task.md           # Should be skipped
//     no-tasks/
//       IMPLEMENTATION_PLAN.md  # Should be skipped
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `filepath.Glob` with pattern `[0-9][0-9]-*.md` for task discovery
- Sort task files with `sort.Strings` for deterministic order
- Check for `IMPLEMENTATION_PLAN.md` existence before processing directory
- Read full file content for `Task.Content` field
- Parse time strings from orchestrator fields using RFC3339 format
- Return descriptive errors including file paths

### Discovery Flow

```
1. Verify tasksDir exists and is a directory
2. List all subdirectories of tasksDir
3. For each subdirectory:
   a. Check for IMPLEMENTATION_PLAN.md
      - If missing: skip directory (not a unit)
   b. Glob for [0-9][0-9]-*.md files
      - If none found: skip directory (no tasks)
   c. Parse IMPLEMENTATION_PLAN.md:
      - Read file content
      - Extract frontmatter
      - Parse YAML into UnitFrontmatter
      - Populate Unit fields
   d. For each task file (sorted by filename):
      - Read file content
      - Extract frontmatter
      - Parse YAML into TaskFrontmatter
      - Extract title from first H1
      - Populate Task fields
      - Append to Unit.Tasks
   e. Append Unit to results
4. Return []*Unit (validation happens in Task #4)
```

## NOT In Scope

- Validation of task sequences (Task #4)
- Validation of dependencies (Task #4)
- Cycle detection (Task #4)
- Error aggregation (Task #4)
