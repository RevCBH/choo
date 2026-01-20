---
task: 2
status: pending
backpressure: "go test ./internal/feature/... -run PRD"
depends_on: [1]
---

# PRD Store

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement the PRD store for loading, parsing, and atomically updating PRD frontmatter.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`, `FeatureState`)

### Package Dependencies
- `os` (standard library)
- `path/filepath` (standard library)
- `gopkg.in/yaml.v3`

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── prd.go    # CREATE: PRD store implementation
```

### Types to Implement

```go
// PRDStore handles PRD file operations
type PRDStore struct {
    baseDir string
}

// PRDMetadata represents parsed PRD frontmatter
type PRDMetadata struct {
    Title            string                 `yaml:"title"`
    FeatureStatus    FeatureStatus          `yaml:"feature_status,omitempty"`
    Branch           string                 `yaml:"branch,omitempty"`
    StartedAt        *time.Time             `yaml:"started_at,omitempty"`
    ReviewIterations int                    `yaml:"review_iterations,omitempty"`
    MaxReviewIter    int                    `yaml:"max_review_iter,omitempty"`
    LastFeedback     string                 `yaml:"last_feedback,omitempty"`
    SpecCount        int                    `yaml:"spec_count,omitempty"`
    TaskCount        int                    `yaml:"task_count,omitempty"`
    Extra            map[string]interface{} `yaml:",inline"`
}
```

### Functions to Implement

```go
// NewPRDStore creates a PRD store for the given directory
func NewPRDStore(baseDir string) *PRDStore {
    // Store base directory for PRD files
}

// Load reads and parses a PRD file, returning metadata and body separately
func (s *PRDStore) Load(prdID string) (*PRDMetadata, string, error) {
    // Construct path: baseDir/<prd-id>.md
    // Read entire file
    // Split frontmatter (between --- markers) from body
    // Parse frontmatter as YAML into PRDMetadata
    // Return metadata, body content, and any error
}

// UpdateStatus atomically updates only the feature_status field
func (s *PRDStore) UpdateStatus(prdID string, status FeatureStatus) error {
    // Load current file
    // Update only the status field
    // Write back atomically
}

// UpdateState atomically updates full feature state in frontmatter
func (s *PRDStore) UpdateState(prdID string, state FeatureState) error {
    // Load current file (preserving extra fields)
    // Merge state fields into metadata
    // Serialize frontmatter with YAML
    // Write complete file (frontmatter + body)
}

// Exists checks if a PRD file exists
func (s *PRDStore) Exists(prdID string) bool {
    // Check if baseDir/<prd-id>.md exists
}

// prdPath returns the full path for a PRD ID
func (s *PRDStore) prdPath(prdID string) string {
    return filepath.Join(s.baseDir, prdID+".md")
}

// parseFrontmatter extracts YAML frontmatter from file content
func parseFrontmatter(content string) (frontmatter string, body string, err error) {
    // Find opening and closing --- markers
    // Return frontmatter between markers and body after
}

// serializeFrontmatter converts metadata back to YAML with --- markers
func serializeFrontmatter(meta *PRDMetadata) (string, error) {
    // Marshal metadata to YAML
    // Wrap with --- markers
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run PRD
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestPRDStore_Load` | Returns metadata and body from valid PRD file |
| `TestPRDStore_Load_NotFound` | Returns error for missing file |
| `TestPRDStore_Load_NoFrontmatter` | Returns error for PRD without frontmatter |
| `TestPRDStore_UpdateStatus` | Status field updated, other fields preserved |
| `TestPRDStore_UpdateState` | All state fields updated atomically |
| `TestPRDStore_Exists` | Returns true for existing, false for missing |
| `TestParseFrontmatter` | Correctly splits frontmatter from body |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `valid.md` | `internal/feature/testdata/` | Valid PRD with frontmatter |
| `no-frontmatter.md` | `internal/feature/testdata/` | PRD missing frontmatter markers |
| `partial-state.md` | `internal/feature/testdata/` | PRD with some state fields set |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Frontmatter is delimited by `---` on its own line at file start and end
- The `Extra` field with `yaml:",inline"` preserves unknown frontmatter fields
- Atomic writes should use write-to-temp + rename pattern for crash safety
- Body content should be preserved exactly (no trailing newline changes)

## NOT In Scope

- Git operations (feature-workflow spec handles those)
- CLI command implementations (tasks #3-6)
- Workflow state machine execution (feature-workflow spec)
