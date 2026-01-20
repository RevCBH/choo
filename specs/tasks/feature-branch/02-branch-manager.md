---
task: 2
status: complete
backpressure: "go test ./internal/feature/... -run TestBranchManager"
depends_on: [1]
---

# Branch Manager

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/oslo/specs/FEATURE-BRANCH.md`
**Task**: #2 of 4 in implementation plan

## Objective

Implement BranchManager for feature branch CRUD operations using the existing git client.

## Dependencies

### External Specs (must be implemented)
- GIT - provides git client operations from `internal/git/`

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Feature`, `Status`)

### Package Dependencies
- `oslo/internal/git` - git operations

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    ├── branch.go       # CREATE: BranchManager implementation
    └── branch_test.go  # CREATE: Unit tests
```

### Types to Implement

```go
// BranchManager handles feature branch operations
type BranchManager struct {
    git    *git.Client  // Git client for operations
    prefix string       // Branch prefix (default: "feature/")
}
```

### Functions to Implement

```go
// NewBranchManager creates a branch manager with the given git client and prefix
// If prefix is empty, defaults to "feature/"
func NewBranchManager(gitClient *git.Client, prefix string) *BranchManager

// Create creates a new feature branch from the target branch
// Returns error if branch already exists
func (b *BranchManager) Create(ctx context.Context, prdID, fromBranch string) error

// Exists checks if a feature branch exists locally or remotely
func (b *BranchManager) Exists(ctx context.Context, prdID string) (bool, error)

// Checkout switches to the feature branch
// Returns error if branch does not exist
func (b *BranchManager) Checkout(ctx context.Context, prdID string) error

// Delete removes a feature branch (typically after merge)
func (b *BranchManager) Delete(ctx context.Context, prdID string) error

// GetBranchName returns the full branch name for a PRD ID
// Concatenates prefix + prdID (e.g., "feature/" + "streaming-events")
func (b *BranchManager) GetBranchName(prdID string) string

// GetPrefix returns the configured branch prefix
func (b *BranchManager) GetPrefix() string
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestBranchManager
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewBranchManager_DefaultPrefix` | Empty prefix defaults to "feature/" |
| `TestNewBranchManager_CustomPrefix` | Custom prefix is used |
| `TestBranchManager_GetBranchName` | Returns prefix + prdID |
| `TestBranchManager_Create` | Creates branch from target |
| `TestBranchManager_Create_AlreadyExists` | Returns error when branch exists |
| `TestBranchManager_Exists` | Returns true for existing branch |
| `TestBranchManager_Checkout` | Switches to branch |
| `TestBranchManager_Checkout_NotExists` | Returns error when branch missing |
| `TestBranchManager_Delete` | Removes branch |

### Test Fixtures

None required - uses mock git client.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use the existing git client from `internal/git/`
- The git client should have `CreateBranch`, `BranchExists`, `Checkout`, `DeleteBranch` methods
- If these methods don't exist, they need to be added to the git package first
- Create should check Exists first and return error if branch already exists
- Checkout should verify branch exists before attempting checkout
- All operations take context for cancellation support

### Error Handling

- `Create` when branch exists: `fmt.Errorf("feature branch %s already exists", branchName)`
- `Checkout` when branch missing: `fmt.Errorf("feature branch %s does not exist", branchName)`
- Wrap git client errors with context: `fmt.Errorf("creating branch %s from %s: %w", branchName, fromBranch, err)`

## NOT In Scope

- Remote push operations (handled separately by orchestrator)
- Branch protection rules
- Stale branch detection
- Configuration loading (Task #3)
- CLI integration (Task #4)
