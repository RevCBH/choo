---
task: 2
status: pending
backpressure: "go test ./internal/discovery/... -run TestParse"
depends_on: [1]
---

# Frontmatter Parsing

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/lahore/specs/DISCOVERY.md`
**Task**: #2 of 4 in implementation plan

## Objective

Implement YAML frontmatter extraction and parsing for IMPLEMENTATION_PLAN.md and task files.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Unit`, `Task`, `UnitStatus`, `TaskStatus` types)

### Package Dependencies
- `gopkg.in/yaml.v3` - YAML parsing

## Deliverables

### Files to Create/Modify

```
internal/
└── discovery/
    ├── frontmatter.go       # CREATE: Frontmatter parsing logic
    └── frontmatter_test.go  # CREATE: Tests for frontmatter parsing
```

### Types to Implement

```go
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

### Functions to Implement

```go
// ParseFrontmatter extracts YAML frontmatter from markdown content
// Returns the frontmatter string and the remaining content
// Frontmatter is delimited by --- on its own line at start and end
func ParseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error)

// ParseUnitFrontmatter parses IMPLEMENTATION_PLAN.md frontmatter
func ParseUnitFrontmatter(data []byte) (*UnitFrontmatter, error)

// ParseTaskFrontmatter parses task file frontmatter
func ParseTaskFrontmatter(data []byte) (*TaskFrontmatter, error)

// extractTitle extracts the first H1 heading from markdown body
func extractTitle(body []byte) string
```

## Backpressure

### Validation Command

```bash
go test ./internal/discovery/... -run TestParse -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParseFrontmatter_Valid` | Extracts frontmatter between `---` delimiters |
| `TestParseFrontmatter_NoFrontmatter` | Returns empty frontmatter, full body when no `---` |
| `TestParseFrontmatter_Unclosed` | Returns error when closing `---` missing |
| `TestParseUnitFrontmatter_Complete` | Parses all unit frontmatter fields |
| `TestParseUnitFrontmatter_Minimal` | Parses with only `unit` field |
| `TestParseTaskFrontmatter_Complete` | Parses all task frontmatter fields |
| `TestParseTaskFrontmatter_WithDeps` | Parses `depends_on: [1, 2]` correctly |
| `TestExtractTitle_Found` | Extracts `"Title"` from `# Title` |
| `TestExtractTitle_NotFound` | Returns empty string when no H1 |

### Test Fixtures

Test cases embedded in test file:

```go
// Valid frontmatter
`---
task: 1
status: pending
backpressure: "go test ./..."
depends_on: []
---

# Title`

// No frontmatter
`# Just a title`

// Unclosed frontmatter
`---
task: 1
status: pending
# Title`
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Frontmatter must start at the very beginning of the file (first line is `---`)
- Use `bytes.Index` to find delimiters efficiently
- Use `bufio.Scanner` for title extraction (scan lines until `# ` found)
- Handle both `[]` and `null` for empty depends_on arrays
- YAML parsing errors should include context about which field failed

## NOT In Scope

- File I/O (Task #3 handles reading files)
- Validation of field values (Task #4)
- Time parsing for orchestrator fields (handled during Unit construction)
