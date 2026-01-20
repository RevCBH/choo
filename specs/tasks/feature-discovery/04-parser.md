---
task: 4
status: pending
backpressure: "go test ./internal/feature/... -run TestParsePRD"
depends_on: [1, 3]
---

# PRD Parser

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #4 of 7 in implementation plan

## Objective

Implement PRD parsing from files and readers, including YAML frontmatter unmarshaling and body hash computation.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRD` struct)
- Task #3 must be complete (provides: `parseFrontmatter` function)

### Package Dependencies
- `gopkg.in/yaml.v3` - YAML parsing
- `crypto/sha256` - Hash computation
- `encoding/hex` - Hash encoding

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    ├── discovery.go       # CREATE: PRD parsing functions
    └── discovery_test.go  # CREATE: Tests for PRD parsing
```

### Functions to Implement

```go
// ParsePRD reads a PRD file and parses its frontmatter and body
func ParsePRD(filePath string) (*PRD, error)

// ParsePRDFromReader parses a PRD from an io.Reader (for testing)
func ParsePRDFromReader(r io.Reader, filePath string) (*PRD, error)

// ComputeBodyHash returns SHA-256 hash of the PRD body content
func ComputeBodyHash(body string) string
```

### Implementation Logic

```go
func ParsePRDFromReader(r io.Reader, filePath string) (*PRD, error) {
    content, err := io.ReadAll(r)
    if err != nil {
        return nil, fmt.Errorf("read PRD: %w", err)
    }

    frontmatter, body, err := parseFrontmatter(content)
    if err != nil {
        return nil, fmt.Errorf("parse frontmatter: %w", err)
    }

    var prd PRD
    if err := yaml.Unmarshal(frontmatter, &prd); err != nil {
        return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
    }

    prd.FilePath = filePath
    prd.Body = string(body)
    prd.BodyHash = ComputeBodyHash(prd.Body)

    return &prd, nil
}

func ParsePRD(filePath string) (*PRD, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("open PRD file: %w", err)
    }
    defer f.Close()

    return ParsePRDFromReader(f, filePath)
}

func ComputeBodyHash(body string) string {
    h := sha256.New()
    h.Write([]byte(body))
    return hex.EncodeToString(h.Sum(nil))
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestParsePRD -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParsePRD_ValidComplete` | Parses all frontmatter fields correctly |
| `TestParsePRD_MinimalFields` | Parses PRD with only required fields |
| `TestParsePRD_WithDependsOn` | Parses `depends_on` array correctly |
| `TestParsePRD_WithEstimates` | Parses complexity estimate fields |
| `TestParsePRD_WithOrchestratorFields` | Parses orchestrator-managed fields |
| `TestParsePRD_BodyContent` | Body field contains content after frontmatter |
| `TestParsePRD_BodyHashComputed` | BodyHash is non-empty 64-char hex string |
| `TestParsePRDFromReader_Error` | Returns error for invalid YAML |
| `TestComputeBodyHash_Deterministic` | Same content produces same hash |
| `TestComputeBodyHash_Different` | Different content produces different hash |

### Test Fixtures

```go
var validPRDContent = `---
prd_id: test-feature
title: "Test Feature"
status: draft
depends_on:
  - other-feature
estimated_units: 3
estimated_tasks: 12
---

# Test Feature

This is the PRD body content.
`

var minimalPRDContent = `---
prd_id: minimal
title: Minimal PRD
status: approved
---

Body here.
`

var invalidYAMLContent = `---
prd_id: [invalid yaml
---

Body.
`
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- `ParsePRDFromReader` is the core implementation; `ParsePRD` is a file wrapper
- YAML unmarshaling handles optional fields gracefully (zero values)
- Time fields (`FeatureStartedAt`, etc.) require RFC3339 format in YAML
- Hash is computed on the body string, not bytes (consistent encoding)
- File path is stored as provided (caller responsible for absolute/relative)

## NOT In Scope

- Frontmatter extraction logic (Task #3)
- Field validation (Task #5)
- Directory discovery (Task #6)
- Caching (Task #7)
