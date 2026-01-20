---
task: 2
status: pending
backpressure: "go test ./internal/feature/... -run TestLoadPRD"
depends_on: [1]
---

# PRD Loader

**Parent spec**: `/specs/FEATURE-PRIORITIZER.md`
**Task**: #2 of 5 in implementation plan

## Objective

Implement PRD file loading with optional frontmatter parsing for dependency hints.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides `PRD` type and `ParsePRD` function

### Task Dependencies (within this unit)
- Task #1 (provides: `PriorityResult`, `Recommendation` types)

### Package Dependencies
- `gopkg.in/yaml.v3` - YAML frontmatter parsing
- `os` - File I/O
- `path/filepath` - Path manipulation

## Deliverables

### Files to Create/Modify

```
internal/feature/
├── prd.go       # CREATE
└── prd_test.go  # CREATE
```

### Types to Implement

```go
// PRDFrontmatter represents optional YAML frontmatter in PRDs
type PRDFrontmatter struct {
    Title     string   `yaml:"title"`
    DependsOn []string `yaml:"depends_on"`
    Status    string   `yaml:"status"`   // draft, ready, in_progress, complete
    Priority  string   `yaml:"priority"` // hint: high, medium, low
}

// PRDForPrioritization represents a PRD loaded for prioritization
type PRDForPrioritization struct {
    ID        string   // filename without extension
    Path      string   // absolute path to file
    Title     string   // extracted from first H1 or frontmatter
    Content   string   // full markdown content
    DependsOn []string // from frontmatter (optional hints)
}
```

### Functions to Implement

```go
// LoadPRDs reads all PRD files from the given directory
func LoadPRDs(prdDir string) ([]*PRDForPrioritization, error)

// ParsePRDFrontmatter extracts optional frontmatter from PRD content
func ParsePRDFrontmatter(content []byte) (*PRDFrontmatter, error)

// ExtractPRDTitle extracts the first H1 heading as title
func ExtractPRDTitle(content []byte) string
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestLoadPRD -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestLoadPRDs_Valid` | Loads 2 PRDs from test directory |
| `TestLoadPRDs_EmptyDir` | Returns error with helpful message |
| `TestLoadPRDs_NoMarkdown` | Returns error when no .md files found |
| `TestParsePRDFrontmatter_Complete` | Parses all frontmatter fields |
| `TestParsePRDFrontmatter_None` | Returns nil (not error) when no frontmatter |
| `TestExtractPRDTitle_Found` | Extracts title from `# Title` |
| `TestExtractPRDTitle_AfterFrontmatter` | Extracts title after `---` block |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `filepath.Glob` with `*.md` pattern for discovery
- Skip README.md files (common in PRD directories)
- PRD ID derived from filename: `auth.md` -> `auth`
- Frontmatter is optional - PRDs without it are valid
- Title extraction: check frontmatter first, then look for H1 heading
- Handle malformed frontmatter gracefully (log warning, skip PRD)

### Edge Cases

- PRD with no frontmatter: valid, uses H1 as title
- PRD with empty frontmatter (`---\n---`): valid
- PRD with malformed YAML: skip with warning
- PRD with no H1 heading: use filename as fallback title

## NOT In Scope

- Full PRD validation (handled by feature-discovery)
- Caching or repository pattern (handled by feature-discovery)
- Prioritization logic (Task #3)
