---
task: 6
status: complete
backpressure: "go test ./internal/feature/... -run TestDiscover"
depends_on: [1, 4, 5]
---

# Discovery Functions

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #6 of 7 in implementation plan

## Objective

Implement filesystem discovery functions to find PRD files in `docs/prds/` directory with optional status filtering.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRD` struct, status constants)
- Task #4 must be complete (provides: `ParsePRD` function)
- Task #5 must be complete (provides: `ValidatePRD` function)

### Package Dependencies
- `io/fs` - Directory walking
- `path/filepath` - Path manipulation
- `log/slog` - Warning logging

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── discovery.go    # MODIFY: Add discovery functions
```

### Functions to Implement

```go
// DiscoverPRDs finds all PRD files in the given directory recursively
// Skips README.md files and logs warnings for invalid PRDs
func DiscoverPRDs(baseDir string) ([]*PRD, error)

// DiscoverPRDsWithFilter finds PRDs matching the given status filter
// Empty filter returns all PRDs
func DiscoverPRDsWithFilter(baseDir string, statusFilter []string) ([]*PRD, error)
```

### Implementation Logic

```go
func DiscoverPRDs(baseDir string) ([]*PRD, error) {
    var prds []*PRD

    err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        // Skip directories and non-markdown files
        if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
            return nil
        }

        // Skip README files
        if strings.ToLower(d.Name()) == "readme.md" {
            return nil
        }

        prd, err := ParsePRD(path)
        if err != nil {
            // Log warning but continue discovery
            slog.Warn("failed to parse PRD", "path", path, "error", err)
            return nil
        }

        if err := ValidatePRD(prd); err != nil {
            slog.Warn("invalid PRD", "path", path, "error", err)
            return nil
        }

        prds = append(prds, prd)
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("discovery failed: %w", err)
    }

    return prds, nil
}

func DiscoverPRDsWithFilter(baseDir string, statusFilter []string) ([]*PRD, error) {
    prds, err := DiscoverPRDs(baseDir)
    if err != nil {
        return nil, err
    }

    if len(statusFilter) == 0 {
        return prds, nil
    }

    filterSet := make(map[string]bool)
    for _, s := range statusFilter {
        filterSet[s] = true
    }

    var filtered []*PRD
    for _, prd := range prds {
        if filterSet[prd.Status] {
            filtered = append(filtered, prd)
        }
    }

    return filtered, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestDiscover -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDiscoverPRDs_Empty` | Returns empty slice for empty directory |
| `TestDiscoverPRDs_SingleFile` | Discovers single PRD file |
| `TestDiscoverPRDs_MultipleFiles` | Discovers all PRD files in directory |
| `TestDiscoverPRDs_Recursive` | Discovers PRDs in subdirectories |
| `TestDiscoverPRDs_SkipsReadme` | Does not include README.md |
| `TestDiscoverPRDs_SkipsInvalid` | Continues on invalid PRD, logs warning |
| `TestDiscoverPRDs_NonExistentDir` | Returns error for missing directory |
| `TestDiscoverPRDsWithFilter_NoFilter` | Returns all PRDs when filter is empty |
| `TestDiscoverPRDsWithFilter_SingleStatus` | Returns only PRDs with matching status |
| `TestDiscoverPRDsWithFilter_MultipleStatuses` | Returns PRDs matching any status in filter |
| `TestDiscoverPRDsWithFilter_NoMatches` | Returns empty slice when no PRDs match |

### Test Fixtures

Create temporary directory structure in tests:

```
testdata/
└── prds/
    ├── README.md          # Should be skipped
    ├── feature-a.md       # status: draft
    ├── feature-b.md       # status: approved
    ├── invalid.md         # Invalid YAML (should warn, skip)
    └── nested/
        └── feature-c.md   # status: draft
```

```go
var featureAPRD = `---
prd_id: feature-a
title: Feature A
status: draft
---

# Feature A
`

var featureBPRD = `---
prd_id: feature-b
title: Feature B
status: approved
---

# Feature B
`
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `filepath.WalkDir` for efficient directory traversal
- Skip directories early (check `d.IsDir()` first)
- Case-insensitive README check (`strings.ToLower`)
- Log warnings with slog for parse/validation errors but continue
- Filter is case-sensitive (must match status constants exactly)
- Empty filter slice means "all statuses"

## NOT In Scope

- Caching discovered PRDs (Task #7)
- Event emission on discovery (Task #7)
- Writing PRD frontmatter updates
