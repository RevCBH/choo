# DISCOVERY — Unit and Task Discovery for Ralph Orchestrator

## Overview

The Discovery package is responsible for finding, parsing, and validating units and tasks from the `specs/tasks/` directory structure. It serves as the first phase of the orchestration pipeline, transforming the file-based specification format into strongly-typed Go data structures that the Scheduler and Worker components consume.

Discovery reads IMPLEMENTATION_PLAN.md files for unit metadata (dependencies, orchestrator state) and numbered task files (01-*.md, 02-*.md, etc.) for individual task specifications. It validates the structure, parses YAML frontmatter, and builds a complete dependency graph representation.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Discovery                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   specs/tasks/                                                          │
│   ├── app-shell/                                                        │
│   │   ├── IMPLEMENTATION_PLAN.md  ──────┐                               │
│   │   ├── 01-nav-types.md         ──────┼──────▶  []*Unit               │
│   │   ├── 02-navigation.md        ──────┤         ├── Unit.ID           │
│   │   └── 03-app-shell.md         ──────┘         ├── Unit.DependsOn    │
│   └── deck-list/                                  ├── Unit.Status       │
│       ├── IMPLEMENTATION_PLAN.md  ──────┐         └── Unit.Tasks        │
│       ├── 01-deck-card.md         ──────┼──────▶      ├── Task.Number   │
│       └── 02-deck-grid.md         ──────┘             ├── Task.Status   │
│                                                       └── Task.Backpressure
│                                                                         │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐             │
│   │  Glob Files  │───▶│ Parse YAML   │───▶│   Validate   │             │
│   │  Discovery   │    │ Frontmatter  │    │   Structure  │             │
│   └──────────────┘    └──────────────┘    └──────────────┘             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Discover all unit directories under `specs/tasks/`
2. Parse IMPLEMENTATION_PLAN.md frontmatter for unit metadata
3. Parse task files matching pattern `[0-9][0-9]-*.md`
4. Extract YAML frontmatter from each file (delimited by `---`)
5. Extract task title from first H1 heading
6. Validate unit has required `unit` field in frontmatter
7. Validate all tasks have required fields: `task`, `status`, `backpressure`
8. Validate task numbers are sequential starting from 1
9. Validate task `depends_on` references are valid task numbers within the unit
10. Validate unit `depends_on` references exist as discovered units
11. Detect and report circular dependencies
12. Skip directories without IMPLEMENTATION_PLAN.md
13. Skip directories without any matching task files
14. Return complete `[]*Unit` slice with populated `Tasks` field

### Performance Requirements

| Metric | Target |
|--------|--------|
| Discovery time for 10 units | <100ms |
| Discovery time for 100 tasks | <200ms |
| Memory per unit | <1KB (excluding file content) |
| Memory per task (with content) | Proportional to file size |

### Constraints

- No external dependencies beyond Go standard library and YAML parser
- Must handle concurrent access (discovery may run while files are being modified)
- Must preserve original file content for Task.Content field
- Must report all validation errors, not just the first one

## Design

### Module Structure

```
internal/discovery/
├── discovery.go       # Main Discover() function and file walking
├── frontmatter.go     # YAML frontmatter parsing
├── types.go           # Unit, Task, and status enum types
└── validation.go      # Validation rules and error collection
```

### Core Types

```go
// internal/discovery/types.go

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

```go
// internal/discovery/frontmatter.go

// UnitFrontmatter represents the YAML frontmatter in IMPLEMENTATION_PLAN.md
type UnitFrontmatter struct {
    // Required fields
    Unit string `yaml:"unit"`

    // Optional dependency field
    DependsOn []string `yaml:"depends_on"`

    // Orchestrator-managed fields (may not be present initially)
    OrchStatus      string `yaml:"orch_status"`
    OrchBranch      string `yaml:"orch_branch"`
    OrchWorktree    string `yaml:"orch_worktree"`
    OrchPRNumber    int    `yaml:"orch_pr_number"`
    OrchStartedAt   string `yaml:"orch_started_at"`
    OrchCompletedAt string `yaml:"orch_completed_at"`
}

// TaskFrontmatter represents the YAML frontmatter in task files
type TaskFrontmatter struct {
    // Required fields
    Task         int    `yaml:"task"`
    Status       string `yaml:"status"`
    Backpressure string `yaml:"backpressure"`

    // Optional dependency field
    DependsOn []int `yaml:"depends_on"`
}
```

```go
// internal/discovery/validation.go

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
```

### API Surface

```go
// internal/discovery/discovery.go

// Discover finds all units and tasks in the given tasks directory
// Returns an error if the directory doesn't exist or validation fails
func Discover(tasksDir string) ([]*Unit, error)

// DiscoverUnit discovers a single unit by directory path
// Useful for targeted re-discovery after file changes
func DiscoverUnit(unitDir string) (*Unit, error)
```

```go
// internal/discovery/frontmatter.go

// ParseFrontmatter extracts YAML frontmatter from markdown content
// Returns the frontmatter string and the remaining content
func ParseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error)

// ParseUnitFrontmatter parses IMPLEMENTATION_PLAN.md frontmatter
func ParseUnitFrontmatter(data []byte) (*UnitFrontmatter, error)

// ParseTaskFrontmatter parses task file frontmatter
func ParseTaskFrontmatter(data []byte) (*TaskFrontmatter, error)
```

```go
// internal/discovery/validation.go

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

### Discovery Flow

```
Input: specs/tasks/ directory path

1. Verify tasksDir exists and is a directory
2. List all subdirectories of tasksDir
3. For each subdirectory:
   a. Check for IMPLEMENTATION_PLAN.md
      - If missing: skip directory (not a unit)
   b. Glob for [0-9][0-9]-*.md files
      - If none found: skip directory (no tasks)
   c. Parse IMPLEMENTATION_PLAN.md:
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

4. Validate all units:
   a. Each unit has `unit` field
   b. Each task has `task`, `status`, `backpressure` fields
   c. Task numbers are sequential (1, 2, 3, ...)
   d. Task depends_on references valid task numbers
   e. Unit depends_on references existing unit IDs
   f. No circular dependencies in unit graph

5. If validation fails: return nil, ValidationError
6. Return []*Unit, nil
```

### File Pattern Matching

Task files must match the pattern `[0-9][0-9]-*.md`:
- Two digits followed by a hyphen
- Any characters for the task name
- `.md` extension

Examples of valid task files:
- `01-nav-types.md`
- `02-navigation.md`
- `10-final-integration.md`

Examples of invalid task files (skipped):
- `1-nav-types.md` (single digit)
- `01_nav_types.md` (underscore instead of hyphen)
- `README.md` (doesn't match pattern)
- `01-nav-types.txt` (wrong extension)

### Frontmatter Parsing

Frontmatter is delimited by `---` on its own line:

```markdown
---
task: 1
status: pending
backpressure: "cd koe && pnpm typecheck"
depends_on: []
---

# Task Title

Content here...
```

The parser:
1. Looks for opening `---` at the start of the file
2. Finds the closing `---`
3. Extracts the content between as YAML
4. Returns the remaining content as body

### Title Extraction

The task title is extracted from the first H1 heading in the body:

```go
func extractTitle(body []byte) string {
    scanner := bufio.NewScanner(bytes.NewReader(body))
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if strings.HasPrefix(line, "# ") {
            return strings.TrimPrefix(line, "# ")
        }
    }
    return "" // No title found
}
```

## Implementation Notes

### Glob-Based File Discovery

Use `filepath.Glob` for task file discovery:

```go
func discoverTaskFiles(unitDir string) ([]string, error) {
    pattern := filepath.Join(unitDir, "[0-9][0-9]-*.md")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return nil, err
    }
    sort.Strings(matches) // Ensure deterministic order
    return matches, nil
}
```

### Status Parsing

Parse status strings into typed enums with validation:

```go
func parseUnitStatus(s string) (UnitStatus, error) {
    switch s {
    case "", "pending":
        return UnitStatusPending, nil
    case "in_progress":
        return UnitStatusInProgress, nil
    case "pr_open":
        return UnitStatusPROpen, nil
    case "in_review":
        return UnitStatusInReview, nil
    case "merging":
        return UnitStatusMerging, nil
    case "complete":
        return UnitStatusComplete, nil
    case "failed":
        return UnitStatusFailed, nil
    default:
        return "", fmt.Errorf("invalid unit status: %q", s)
    }
}

func parseTaskStatus(s string) (TaskStatus, error) {
    switch s {
    case "", "pending":
        return TaskStatusPending, nil
    case "in_progress":
        return TaskStatusInProgress, nil
    case "complete":
        return TaskStatusComplete, nil
    case "failed":
        return TaskStatusFailed, nil
    default:
        return "", fmt.Errorf("invalid task status: %q", s)
    }
}
```

### Cycle Detection

Use depth-first search to detect cycles in the unit dependency graph:

```go
func DetectCycles(units []*Unit) *ValidationResult {
    result := &ValidationResult{}

    // Build adjacency map
    deps := make(map[string][]string)
    for _, u := range units {
        deps[u.ID] = u.DependsOn
    }

    // Track visit state: 0=unvisited, 1=visiting, 2=visited
    state := make(map[string]int)
    var path []string

    var dfs func(id string) bool
    dfs = func(id string) bool {
        if state[id] == 1 {
            // Found cycle - build error message
            cycleStart := 0
            for i, p := range path {
                if p == id {
                    cycleStart = i
                    break
                }
            }
            cycle := append(path[cycleStart:], id)
            result.Errors = append(result.Errors, ValidationError{
                Message: fmt.Sprintf("circular dependency: %s", strings.Join(cycle, " -> ")),
            })
            return true
        }
        if state[id] == 2 {
            return false
        }

        state[id] = 1
        path = append(path, id)

        for _, dep := range deps[id] {
            if dfs(dep) {
                return true
            }
        }

        path = path[:len(path)-1]
        state[id] = 2
        return false
    }

    for _, u := range units {
        if state[u.ID] == 0 {
            dfs(u.ID)
        }
    }

    return result
}
```

### Error Aggregation

Collect all validation errors rather than failing on the first:

```go
func ValidateUnit(unit *Unit) *ValidationResult {
    result := &ValidationResult{}

    // Validate unit-level fields
    if unit.ID == "" {
        result.Errors = append(result.Errors, ValidationError{
            Unit:    unit.ID,
            File:    "IMPLEMENTATION_PLAN.md",
            Field:   "unit",
            Message: "missing required 'unit' field in frontmatter",
        })
    }

    // Validate each task
    for _, task := range unit.Tasks {
        if task.Number <= 0 {
            result.Errors = append(result.Errors, ValidationError{
                Unit:    unit.ID,
                Task:    &task.Number,
                File:    task.FilePath,
                Field:   "task",
                Message: "missing or invalid 'task' field",
            })
        }
        if task.Backpressure == "" {
            result.Errors = append(result.Errors, ValidationError{
                Unit:    unit.ID,
                Task:    &task.Number,
                File:    task.FilePath,
                Field:   "backpressure",
                Message: "missing required 'backpressure' field",
            })
        }
        // ... additional validations
    }

    return result
}
```

## Testing Strategy

### Unit Tests

```go
// internal/discovery/frontmatter_test.go

func TestParseFrontmatter(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        wantFM      string
        wantBody    string
        wantErr     bool
    }{
        {
            name: "valid frontmatter",
            input: `---
task: 1
status: pending
---

# Title`,
            wantFM:   "task: 1\nstatus: pending\n",
            wantBody: "\n# Title",
            wantErr:  false,
        },
        {
            name:    "no frontmatter",
            input:   "# Just a title",
            wantFM:  "",
            wantBody: "# Just a title",
            wantErr: false,
        },
        {
            name: "unclosed frontmatter",
            input: `---
task: 1
status: pending
# Title`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            fm, body, err := ParseFrontmatter([]byte(tt.input))
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr {
                if string(fm) != tt.wantFM {
                    t.Errorf("frontmatter = %q, want %q", fm, tt.wantFM)
                }
                if string(body) != tt.wantBody {
                    t.Errorf("body = %q, want %q", body, tt.wantBody)
                }
            }
        })
    }
}

func TestParseTaskFrontmatter(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *TaskFrontmatter
        wantErr bool
    }{
        {
            name: "complete frontmatter",
            input: `task: 1
status: pending
backpressure: "pnpm typecheck"
depends_on: []`,
            want: &TaskFrontmatter{
                Task:         1,
                Status:       "pending",
                Backpressure: "pnpm typecheck",
                DependsOn:    []int{},
            },
        },
        {
            name: "with dependencies",
            input: `task: 3
status: pending
backpressure: "pnpm test"
depends_on: [1, 2]`,
            want: &TaskFrontmatter{
                Task:         3,
                Status:       "pending",
                Backpressure: "pnpm test",
                DependsOn:    []int{1, 2},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseTaskFrontmatter([]byte(tt.input))
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseTaskFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseTaskFrontmatter() = %+v, want %+v", got, tt.want)
            }
        })
    }
}
```

```go
// internal/discovery/validation_test.go

func TestValidateTaskSequence(t *testing.T) {
    tests := []struct {
        name     string
        tasks    []*Task
        wantErrs int
    }{
        {
            name: "sequential from 1",
            tasks: []*Task{
                {Number: 1, FilePath: "01-a.md"},
                {Number: 2, FilePath: "02-b.md"},
                {Number: 3, FilePath: "03-c.md"},
            },
            wantErrs: 0,
        },
        {
            name: "starts from 2",
            tasks: []*Task{
                {Number: 2, FilePath: "02-a.md"},
                {Number: 3, FilePath: "03-b.md"},
            },
            wantErrs: 1, // missing task 1
        },
        {
            name: "gap in sequence",
            tasks: []*Task{
                {Number: 1, FilePath: "01-a.md"},
                {Number: 3, FilePath: "03-c.md"},
            },
            wantErrs: 1, // missing task 2
        },
        {
            name: "duplicate numbers",
            tasks: []*Task{
                {Number: 1, FilePath: "01-a.md"},
                {Number: 1, FilePath: "01-b.md"},
            },
            wantErrs: 1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ValidateTaskSequence(tt.tasks)
            if len(result.Errors) != tt.wantErrs {
                t.Errorf("ValidateTaskSequence() got %d errors, want %d: %v",
                    len(result.Errors), tt.wantErrs, result.Errors)
            }
        })
    }
}

func TestDetectCycles(t *testing.T) {
    tests := []struct {
        name     string
        units    []*Unit
        wantCycle bool
    }{
        {
            name: "no dependencies",
            units: []*Unit{
                {ID: "a", DependsOn: []string{}},
                {ID: "b", DependsOn: []string{}},
            },
            wantCycle: false,
        },
        {
            name: "linear chain",
            units: []*Unit{
                {ID: "a", DependsOn: []string{}},
                {ID: "b", DependsOn: []string{"a"}},
                {ID: "c", DependsOn: []string{"b"}},
            },
            wantCycle: false,
        },
        {
            name: "simple cycle",
            units: []*Unit{
                {ID: "a", DependsOn: []string{"b"}},
                {ID: "b", DependsOn: []string{"a"}},
            },
            wantCycle: true,
        },
        {
            name: "three-node cycle",
            units: []*Unit{
                {ID: "a", DependsOn: []string{"c"}},
                {ID: "b", DependsOn: []string{"a"}},
                {ID: "c", DependsOn: []string{"b"}},
            },
            wantCycle: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := DetectCycles(tt.units)
            hasCycle := len(result.Errors) > 0
            if hasCycle != tt.wantCycle {
                t.Errorf("DetectCycles() hasCycle = %v, want %v", hasCycle, tt.wantCycle)
            }
        })
    }
}
```

```go
// internal/discovery/discovery_test.go

func TestDiscover(t *testing.T) {
    // Create temp directory with test fixtures
    tmpDir := t.TempDir()
    tasksDir := filepath.Join(tmpDir, "specs", "tasks")

    // Create app-shell unit
    appShellDir := filepath.Join(tasksDir, "app-shell")
    os.MkdirAll(appShellDir, 0755)

    writeFile(t, filepath.Join(appShellDir, "IMPLEMENTATION_PLAN.md"), `---
unit: app-shell
depends_on: []
---

# App Shell Implementation`)

    writeFile(t, filepath.Join(appShellDir, "01-nav-types.md"), `---
task: 1
status: pending
backpressure: "pnpm typecheck"
depends_on: []
---

# Navigation Types`)

    writeFile(t, filepath.Join(appShellDir, "02-navigation.md"), `---
task: 2
status: pending
backpressure: "pnpm typecheck"
depends_on: [1]
---

# Navigation Component`)

    // Run discovery
    units, err := Discover(tasksDir)
    if err != nil {
        t.Fatalf("Discover() error = %v", err)
    }

    if len(units) != 1 {
        t.Fatalf("Discover() got %d units, want 1", len(units))
    }

    unit := units[0]
    if unit.ID != "app-shell" {
        t.Errorf("unit.ID = %q, want %q", unit.ID, "app-shell")
    }

    if len(unit.Tasks) != 2 {
        t.Fatalf("unit.Tasks has %d tasks, want 2", len(unit.Tasks))
    }

    if unit.Tasks[0].Title != "Navigation Types" {
        t.Errorf("Tasks[0].Title = %q, want %q", unit.Tasks[0].Title, "Navigation Types")
    }

    if unit.Tasks[1].DependsOn[0] != 1 {
        t.Errorf("Tasks[1].DependsOn = %v, want [1]", unit.Tasks[1].DependsOn)
    }
}

func TestDiscover_SkipsInvalidDirectories(t *testing.T) {
    tmpDir := t.TempDir()
    tasksDir := filepath.Join(tmpDir, "specs", "tasks")

    // Directory without IMPLEMENTATION_PLAN.md
    noImplPlan := filepath.Join(tasksDir, "no-impl-plan")
    os.MkdirAll(noImplPlan, 0755)
    writeFile(t, filepath.Join(noImplPlan, "01-task.md"), `---
task: 1
status: pending
backpressure: "test"
---
# Task`)

    // Directory without task files
    noTasks := filepath.Join(tasksDir, "no-tasks")
    os.MkdirAll(noTasks, 0755)
    writeFile(t, filepath.Join(noTasks, "IMPLEMENTATION_PLAN.md"), `---
unit: no-tasks
---
# Implementation`)

    // Valid unit
    validDir := filepath.Join(tasksDir, "valid")
    os.MkdirAll(validDir, 0755)
    writeFile(t, filepath.Join(validDir, "IMPLEMENTATION_PLAN.md"), `---
unit: valid
---
# Valid`)
    writeFile(t, filepath.Join(validDir, "01-task.md"), `---
task: 1
status: pending
backpressure: "test"
---
# Task`)

    units, err := Discover(tasksDir)
    if err != nil {
        t.Fatalf("Discover() error = %v", err)
    }

    if len(units) != 1 {
        t.Fatalf("Discover() got %d units, want 1", len(units))
    }

    if units[0].ID != "valid" {
        t.Errorf("unit.ID = %q, want %q", units[0].ID, "valid")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Valid multi-unit discovery | Fixture with 3 units, verify all discovered correctly |
| Unit with dependencies | Fixture with A depends on B, verify DependsOn populated |
| Validation error reporting | Fixture with multiple errors, verify all reported |
| Empty tasks directory | Empty specs/tasks/, verify empty slice returned |
| Partial orchestrator state | Fixture with orch_status set, verify state preserved |

### Manual Testing

- [ ] `Discover()` finds all units in real project structure
- [ ] Invalid frontmatter produces clear error messages
- [ ] Missing required fields produce specific field errors
- [ ] Duplicate task numbers are detected
- [ ] Circular dependencies are detected and reported
- [ ] Non-unit directories are silently skipped

## Design Decisions

### Why Glob Pattern [0-9][0-9]-*.md?

The two-digit prefix enables:
1. Natural sort order matches task sequence
2. Support for up to 99 tasks per unit (sufficient)
3. Clear visual distinction from non-task files
4. Simple, fast pattern matching with `filepath.Glob`

Alternative considered: regex-based matching (more flexible but slower and more complex).

### Why Store Full Content in Task.Content?

The Worker needs access to the full task specification to build Claude prompts. Storing content at discovery time:
1. Avoids repeated file reads during execution
2. Ensures consistent state (file may change during run)
3. Simplifies Worker implementation

Trade-off: Higher memory usage for large specs (acceptable given expected file sizes <50KB).

### Why Validate All Errors at Once?

Failing on the first error provides poor UX for spec authors. They would need to fix one error, re-run, discover another, repeat. Collecting all errors enables fixing everything in one pass.

### Why Typed Status Enums?

String comparisons are error-prone. Typed enums provide:
1. Compile-time checking
2. IDE autocomplete
3. Exhaustive switch statements
4. Self-documenting code

## Future Enhancements

1. Watch mode for file changes during execution
2. Caching of parsed units to avoid re-parsing unchanged files
3. Parallel file parsing for large unit counts
4. JSON Schema validation for frontmatter
5. Support for nested unit directories

## References

- [PRD Section 4.1: Discovery Flow](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 3.4: Go Types](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [Go filepath.Glob documentation](https://pkg.go.dev/path/filepath#Glob)
- [YAML v3 for Go](https://pkg.go.dev/gopkg.in/yaml.v3)
