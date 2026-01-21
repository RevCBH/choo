---
task: 4
status: complete
backpressure: "go test ./internal/feature/... -run TestCompletion"
depends_on: [1]
---

# Auto-Triggered Completion

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement completion checking that detects when all units have merged and triggers feature PR creation.

## Dependencies

### External Specs (must be implemented)
- GIT - provides `git.Client` for branch operations
- GITHUB - provides `github.Client` for PR operations
- EVENTS - provides `events.Bus` for event emission

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`)

### Package Dependencies
- `internal/git` - git operations
- `internal/github` - GitHub PR operations
- `internal/events` - event bus

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── completion.go    # CREATE: Completion logic
```

### Types to Implement

```go
// CompletionChecker monitors unit completion and triggers feature PR
type CompletionChecker struct {
    prd    *PRD
    git    *git.Client
    github *github.Client
    events *events.Bus
}

// CompletionStatus holds the current completion state
type CompletionStatus struct {
    AllUnitsMerged bool
    BranchClean    bool
    ExistingPR     *github.PullRequest
    ReadyForPR     bool
}
```

### Functions to Implement

```go
// NewCompletionChecker creates a completion checker for the feature
func NewCompletionChecker(prd *PRD, gitClient *git.Client, ghClient *github.Client, bus *events.Bus) *CompletionChecker

// Check evaluates completion conditions and returns status
func (c *CompletionChecker) Check(ctx context.Context) (*CompletionStatus, error)

// TriggerCompletion creates the feature PR if conditions are met
func (c *CompletionChecker) TriggerCompletion(ctx context.Context) error

// allUnitsComplete checks if all units have merged PRs
func (c *CompletionChecker) allUnitsComplete() (bool, error)

// branchIsClean checks for uncommitted changes
func (c *CompletionChecker) branchIsClean() bool

// findExistingPR checks if a feature PR already exists
func (c *CompletionChecker) findExistingPR(ctx context.Context) (*github.PullRequest, error)

// openFeaturePR creates the PR from feature branch to main
func (c *CompletionChecker) openFeaturePR(ctx context.Context) error
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestCompletion
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCompletionChecker_AllUnitsComplete` | Returns `true` when all units merged |
| `TestCompletionChecker_SomeUnitsPending` | Returns `false` when units pending |
| `TestCompletionChecker_BranchClean` | Returns `true` when no uncommitted changes |
| `TestCompletionChecker_BranchDirty` | Returns `false` when uncommitted changes |
| `TestCompletionChecker_ExistingPR` | Detects existing PR, skips creation |
| `TestCompletionChecker_IdempotentPROpen` | No-op when status is `pr_open` |
| `TestCompletionChecker_IdempotentComplete` | No-op when status is `complete` |
| `TestCompletionChecker_TriggerSuccess` | Creates PR, updates state to `pr_open` |
| `TestCompletionChecker_ReadyForPR` | `ReadyForPR` true when all conditions met |

### Test Fixtures

None required - uses mocked git and github clients.

### CI Compatibility

- [x] No external API keys required (mock GitHub client)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Trigger conditions (all must be true):
  1. All units in `specs/tasks/<feature>/` have merged PRs
  2. Feature branch is clean (no uncommitted changes)
  3. No pending or in-progress units remain

- Idempotency checks:
  - If PR already exists: Update state to `pr_open`, log, skip creation
  - If state is `pr_open` or `complete`: No-op with log message

```go
func (o *Orchestrator) checkFeatureCompletion(ctx context.Context) error {
    if !o.cfg.FeatureMode {
        return nil
    }

    prd, err := o.loadPRD()
    if err != nil {
        return err
    }

    // Idempotency: already done
    if prd.FeatureStatus == "pr_open" || prd.FeatureStatus == "complete" {
        return nil
    }

    allComplete, err := o.allUnitsComplete()
    if err != nil || !allComplete {
        return err
    }

    if !o.branchIsClean() {
        return nil // Not ready yet
    }

    // Check for existing PR
    existingPR, err := o.findExistingPR()
    if err != nil {
        return err
    }
    if existingPR != nil {
        prd.FeatureStatus = "pr_open"
        return o.savePRD(prd)
    }

    return o.openFeaturePR(ctx)
}
```

## NOT In Scope

- State definitions (Task #1)
- Commit operations (Task #2)
- Drift detection (Task #3)
- Review cycle management (Task #5)
- Workflow orchestration (Task #6)
