# FEATURE-BRANCH — Feature Branch Creation and Management

## Overview

The FEATURE-BRANCH module provides feature branch creation, lifecycle management, and orchestrator integration for Oslo's feature development workflow. When working on multi-spec features derived from PRDs, this module manages the dedicated feature branch where all related unit PRs are merged before the final merge to main.

Feature branches follow a consistent naming convention (`feature/<prd-id>`) and are created automatically when the orchestrator runs in feature mode. The module integrates with the existing git client infrastructure and provides the `BranchManager` type for all branch operations. This enables the PRD workflow where specs are implemented incrementally on a feature branch, then merged to main as a cohesive unit.

The orchestrator integration allows `--feature` flag support, redirecting unit PR targets from main to the feature branch and tracking feature state throughout the development lifecycle.

```
┌─────────────────────────────────────────────────────────────┐
│                        Orchestrator                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐  │
│  │   Config    │───▶│FeatureMode │───▶│  TargetBranch   │  │
│  │  --feature  │    │   = true   │    │ feature/<prd-id>│  │
│  └─────────────┘    └─────────────┘    └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     BranchManager                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │
│  │  Create  │  │  Exists  │  │ Checkout │  │   Delete    │  │
│  └──────────┘  └──────────┘  └──────────┘  └─────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Git Client                             │
│               (internal/git/git.go)                          │
└─────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. **Branch Creation**: Create feature branches from a specified base branch (typically main) using the pattern `feature/<prd-id>`
2. **Branch Existence Check**: Verify whether a feature branch exists locally or remotely before operations
3. **Branch Checkout**: Switch the working directory to a feature branch
4. **Branch Deletion**: Remove feature branches after successful merge to main
5. **Branch Name Generation**: Generate consistent branch names from PRD IDs
6. **Configurable Prefix**: Support custom branch prefixes via configuration (default: `feature/`)
7. **Feature Lifecycle**: Track feature state including PRD association, branch name, status, and timestamps
8. **Orchestrator Integration**: Support `--feature` flag to redirect unit PR targets to feature branch

### Performance Requirements

| Metric | Target |
|--------|--------|
| Branch creation | < 2s (local), < 5s (with remote push) |
| Existence check | < 500ms |
| Checkout operation | < 1s |

### Constraints

- Depends on `internal/git` package for git operations
- Depends on `internal/prd` package for PRD type definitions
- Requires git repository with remote configured for push operations
- Branch names must be valid git ref names (no spaces, special characters)

## Design

### Module Structure

```
internal/feature/
├── feature.go      # Feature type and lifecycle
├── branch.go       # BranchManager implementation
└── feature_test.go # Unit tests
```

### Core Types

```go
// internal/feature/feature.go

package feature

import (
    "fmt"
    "time"

    "oslo/internal/prd"
)

// Status represents the current state of a feature
type Status string

const (
    StatusPending    Status = "pending"     // Feature created, no work started
    StatusInProgress Status = "in_progress" // Specs being implemented
    StatusComplete   Status = "complete"    // All specs merged to feature branch
    StatusMerged     Status = "merged"      // Feature branch merged to main
)

// Feature represents a feature being developed from a PRD
type Feature struct {
    PRD       *prd.PRD
    Branch    string
    Status    Status
    StartedAt time.Time
}

// NewFeature creates a Feature from a PRD
func NewFeature(p *prd.PRD) *Feature {
    return &Feature{
        PRD:       p,
        Branch:    fmt.Sprintf("feature/%s", p.ID),
        Status:    StatusPending,
        StartedAt: time.Now(),
    }
}

// GetBranch returns the feature branch name
func (f *Feature) GetBranch() string {
    return f.Branch
}

// SetStatus updates the feature status
func (f *Feature) SetStatus(status Status) {
    f.Status = status
}

// IsComplete returns true if all specs have been merged to the feature branch
func (f *Feature) IsComplete() bool {
    return f.Status == StatusComplete
}

// IsMerged returns true if the feature branch has been merged to main
func (f *Feature) IsMerged() bool {
    return f.Status == StatusMerged
}
```

```go
// internal/feature/branch.go

package feature

import (
    "context"
    "fmt"

    "oslo/internal/git"
)

// BranchManager handles feature branch operations
type BranchManager struct {
    git    *git.Client
    prefix string
}

// NewBranchManager creates a branch manager
func NewBranchManager(gitClient *git.Client, prefix string) *BranchManager {
    if prefix == "" {
        prefix = "feature/"
    }
    return &BranchManager{
        git:    gitClient,
        prefix: prefix,
    }
}

// Create creates a new feature branch from the target branch
func (b *BranchManager) Create(ctx context.Context, prdID, fromBranch string) error {
    branchName := b.GetBranchName(prdID)

    // Check if branch already exists
    exists, err := b.Exists(ctx, prdID)
    if err != nil {
        return fmt.Errorf("checking branch existence: %w", err)
    }
    if exists {
        return fmt.Errorf("feature branch %s already exists", branchName)
    }

    // Create the branch from the target
    if err := b.git.CreateBranch(ctx, branchName, fromBranch); err != nil {
        return fmt.Errorf("creating branch %s from %s: %w", branchName, fromBranch, err)
    }

    return nil
}

// Exists checks if a feature branch exists locally or remotely
func (b *BranchManager) Exists(ctx context.Context, prdID string) (bool, error) {
    branchName := b.GetBranchName(prdID)
    return b.git.BranchExists(ctx, branchName)
}

// Checkout switches to the feature branch
func (b *BranchManager) Checkout(ctx context.Context, prdID string) error {
    branchName := b.GetBranchName(prdID)

    exists, err := b.Exists(ctx, prdID)
    if err != nil {
        return fmt.Errorf("checking branch existence: %w", err)
    }
    if !exists {
        return fmt.Errorf("feature branch %s does not exist", branchName)
    }

    return b.git.Checkout(ctx, branchName)
}

// Delete removes a feature branch (after merge)
func (b *BranchManager) Delete(ctx context.Context, prdID string) error {
    branchName := b.GetBranchName(prdID)
    return b.git.DeleteBranch(ctx, branchName)
}

// GetBranchName returns the full branch name for a PRD ID
func (b *BranchManager) GetBranchName(prdID string) string {
    return b.prefix + prdID
}

// GetPrefix returns the configured branch prefix
func (b *BranchManager) GetPrefix() string {
    return b.prefix
}
```

```go
// Updates to internal/config/config.go

type FeatureConfig struct {
    PRDDir       string `yaml:"prd_dir"`
    SpecsDir     string `yaml:"specs_dir"`
    BranchPrefix string `yaml:"branch_prefix"`
}

// Default values
func DefaultFeatureConfig() FeatureConfig {
    return FeatureConfig{
        PRDDir:       "docs/prds",
        SpecsDir:     "specs",
        BranchPrefix: "feature/",
    }
}
```

```go
// Updates to internal/orchestrator/config.go

type Config struct {
    // ... existing fields ...

    FeatureBranch string  // Set when --feature flag provided (e.g., "feature/streaming-events")
    FeatureMode   bool    // true when in feature mode
}

// getTargetBranch returns the appropriate target branch for PRs
func (o *Orchestrator) getTargetBranch() string {
    if o.cfg.FeatureMode && o.cfg.FeatureBranch != "" {
        return o.cfg.FeatureBranch
    }
    return o.cfg.TargetBranch
}
```

```go
// Updates to internal/cli/run.go

type RunFlags struct {
    // ... existing fields ...

    Feature string // PRD ID to work on in feature mode
}

func (r *RunCmd) Execute(ctx context.Context) error {
    cfg := orchestrator.Config{
        // ... existing config ...
    }

    // Configure feature mode if --feature flag provided
    if r.flags.Feature != "" {
        branchMgr := feature.NewBranchManager(gitClient, r.cfg.Feature.BranchPrefix)

        cfg.FeatureMode = true
        cfg.FeatureBranch = branchMgr.GetBranchName(r.flags.Feature)

        // Ensure feature branch exists
        exists, err := branchMgr.Exists(ctx, r.flags.Feature)
        if err != nil {
            return fmt.Errorf("checking feature branch: %w", err)
        }
        if !exists {
            // Create the feature branch from main
            if err := branchMgr.Create(ctx, r.flags.Feature, cfg.TargetBranch); err != nil {
                return fmt.Errorf("creating feature branch: %w", err)
            }
        }
    }

    return orchestrator.New(cfg).Run(ctx)
}
```

### API Surface

| Function | Description |
|----------|-------------|
| `NewFeature(prd *prd.PRD) *Feature` | Create a Feature from a PRD |
| `Feature.GetBranch() string` | Get the feature branch name |
| `Feature.SetStatus(status Status)` | Update feature status |
| `Feature.IsComplete() bool` | Check if feature is complete |
| `Feature.IsMerged() bool` | Check if feature is merged |
| `NewBranchManager(git *git.Client, prefix string) *BranchManager` | Create a branch manager |
| `BranchManager.Create(ctx, prdID, fromBranch string) error` | Create a feature branch |
| `BranchManager.Exists(ctx, prdID string) (bool, error)` | Check if branch exists |
| `BranchManager.Checkout(ctx, prdID string) error` | Switch to feature branch |
| `BranchManager.Delete(ctx, prdID string) error` | Delete feature branch |
| `BranchManager.GetBranchName(prdID string) string` | Get full branch name |

## Implementation Notes

1. **Branch Name Sanitization**: PRD IDs should be validated to ensure they produce valid git branch names. Consider adding a sanitization function that replaces invalid characters.

2. **Remote Tracking**: When creating branches, consider whether to automatically set up remote tracking. The current design creates local branches only; pushing is handled separately.

3. **Concurrent Access**: The BranchManager is safe for concurrent use as it doesn't maintain mutable state beyond configuration. Git operations are serialized by git itself.

4. **Error Recovery**: If branch creation fails mid-operation, the system should be able to recover on retry. The `Exists` check prevents duplicate creation attempts.

5. **Feature State Persistence**: Feature status is tracked in PRD frontmatter, not in-memory. The Feature struct is a runtime representation hydrated from PRD state.

## Testing Strategy

### Unit Tests

```go
// internal/feature/feature_test.go

func TestNewFeature(t *testing.T) {
    p := &prd.PRD{ID: "streaming-events"}
    f := NewFeature(p)

    assert.Equal(t, "feature/streaming-events", f.GetBranch())
    assert.Equal(t, StatusPending, f.Status)
    assert.NotZero(t, f.StartedAt)
}

func TestFeatureStatus(t *testing.T) {
    p := &prd.PRD{ID: "test"}
    f := NewFeature(p)

    assert.False(t, f.IsComplete())
    assert.False(t, f.IsMerged())

    f.SetStatus(StatusComplete)
    assert.True(t, f.IsComplete())

    f.SetStatus(StatusMerged)
    assert.True(t, f.IsMerged())
}

func TestBranchManagerGetBranchName(t *testing.T) {
    tests := []struct {
        prefix string
        prdID  string
        want   string
    }{
        {"feature/", "streaming-events", "feature/streaming-events"},
        {"feat/", "multi-repo", "feat/multi-repo"},
        {"", "webhooks", "feature/webhooks"}, // default prefix
    }

    for _, tt := range tests {
        bm := NewBranchManager(nil, tt.prefix)
        got := bm.GetBranchName(tt.prdID)
        assert.Equal(t, tt.want, got)
    }
}

func TestBranchManagerCreate(t *testing.T) {
    mockGit := &git.MockClient{}
    mockGit.On("BranchExists", mock.Anything, "feature/test-prd").Return(false, nil)
    mockGit.On("CreateBranch", mock.Anything, "feature/test-prd", "main").Return(nil)

    bm := NewBranchManager(mockGit, "feature/")
    err := bm.Create(context.Background(), "test-prd", "main")

    assert.NoError(t, err)
    mockGit.AssertExpectations(t)
}

func TestBranchManagerCreateAlreadyExists(t *testing.T) {
    mockGit := &git.MockClient{}
    mockGit.On("BranchExists", mock.Anything, "feature/test-prd").Return(true, nil)

    bm := NewBranchManager(mockGit, "feature/")
    err := bm.Create(context.Background(), "test-prd", "main")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already exists")
}
```

### Integration Tests

```go
// internal/feature/integration_test.go

func TestBranchManagerIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create temp git repo
    dir := t.TempDir()
    setupGitRepo(t, dir)

    gitClient := git.NewClient(dir)
    bm := NewBranchManager(gitClient, "feature/")
    ctx := context.Background()

    // Test create
    err := bm.Create(ctx, "test-feature", "main")
    require.NoError(t, err)

    // Test exists
    exists, err := bm.Exists(ctx, "test-feature")
    require.NoError(t, err)
    assert.True(t, exists)

    // Test checkout
    err = bm.Checkout(ctx, "test-feature")
    require.NoError(t, err)

    // Verify current branch
    current, _ := gitClient.CurrentBranch(ctx)
    assert.Equal(t, "feature/test-feature", current)

    // Test delete (switch back to main first)
    _ = gitClient.Checkout(ctx, "main")
    err = bm.Delete(ctx, "test-feature")
    require.NoError(t, err)

    // Verify deleted
    exists, _ = bm.Exists(ctx, "test-feature")
    assert.False(t, exists)
}
```

## Design Decisions

1. **Prefix as Configuration**: The branch prefix (`feature/`) is configurable rather than hardcoded to support teams with different naming conventions. Default ensures consistency for most users.

2. **Stateless BranchManager**: The BranchManager doesn't track state internally. All state is derived from git itself, making the system resilient to restarts and concurrent access.

3. **Feature Status in PRD**: Feature status lives in PRD frontmatter rather than a separate database. This keeps all feature metadata colocated and version-controlled.

4. **Explicit Error on Existing Branch**: Creating a branch that already exists returns an error rather than silently succeeding. This prevents accidental state corruption and makes the operation idempotent-safe (callers can check Exists first).

5. **Local-First Operations**: Branch operations are local by default. Remote push/pull operations are handled separately by the orchestrator, keeping concerns separated.

## Future Enhancements

1. **Branch Protection Rules**: Automatically configure branch protection on feature branches via GitHub API
2. **Stale Branch Detection**: Identify feature branches that haven't received commits in N days
3. **Branch Metrics**: Track time-to-merge, number of specs completed, and other metrics per feature
4. **Auto-Cleanup**: Automatically delete merged feature branches after configurable retention period
5. **Branch Dependencies**: Support feature branches that depend on other feature branches

## References

- PRD Section 5: Feature Branch Management
- PRD Section 7: Configuration Additions
- PRD Section 10 Phase 3: Feature Branch Management
- `internal/git/git.go` - Git client interface
- `internal/prd/prd.go` - PRD type definitions
