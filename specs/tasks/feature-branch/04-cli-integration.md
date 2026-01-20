---
task: 4
status: complete
backpressure: "go build ./cmd/oslo/..."
depends_on: [2, 3]
---

# CLI Integration

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/oslo/specs/FEATURE-BRANCH.md`
**Task**: #4 of 4 in implementation plan

## Objective

Add --feature flag to the run command that enables feature mode and redirects PR targets to the feature branch.

## Dependencies

### External Specs (must be implemented)
- CLI - provides run command infrastructure from `internal/cli/`
- ORCHESTRATOR - provides orchestrator config from `internal/orchestrator/`

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: BranchManager)
- Task #3 must be complete (provides: FeatureConfig)

### Package Dependencies
- `oslo/internal/feature` - BranchManager
- `oslo/internal/config` - FeatureConfig

## Deliverables

### Files to Create/Modify

```
internal/
├── cli/
│   └── run.go           # MODIFY: Add --feature flag handling
└── orchestrator/
    └── config.go        # MODIFY: Add FeatureBranch and FeatureMode fields
```

### Orchestrator Config Updates

Add fields to orchestrator Config struct:

```go
type Config struct {
    // ... existing fields ...

    FeatureBranch string // Set when --feature flag provided (e.g., "feature/streaming-events")
    FeatureMode   bool   // true when in feature mode
}
```

Add helper method:

```go
// getTargetBranch returns the appropriate target branch for PRs
// Returns FeatureBranch if in feature mode, otherwise TargetBranch
func (o *Orchestrator) getTargetBranch() string
```

### CLI Run Command Updates

Add to RunFlags struct:

```go
type RunFlags struct {
    // ... existing fields ...

    Feature string // PRD ID to work on in feature mode
}
```

Add flag registration:

```go
cmd.Flags().StringVar(&flags.Feature, "feature", "", "PRD ID for feature mode (targets feature branch)")
```

### Feature Mode Logic

In run command Execute:

```go
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

## Backpressure

### Validation Command

```bash
go build ./cmd/oslo/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `oslo run --help` | Shows --feature flag in help |
| `orchestrator.Config.FeatureMode` | Field exists on Config struct |
| `orchestrator.Config.FeatureBranch` | Field exists on Config struct |

### Test Fixtures

None required - build verification only.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The --feature flag takes a PRD ID (e.g., `oslo run --feature streaming-events`)
- Feature mode creates the branch automatically if it doesn't exist
- When in feature mode, all unit PRs target the feature branch instead of main
- The feature branch is created from the configured TargetBranch (typically "main")
- FeatureBranch stores the full branch name including prefix

## NOT In Scope

- PRD validation (handled by feature-discovery)
- Feature branch merge to main
- Feature status tracking in PRD frontmatter
- Feature completion detection
