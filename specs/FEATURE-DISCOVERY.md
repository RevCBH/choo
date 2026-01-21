# FEATURE-DISCOVERY — PRD Frontmatter Parsing and Discovery

## Overview

The Feature Discovery module provides the foundation for PRD-driven workflow orchestration. It handles parsing YAML frontmatter from PRD files stored in `docs/prd/`, discovering available PRDs from the filesystem, and defines the event types that track feature lifecycle state changes.

This module serves as the entry point for the feature workflow system. Before any feature can be processed, its PRD must be discovered and parsed. The module enforces a single-writer model where only the orchestrator on a feature branch updates PRD frontmatter, ensuring consistent state management across parallel unit work.

The discovery system also supports drift detection by computing content hashes of PRD bodies. When the orchestrator detects that a PRD's content has changed since specs were generated, it can trigger re-validation or regeneration as needed.

## Requirements

### Functional Requirements

1. Parse YAML frontmatter from PRD files with all required fields (prd_id, title, status)
2. Parse optional dependency hints (depends_on array of PRD IDs)
3. Parse optional complexity estimates (estimated_units, estimated_tasks)
4. Parse orchestrator-managed fields (feature_branch, feature_status, timestamps, review tracking)
5. Extract markdown body content after frontmatter delimiter
6. Compute SHA-256 hash of body content for drift detection
7. Discover all PRD files in `docs/prd/` directory recursively
8. Filter PRDs by status (draft, approved, in_progress, complete, archived)
9. Validate PRD frontmatter against required field schema
10. Emit feature and PRD lifecycle events through the event bus

### Performance Requirements

| Metric | Target |
|--------|--------|
| Single PRD parse time | < 5ms |
| Directory discovery (100 PRDs) | < 100ms |
| Frontmatter validation | < 1ms |
| Body hash computation | < 2ms per KB |

### Constraints

- PRD files must use `.md` extension
- Frontmatter must be delimited by `---` markers
- PRD IDs must be valid for use in git branch names (no spaces, special chars)
- Only orchestrator process may update frontmatter fields
- Unit PRs must not modify PRD frontmatter

## Design

### Module Structure

```
internal/feature/
├── discovery.go      # PRD parsing and filesystem discovery
├── discovery_test.go # Unit tests for parsing and discovery
├── types.go          # PRD struct and status constants
└── events.go         # Feature-specific event type definitions
```

### Core Types

```go
// PRD represents a Product Requirements Document
type PRD struct {
    // Required fields
    ID     string `yaml:"prd_id"`
    Title  string `yaml:"title"`
    Status string `yaml:"status"` // draft | approved | in_progress | complete | archived

    // Optional dependency hints
    DependsOn []string `yaml:"depends_on,omitempty"`

    // Complexity estimates
    EstimatedUnits int `yaml:"estimated_units,omitempty"`
    EstimatedTasks int `yaml:"estimated_tasks,omitempty"`

    // Orchestrator-managed fields (updated at runtime)
    FeatureBranch        string     `yaml:"feature_branch,omitempty"`
    FeatureStatus        string     `yaml:"feature_status,omitempty"`
    FeatureStartedAt     *time.Time `yaml:"feature_started_at,omitempty"`
    FeatureCompletedAt   *time.Time `yaml:"feature_completed_at,omitempty"`
    SpecReviewIterations int        `yaml:"spec_review_iterations,omitempty"`
    LastSpecReview       *time.Time `yaml:"last_spec_review,omitempty"`

    // File metadata (not in frontmatter)
    FilePath string `yaml:"-"`
    Body     string `yaml:"-"` // Markdown content after frontmatter
    BodyHash string `yaml:"-"` // SHA-256 for drift detection
}

// PRDStatus values for the status field
const (
    PRDStatusDraft      = "draft"
    PRDStatusApproved   = "approved"
    PRDStatusInProgress = "in_progress"
    PRDStatusComplete   = "complete"
    PRDStatusArchived   = "archived"
)

// FeatureStatus values for orchestrator-managed feature_status field
const (
    FeatureStatusPending         = "pending"
    FeatureStatusGeneratingSpecs = "generating_specs"
    FeatureStatusReviewingSpecs  = "reviewing_specs"
    FeatureStatusReviewBlocked   = "review_blocked"
    FeatureStatusValidatingSpecs = "validating_specs"
    FeatureStatusGeneratingTasks = "generating_tasks"
    FeatureStatusSpecsCommitted  = "specs_committed"
    FeatureStatusInProgress      = "in_progress"
    FeatureStatusUnitsComplete   = "units_complete"
    FeatureStatusPROpen          = "pr_open"
    FeatureStatusComplete        = "complete"
    FeatureStatusFailed          = "failed"
)

// ValidationError represents a frontmatter validation failure
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("invalid PRD: %s: %s", e.Field, e.Message)
}
```

### Event Types

Add to `internal/events/types.go`:

```go
// Feature lifecycle events
const (
    FeatureStarted        EventType = "feature.started"
    FeatureSpecsGenerated EventType = "feature.specs.generated"
    FeatureSpecsReviewed  EventType = "feature.specs.reviewed"
    FeatureSpecsCommitted EventType = "feature.specs.committed"
    FeatureTasksGenerated EventType = "feature.tasks.generated"
    FeatureUnitsComplete  EventType = "feature.units.complete"
    FeaturePROpened       EventType = "feature.pr.opened"
    FeatureCompleted      EventType = "feature.completed"
    FeatureFailed         EventType = "feature.failed"
)

// PRD events
const (
    PRDDiscovered    EventType = "prd.discovered"
    PRDSelected      EventType = "prd.selected"
    PRDUpdated       EventType = "prd.updated"
    PRDBodyChanged   EventType = "prd.body.changed"
    PRDDriftDetected EventType = "prd.drift.detected"
)
```

### API Surface

```go
// ParsePRD reads a PRD file and parses its frontmatter and body
func ParsePRD(filePath string) (*PRD, error)

// ParsePRDFromReader parses a PRD from an io.Reader (for testing)
func ParsePRDFromReader(r io.Reader, filePath string) (*PRD, error)

// ValidatePRD checks that all required fields are present and valid
func ValidatePRD(prd *PRD) error

// DiscoverPRDs finds all PRD files in the given directory
func DiscoverPRDs(baseDir string) ([]*PRD, error)

// DiscoverPRDsWithFilter finds PRDs matching the given status filter
func DiscoverPRDsWithFilter(baseDir string, statusFilter []string) ([]*PRD, error)

// ComputeBodyHash returns SHA-256 hash of the PRD body content
func ComputeBodyHash(body string) string

// WritePRDFrontmatter updates the frontmatter in a PRD file
// Preserves the body content unchanged
func WritePRDFrontmatter(prd *PRD) error

// PRDRepository provides access to PRDs with caching
type PRDRepository struct {
    baseDir string
    cache   map[string]*PRD
    mu      sync.RWMutex
}

// NewPRDRepository creates a repository for the given base directory
func NewPRDRepository(baseDir string) *PRDRepository

// Get retrieves a PRD by ID, using cache if available
func (r *PRDRepository) Get(id string) (*PRD, error)

// List returns all PRDs, optionally filtered by status
func (r *PRDRepository) List(statusFilter []string) ([]*PRD, error)

// Refresh clears the cache and re-discovers PRDs
func (r *PRDRepository) Refresh() error

// CheckDrift compares current body hash against stored hash
func (r *PRDRepository) CheckDrift(id string) (bool, error)
```

### Frontmatter Parsing

The parser extracts YAML frontmatter between `---` delimiters:

```go
// parseFrontmatter splits content into frontmatter YAML and body markdown
func parseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error) {
    // Content must start with "---\n"
    if !bytes.HasPrefix(content, []byte("---\n")) {
        return nil, nil, fmt.Errorf("missing frontmatter delimiter")
    }

    // Find closing delimiter
    rest := content[4:] // Skip opening "---\n"
    idx := bytes.Index(rest, []byte("\n---\n"))
    if idx == -1 {
        // Try "---" at end of file
        idx = bytes.Index(rest, []byte("\n---"))
        if idx == -1 || idx+4 != len(rest) {
            return nil, nil, fmt.Errorf("missing closing frontmatter delimiter")
        }
    }

    frontmatter = rest[:idx]
    body = rest[idx+5:] // Skip "\n---\n"

    return frontmatter, body, nil
}
```

### Discovery Implementation

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
```

### Drift Detection

```go
func ComputeBodyHash(body string) string {
    h := sha256.New()
    h.Write([]byte(body))
    return hex.EncodeToString(h.Sum(nil))
}

func (r *PRDRepository) CheckDrift(id string) (bool, error) {
    // Get cached PRD
    r.mu.RLock()
    cached, ok := r.cache[id]
    r.mu.RUnlock()

    if !ok {
        return false, fmt.Errorf("PRD not found: %s", id)
    }

    // Re-read file from disk
    current, err := ParsePRD(cached.FilePath)
    if err != nil {
        return false, err
    }

    // Compare hashes
    return cached.BodyHash != current.BodyHash, nil
}
```

## Implementation Notes

### ID Validation for Branch Names

PRD IDs are used in git branch names (`feature/<prd_id>`), so they must be validated:

```go
var validPRDID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validatePRDID(id string) error {
    if len(id) < 2 {
        return fmt.Errorf("prd_id too short (minimum 2 characters)")
    }
    if len(id) > 50 {
        return fmt.Errorf("prd_id too long (maximum 50 characters)")
    }
    if !validPRDID.MatchString(id) {
        return fmt.Errorf("prd_id must be lowercase alphanumeric with hyphens")
    }
    return nil
}
```

### Time Field Handling

YAML time parsing requires careful handling of null values:

```go
// Custom time unmarshaling that handles null/empty values
type nullableTime struct {
    time.Time
    Valid bool
}

func (t *nullableTime) UnmarshalYAML(unmarshal func(interface{}) error) error {
    var s string
    if err := unmarshal(&s); err == nil {
        if s == "" || s == "null" {
            t.Valid = false
            return nil
        }
        parsed, err := time.Parse(time.RFC3339, s)
        if err != nil {
            return err
        }
        t.Time = parsed
        t.Valid = true
    }
    return nil
}
```

### Concurrent Access

The PRDRepository uses RWMutex for thread-safe access:

- Read operations (Get, List) use RLock
- Write operations (Refresh, cache updates) use Lock
- Drift detection reads cache then re-parses file (no lock held during I/O)

### Event Emission

Discovery emits events for monitoring:

```go
func (r *PRDRepository) Refresh() error {
    prds, err := DiscoverPRDs(r.baseDir)
    if err != nil {
        return err
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Clear cache and repopulate
    r.cache = make(map[string]*PRD)
    for _, prd := range prds {
        r.cache[prd.ID] = prd
        // Emit discovery event
        r.bus.Emit(events.NewEvent(events.PRDDiscovered, "").
            WithPayload(map[string]string{
                "prd_id": prd.ID,
                "status": prd.Status,
                "path":   prd.FilePath,
            }))
    }

    return nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestParsePRD(t *testing.T) {
    content := `---
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
    prd, err := ParsePRDFromReader(strings.NewReader(content), "test.md")
    if err != nil {
        t.Fatalf("ParsePRD failed: %v", err)
    }

    if prd.ID != "test-feature" {
        t.Errorf("ID = %q, want %q", prd.ID, "test-feature")
    }
    if prd.Title != "Test Feature" {
        t.Errorf("Title = %q, want %q", prd.Title, "Test Feature")
    }
    if prd.Status != PRDStatusDraft {
        t.Errorf("Status = %q, want %q", prd.Status, PRDStatusDraft)
    }
    if len(prd.DependsOn) != 1 || prd.DependsOn[0] != "other-feature" {
        t.Errorf("DependsOn = %v, want [other-feature]", prd.DependsOn)
    }
    if prd.EstimatedUnits != 3 {
        t.Errorf("EstimatedUnits = %d, want 3", prd.EstimatedUnits)
    }
    if !strings.Contains(prd.Body, "PRD body content") {
        t.Errorf("Body does not contain expected content")
    }
    if prd.BodyHash == "" {
        t.Error("BodyHash should be computed")
    }
}

func TestValidatePRD_MissingRequired(t *testing.T) {
    tests := []struct {
        name    string
        prd     *PRD
        wantErr string
    }{
        {
            name:    "missing prd_id",
            prd:     &PRD{Title: "Test", Status: "draft"},
            wantErr: "prd_id",
        },
        {
            name:    "missing title",
            prd:     &PRD{ID: "test", Status: "draft"},
            wantErr: "title",
        },
        {
            name:    "missing status",
            prd:     &PRD{ID: "test", Title: "Test"},
            wantErr: "status",
        },
        {
            name:    "invalid status",
            prd:     &PRD{ID: "test", Title: "Test", Status: "unknown"},
            wantErr: "status",
        },
        {
            name:    "invalid prd_id format",
            prd:     &PRD{ID: "Test Feature!", Title: "Test", Status: "draft"},
            wantErr: "prd_id",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePRD(tt.prd)
            if err == nil {
                t.Fatal("expected error, got nil")
            }
            if !strings.Contains(err.Error(), tt.wantErr) {
                t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
            }
        })
    }
}

func TestComputeBodyHash(t *testing.T) {
    body := "# Test Content\n\nSome markdown here."
    hash1 := ComputeBodyHash(body)
    hash2 := ComputeBodyHash(body)

    if hash1 != hash2 {
        t.Error("same content should produce same hash")
    }

    hash3 := ComputeBodyHash(body + " modified")
    if hash1 == hash3 {
        t.Error("different content should produce different hash")
    }

    // Verify it's a valid SHA-256 hex string (64 chars)
    if len(hash1) != 64 {
        t.Errorf("hash length = %d, want 64", len(hash1))
    }
}

func TestParseFrontmatter_EdgeCases(t *testing.T) {
    tests := []struct {
        name    string
        content string
        wantErr bool
    }{
        {
            name:    "no frontmatter",
            content: "# Just markdown",
            wantErr: true,
        },
        {
            name:    "unclosed frontmatter",
            content: "---\nprd_id: test\n# Content",
            wantErr: true,
        },
        {
            name:    "empty frontmatter",
            content: "---\n---\n# Content",
            wantErr: false, // Valid syntax, validation catches missing fields
        },
        {
            name:    "frontmatter at end of file",
            content: "---\nprd_id: test\n---",
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, _, err := parseFrontmatter([]byte(tt.content))
            if (err != nil) != tt.wantErr {
                t.Errorf("parseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

1. **Filesystem Discovery**: Create temp directory with multiple PRD files, verify all are discovered
2. **Filter by Status**: Create PRDs with different statuses, verify filtering works
3. **Drift Detection**: Modify PRD body after caching, verify drift is detected
4. **Concurrent Access**: Multiple goroutines reading/refreshing repository
5. **Event Emission**: Verify PRDDiscovered events are emitted during Refresh

### Manual Testing

- [ ] Create PRD file in `docs/prd/` with valid frontmatter
- [ ] Create PRD with missing required field, verify validation error
- [ ] Create PRD with invalid prd_id format, verify error message
- [ ] Run discovery on directory with mixed valid/invalid PRDs
- [ ] Modify PRD body, verify drift detection works
- [ ] Verify orchestrator-managed fields are preserved on write

## Design Decisions

### Why YAML Frontmatter?

YAML frontmatter is a well-established convention in static site generators and documentation tools. It provides:
- Human-readable metadata that doesn't clutter the document
- Easy parsing with standard YAML libraries
- Clear separation between metadata and content
- Compatibility with GitHub markdown rendering (frontmatter is hidden)

### Why SHA-256 for Drift Detection?

- Cryptographically strong hash prevents accidental collisions
- Fast enough for our file sizes (< 1MB typical)
- Hex encoding produces consistent 64-character string
- No external dependencies (stdlib crypto/sha256)

### Why Single-Writer Model?

Multiple writers updating PRD frontmatter would create merge conflicts and race conditions. The single-writer model ensures:
- No merge conflicts on feature branches
- Predictable state transitions
- Clear audit trail (all changes from orchestrator)
- Unit workers can safely read PRD state without locking

### Why Separate Body Hash from Frontmatter?

The body hash is computed at parse time and stored in memory, not in the frontmatter. This keeps the frontmatter clean and avoids recursive modification issues (changing the hash would change the content, which would change the hash).

## Future Enhancements

1. **PRD Templates**: Support for different PRD types with custom frontmatter schemas
2. **Dependency Graph**: Build DAG from depends_on fields for feature ordering
3. **Status Transitions**: Enforce valid state machine transitions for feature_status
4. **Watch Mode**: Filesystem watcher for automatic PRD re-discovery
5. **Remote PRDs**: Support for PRDs stored in external systems (e.g., Notion, Confluence)
6. **Frontmatter Migrations**: Versioned schema with automatic migration support

## References

- PRD Section 2: PRD Storage and Format
- PRD Section 8: Event Types
- PRD Section 10 Phase 1: PRD Foundation
- Related: `internal/events/types.go` for existing event type patterns
