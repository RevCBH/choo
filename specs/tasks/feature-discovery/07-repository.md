---
task: 7
status: complete
backpressure: "go test ./internal/feature/... -run TestRepository"
depends_on: [1, 4, 6]
---

# PRD Repository

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #7 of 7 in implementation plan

## Objective

Implement a caching PRD repository with thread-safe access, refresh capability, and drift detection for PRD body content changes.

## Dependencies

### External Specs (must be implemented)
- Events module (provides: `Bus`, `NewEvent`)

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRD` struct)
- Task #4 must be complete (provides: `ParsePRD`, `ComputeBodyHash`)
- Task #6 must be complete (provides: `DiscoverPRDs`)

### Package Dependencies
- `sync` - RWMutex for thread safety

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    ├── repository.go       # CREATE: PRDRepository implementation
    └── repository_test.go  # CREATE: Repository tests
```

### Types to Implement

```go
// PRDRepository provides access to PRDs with caching
type PRDRepository struct {
    baseDir string
    cache   map[string]*PRD
    mu      sync.RWMutex
    bus     *events.Bus // optional, for event emission
}
```

### Functions to Implement

```go
// NewPRDRepository creates a repository for the given base directory
func NewPRDRepository(baseDir string) *PRDRepository

// NewPRDRepositoryWithBus creates a repository with event emission support
func NewPRDRepositoryWithBus(baseDir string, bus *events.Bus) *PRDRepository

// Get retrieves a PRD by ID, using cache if available
// Returns nil, error if PRD not found
func (r *PRDRepository) Get(id string) (*PRD, error)

// List returns all PRDs, optionally filtered by status
// Empty filter returns all PRDs
func (r *PRDRepository) List(statusFilter []string) ([]*PRD, error)

// Refresh clears the cache and re-discovers PRDs from filesystem
// Emits PRDDiscovered events if bus is configured
func (r *PRDRepository) Refresh() error

// CheckDrift compares current body hash against cached hash
// Returns true if PRD body has changed since last refresh
func (r *PRDRepository) CheckDrift(id string) (bool, error)

// WritePRDFrontmatter updates the frontmatter in a PRD file
// Preserves the body content unchanged
func WritePRDFrontmatter(prd *PRD) error
```

### Implementation Logic

```go
func (r *PRDRepository) Get(id string) (*PRD, error) {
    r.mu.RLock()
    prd, ok := r.cache[id]
    r.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("PRD not found: %s", id)
    }
    return prd, nil
}

func (r *PRDRepository) Refresh() error {
    prds, err := DiscoverPRDs(r.baseDir)
    if err != nil {
        return err
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    r.cache = make(map[string]*PRD)
    for _, prd := range prds {
        r.cache[prd.ID] = prd
        if r.bus != nil {
            r.bus.Emit(events.NewEvent(events.PRDDiscovered, "").
                WithPayload(map[string]string{
                    "prd_id": prd.ID,
                    "status": prd.Status,
                    "path":   prd.FilePath,
                }))
        }
    }

    return nil
}

func (r *PRDRepository) CheckDrift(id string) (bool, error) {
    r.mu.RLock()
    cached, ok := r.cache[id]
    r.mu.RUnlock()

    if !ok {
        return false, fmt.Errorf("PRD not found: %s", id)
    }

    current, err := ParsePRD(cached.FilePath)
    if err != nil {
        return false, err
    }

    return cached.BodyHash != current.BodyHash, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestRepository -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRepository_NewEmpty` | Creates repository with empty cache |
| `TestRepository_Refresh` | Populates cache from filesystem |
| `TestRepository_Get_Found` | Returns cached PRD by ID |
| `TestRepository_Get_NotFound` | Returns error for unknown ID |
| `TestRepository_List_All` | Returns all cached PRDs |
| `TestRepository_List_Filtered` | Returns only PRDs matching status filter |
| `TestRepository_CheckDrift_NoChange` | Returns false when body unchanged |
| `TestRepository_CheckDrift_Changed` | Returns true when body differs |
| `TestRepository_CheckDrift_NotFound` | Returns error for unknown ID |
| `TestRepository_Concurrent` | Thread-safe under concurrent access |
| `TestRepository_EventEmission` | Emits PRDDiscovered events on Refresh |
| `TestWritePRDFrontmatter_Preserves` | Body content unchanged after write |

### Test Fixtures

Use temporary directories with PRD files:

```go
func setupTestRepo(t *testing.T) (string, func()) {
    dir := t.TempDir()
    // Create test PRD files
    writeFile(filepath.Join(dir, "feature-a.md"), featureAPRD)
    writeFile(filepath.Join(dir, "feature-b.md"), featureBPRD)
    return dir, func() { /* cleanup if needed */ }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `RWMutex` for concurrent read access, exclusive write access
- `Get` and `List` use `RLock` (read lock)
- `Refresh` uses `Lock` (write lock) only when updating cache
- `CheckDrift` reads cache then re-parses file (no lock held during I/O)
- Event bus is optional (nil check before emit)
- `WritePRDFrontmatter` re-serializes frontmatter, preserves original body
- Cache is keyed by PRD ID, not file path

## NOT In Scope

- Automatic cache invalidation (manual Refresh required)
- Watch mode for filesystem changes
- PRD dependency graph construction
- Feature status state machine enforcement
