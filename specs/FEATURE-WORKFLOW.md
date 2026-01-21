# FEATURE-WORKFLOW — Feature Workflow State Machine, Commit Specs, Auto-Completion, and Drift Detection

## Overview

The feature workflow manages the complete lifecycle of feature development from PRD intake through final PR merge. It implements a state machine that orchestrates spec generation, review cycles, task breakdown, spec commits, unit execution, and automatic feature completion. The workflow includes drift detection to handle PRD changes during development.

Key capabilities:
- State machine with well-defined transitions and failure handling
- Automatic spec commits to feature branch before unit execution
- Auto-triggered feature completion when all units merge
- Drift detection to assess impact of PRD changes on in-progress work

## Requirements

### Functional Requirements

1. **State Machine Management**
   - Track feature through all states: `pending`, `generating_specs`, `reviewing_specs`, `updating_specs`, `review_blocked`, `validating_specs`, `generating_tasks`, `specs_committed`, `in_progress`, `units_complete`, `pr_open`, `complete`, `failed`
   - Enforce valid state transitions only
   - Support resume from `review_blocked` state
   - Transition to `failed` on unrecoverable errors with escalation

2. **Spec Review Cycle**
   - Loop between `reviewing_specs` and `updating_specs` until pass or max iterations
   - Transition to `review_blocked` when max iterations reached or malformed output
   - Allow manual resume after user intervention

3. **Spec Commit Step**
   - Stage all generated spec files in `specs/tasks/<prd-id>/`
   - Commit with standardized message (no Claude invocation)
   - Push to remote feature branch
   - Retry push once on failure before transitioning to `failed`

4. **Auto-Triggered Completion**
   - Detect when all units have merged PRs
   - Verify feature branch is clean (no uncommitted changes)
   - Create PR from feature branch to main automatically
   - Idempotent: skip if PR already exists or feature already complete

5. **Drift Detection**
   - Detect PRD body changes during development
   - Pause unit work when drift detected
   - Assess impact on in-progress units via Claude
   - Escalate significant drift to user with affected units list

### Performance Requirements

1. State transitions must complete within 100ms (excluding external operations)
2. Drift detection check must run in under 50ms for hash comparison
3. Completion checks should not block unit execution

### Constraints

1. State machine must be persisted to survive process restarts
2. All git operations must be atomic where possible
3. No Claude invocation for commit messages (deterministic commits)
4. Feature PR creation must be idempotent

## Design

### Module Structure

```
internal/feature/
├── workflow.go      # Main state machine and transitions
├── commit.go        # Spec commit operations
├── drift.go         # Drift detection and assessment
├── completion.go    # Auto-triggered completion logic
└── states.go        # State definitions and validation
```

### Core Types

```go
// internal/feature/states.go

// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
    StatusPending         FeatureStatus = "pending"
    StatusGeneratingSpecs FeatureStatus = "generating_specs"
    StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
    StatusUpdatingSpecs   FeatureStatus = "updating_specs"
    StatusReviewBlocked   FeatureStatus = "review_blocked"
    StatusValidatingSpecs FeatureStatus = "validating_specs"
    StatusGeneratingTasks FeatureStatus = "generating_tasks"
    StatusSpecsCommitted  FeatureStatus = "specs_committed"
    StatusInProgress      FeatureStatus = "in_progress"
    StatusUnitsComplete   FeatureStatus = "units_complete"
    StatusPROpen          FeatureStatus = "pr_open"
    StatusComplete        FeatureStatus = "complete"
    StatusFailed          FeatureStatus = "failed"
)

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
    StatusPending:         {StatusGeneratingSpecs, StatusFailed},
    StatusGeneratingSpecs: {StatusReviewingSpecs, StatusFailed},
    StatusReviewingSpecs:  {StatusUpdatingSpecs, StatusReviewBlocked, StatusValidatingSpecs, StatusFailed},
    StatusUpdatingSpecs:   {StatusReviewingSpecs, StatusFailed},
    StatusReviewBlocked:   {StatusReviewingSpecs, StatusFailed},
    StatusValidatingSpecs: {StatusGeneratingTasks, StatusFailed},
    StatusGeneratingTasks: {StatusSpecsCommitted, StatusFailed},
    StatusSpecsCommitted:  {StatusInProgress, StatusFailed},
    StatusInProgress:      {StatusUnitsComplete, StatusFailed},
    StatusUnitsComplete:   {StatusPROpen, StatusFailed},
    StatusPROpen:          {StatusComplete, StatusFailed},
    StatusComplete:        {},
    StatusFailed:          {},
}

// CanTransition checks if a state transition is valid
func CanTransition(from, to FeatureStatus) bool {
    allowed, ok := ValidTransitions[from]
    if !ok {
        return false
    }
    for _, s := range allowed {
        if s == to {
            return true
        }
    }
    return false
}
```

```go
// internal/feature/workflow.go

// Workflow manages the feature development lifecycle
type Workflow struct {
    prd       *PRD
    branchMgr *BranchManager
    reviewer  *review.Reviewer
    discovery *Discovery
    events    *events.Bus
    git       *git.Client
    github    *github.Client
    drift     *DriftDetector
    maxReviewIterations int
}

// WorkflowConfig holds configuration for the workflow
type WorkflowConfig struct {
    MaxReviewIterations int
    PushRetries         int
    DriftCheckInterval  time.Duration
}

// ResumeOptions configures how to resume from blocked state
type ResumeOptions struct {
    SkipReview bool   // Skip directly to validation
    Message    string // User-provided context for resume
}
```

```go
// internal/feature/commit.go

// CommitResult holds the result of the commit operation
type CommitResult struct {
    CommitHash string
    FileCount  int
    Pushed     bool
}

// CommitOptions configures the commit operation
type CommitOptions struct {
    PushRetries int
    DryRun      bool
}
```

```go
// internal/feature/drift.go

// DriftDetector monitors PRD changes and assesses impact
type DriftDetector struct {
    prd          *PRD
    lastBodyHash string
    claude       *claude.Client
}

// DriftResult contains the assessment of PRD changes
type DriftResult struct {
    HasDrift       bool
    Significant    bool
    Changes        string   // Diff summary
    AffectedUnits  []string
    Recommendation string
}
```

```go
// internal/feature/completion.go

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

### API Surface

```go
// internal/feature/workflow.go

// NewWorkflow creates a new feature workflow manager
func NewWorkflow(cfg WorkflowConfig, deps WorkflowDeps) *Workflow

// Start initiates the feature workflow from pending state
func (w *Workflow) Start(ctx context.Context) error

// GenerateSpecs transitions to spec generation
func (w *Workflow) GenerateSpecs(ctx context.Context) error

// ReviewSpecs runs the spec review cycle
func (w *Workflow) ReviewSpecs(ctx context.Context) error

// ValidateSpecs validates specs before task generation
func (w *Workflow) ValidateSpecs(ctx context.Context) error

// GenerateTasks generates implementation tasks from specs
func (w *Workflow) GenerateTasks(ctx context.Context) error

// CommitSpecs commits generated specs to feature branch
func (w *Workflow) CommitSpecs(ctx context.Context) error

// Resume continues workflow from review_blocked state
func (w *Workflow) Resume(ctx context.Context, opts ResumeOptions) error

// CurrentStatus returns the current workflow state
func (w *Workflow) CurrentStatus() FeatureStatus

// CanResume returns true if workflow can be resumed
func (w *Workflow) CanResume() bool
```

```go
// internal/feature/commit.go

// CommitSpecs stages and commits generated specs to the feature branch
func CommitSpecs(ctx context.Context, git *git.Client, prdID string) (*CommitResult, error)

// CommitSpecsWithOptions commits with custom options
func CommitSpecsWithOptions(ctx context.Context, git *git.Client, prdID string, opts CommitOptions) (*CommitResult, error)
```

```go
// internal/feature/drift.go

// NewDriftDetector creates a drift detector for the given PRD
func NewDriftDetector(prd *PRD, claude *claude.Client) *DriftDetector

// CheckDrift compares current PRD body to last known state
func (d *DriftDetector) CheckDrift(ctx context.Context) (*DriftResult, error)

// UpdateBaseline sets the current PRD body as the new baseline
func (d *DriftDetector) UpdateBaseline()
```

```go
// internal/feature/completion.go

// NewCompletionChecker creates a completion checker for the feature
func NewCompletionChecker(prd *PRD, git *git.Client, gh *github.Client, bus *events.Bus) *CompletionChecker

// Check evaluates completion conditions and returns status
func (c *CompletionChecker) Check(ctx context.Context) (*CompletionStatus, error)

// TriggerCompletion creates the feature PR if conditions are met
func (c *CompletionChecker) TriggerCompletion(ctx context.Context) error
```

### State Machine

```
┌─────────┐
│ pending │ ─────────────────────────────────────────┐
└────┬────┘                                          │
     │ choo feature start                            │
     ▼                                               │
┌────────────┐                                       │
│ generating │                                       │
│   specs    │                                       │
└─────┬──────┘                                       │
      │                                              │
      ▼                                              │
┌────────────┐     needs_revision     ┌──────────┐  │
│  reviewing │ ◀─────────────────────▶│ updating │  │
│   specs    │      (iter < max)      │  specs   │  │
└─────┬──────┘                        └──────────┘  │
      │                                             │
      │ pass                                        │
      │                                             │
      │ iter >= max OR malformed output             │
      ├────────────────────────────────────────┐    │
      │                                        ▼    │
      │                               ┌─────────────┐
      │                               │   review_   │
      │                               │   blocked   │
      │                               └──────┬──────┘
      │                                      │
      │              choo feature resume     │
      │◀─────────────────────────────────────┘
      │
      ▼
┌────────────┐
│ validating │
│   specs    │
└─────┬──────┘
      │
      ▼
┌────────────┐
│ generating │
│   tasks    │
└─────┬──────┘
      │
      ▼
┌────────────┐
│   specs_   │  ◀── commit specs/tasks to feature branch
│ committed  │
└─────┬──────┘
      │
      │ choo run --feature
      ▼
┌────────────┐
│in_progress │  ◀── orchestrator runs units
│  (units)   │
└─────┬──────┘
      │ all units merged + branch clean
      ▼
┌──────────────┐
│units_complete│
└──────┬───────┘
       │ auto-triggered (idempotent)
       ▼
┌────────────┐
│  pr_open   │  PR: feature branch → main
└─────┬──────┘
      │ merged
      ▼
┌────────────┐
│  complete  │
└────────────┘

┌────────────┐ ◀────────────────────────────────────┘
│   failed   │   (any failure with escalation)
└────────────┘
```

**Transition Rules:**

| From | To | Trigger |
|------|-----|---------|
| `pending` | `generating_specs` | `choo feature start` command |
| `generating_specs` | `reviewing_specs` | Specs generated successfully |
| `reviewing_specs` | `updating_specs` | Review returns `needs_revision` |
| `reviewing_specs` | `validating_specs` | Review returns `pass` |
| `reviewing_specs` | `review_blocked` | Max iterations or malformed output |
| `updating_specs` | `reviewing_specs` | Updates complete |
| `review_blocked` | `reviewing_specs` | `choo feature resume` command |
| `validating_specs` | `generating_tasks` | Validation passes |
| `generating_tasks` | `specs_committed` | Tasks generated, specs committed |
| `specs_committed` | `in_progress` | `choo run --feature` command |
| `in_progress` | `units_complete` | All units have merged PRs |
| `units_complete` | `pr_open` | Feature PR created (auto-triggered) |
| `pr_open` | `complete` | Feature PR merged |
| Any | `failed` | Unrecoverable error with escalation |

## Implementation Notes

### Spec Commit Step

The commit step must execute after task generation completes and before transitioning to `specs_committed`:

```go
func (w *Workflow) commitSpecs(ctx context.Context) error {
    specsDir := filepath.Join("specs/tasks", w.prd.ID)

    // Stage all generated spec files
    if err := w.git.Add(specsDir); err != nil {
        return fmt.Errorf("failed to stage specs: %w", err)
    }

    // Commit with standardized message (no Claude invocation)
    commitMsg := fmt.Sprintf("chore(feature): add specs for %s", w.prd.ID)
    if err := w.git.Commit(commitMsg); err != nil {
        return fmt.Errorf("failed to commit specs: %w", err)
    }

    // Push to remote feature branch with retry
    var pushErr error
    for i := 0; i <= 1; i++ { // One retry
        if pushErr = w.git.Push(); pushErr == nil {
            return nil
        }
    }
    return fmt.Errorf("failed to push specs after retry: %w", pushErr)
}
```

**Failure Handling:**
- Staging failure: Transition to `failed`, emit notification with error details
- Commit failure: Transition to `failed`, emit notification (likely nothing staged)
- Push failure: Retry once, then transition to `failed` with notification

### Auto-Triggered Completion

The orchestrator checks completion conditions after each unit merge:

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

**Trigger Conditions (all must be true):**
1. All units in `specs/tasks/<feature>/` have merged PRs
2. Feature branch is clean (no uncommitted changes)
3. No pending or in-progress units remain

**Idempotency Checks:**
- If PR already exists: Update state to `pr_open`, log, and skip creation
- If state is `pr_open` or `complete`: No-op with log message

### Drift Detection

Drift detection runs when PRD body changes are detected:

```go
func (d *DriftDetector) CheckDrift(ctx context.Context) (*DriftResult, error) {
    currentHash := hashBody(d.prd.Body)
    if currentHash == d.lastBodyHash {
        return &DriftResult{HasDrift: false}, nil
    }

    // Compute diff
    diff := computeDiff(d.lastBody, d.prd.Body)

    // Invoke Claude to assess impact
    assessment, err := d.claude.AssessDrift(ctx, AssessDriftRequest{
        OriginalBody: d.lastBody,
        NewBody:      d.prd.Body,
        Diff:         diff,
        InProgressUnits: d.getInProgressUnits(),
    })
    if err != nil {
        return nil, fmt.Errorf("drift assessment failed: %w", err)
    }

    return &DriftResult{
        HasDrift:       true,
        Significant:    assessment.Significant,
        Changes:        diff,
        AffectedUnits:  assessment.AffectedUnits,
        Recommendation: assessment.Recommendation,
    }, nil
}
```

**Drift Assessment Flow:**
1. Pause unit work
2. Diff PRD body against last-seen version
3. Invoke Claude to assess impact on in-progress units
4. If significant: Escalate to user with affected units list
5. If minor: Log and continue, update baseline

### Review Cycle Management

The review cycle tracks iterations and handles blocking:

```go
func (w *Workflow) runReviewCycle(ctx context.Context) error {
    for iteration := 0; iteration < w.maxReviewIterations; iteration++ {
        result, err := w.reviewer.Review(ctx, w.prd.Specs)
        if err != nil {
            if isMalformedOutput(err) {
                w.transitionTo(StatusReviewBlocked)
                return w.escalate("Review produced malformed output", err)
            }
            return err
        }

        switch result.Verdict {
        case review.Pass:
            return w.transitionTo(StatusValidatingSpecs)
        case review.NeedsRevision:
            w.transitionTo(StatusUpdatingSpecs)
            if err := w.updateSpecs(ctx, result.Feedback); err != nil {
                return err
            }
            w.transitionTo(StatusReviewingSpecs)
        }
    }

    // Max iterations reached
    w.transitionTo(StatusReviewBlocked)
    return w.escalate("Max review iterations reached", nil)
}
```

## Testing Strategy

### Unit Tests

1. **State Machine Tests** (`workflow_test.go`)
   - Test all valid state transitions
   - Test rejection of invalid transitions
   - Test CanTransition helper function
   - Test state persistence across restarts

2. **Commit Tests** (`commit_test.go`)
   - Test successful commit flow
   - Test staging failure handling
   - Test commit failure handling
   - Test push retry logic
   - Test dry-run mode

3. **Drift Detection Tests** (`drift_test.go`)
   - Test no-drift case (identical body)
   - Test drift detection with changes
   - Test significant vs minor drift classification
   - Test baseline update

4. **Completion Tests** (`completion_test.go`)
   - Test completion condition checks
   - Test idempotency (existing PR)
   - Test idempotency (already complete)
   - Test dirty branch handling

### Integration Tests

1. **Full Workflow Test**
   - Start from pending, complete through specs_committed
   - Verify all state transitions emit events
   - Verify specs are committed to correct branch

2. **Review Cycle Test**
   - Test passing on first review
   - Test multiple revision cycles
   - Test max iterations blocking
   - Test resume from blocked state

3. **Completion Integration Test**
   - Mock all units as merged
   - Verify PR creation
   - Verify state transitions

### Mock Requirements

- `git.Client`: For commit/push operations
- `github.Client`: For PR creation and detection
- `claude.Client`: For drift assessment
- `events.Bus`: For event emission verification

## Design Decisions

### DD1: Deterministic Commit Messages

**Decision**: Commit messages are generated without Claude invocation.

**Rationale**: Commit messages for spec commits should be predictable and consistent. Using Claude would introduce variability and latency for minimal benefit. The standardized format `chore(feature): add specs for <prd-id>` provides sufficient context.

**Trade-offs**: Less descriptive messages, but consistent and fast.

### DD2: Single Push Retry

**Decision**: Push failures retry exactly once before failing.

**Rationale**: Network issues are common but usually transient. A single retry handles most transient failures without excessive delay. More retries would delay error reporting for persistent issues.

**Trade-offs**: May fail on slow networks, but escalates quickly on persistent issues.

### DD3: Idempotent Completion Checks

**Decision**: Completion triggers are fully idempotent with existing PR detection.

**Rationale**: The orchestrator may check completion multiple times (after each unit merge, on restart). Creating duplicate PRs would be confusing. Detecting existing PRs and updating state ensures consistency.

**Trade-offs**: Additional API call to check for existing PR, but prevents duplicates.

### DD4: Pause-First Drift Response

**Decision**: Drift detection pauses unit work before assessment.

**Rationale**: Continuing work while assessing drift could waste effort on outdated requirements. Pausing ensures we don't compound the problem. Assessment is fast enough that the pause is minimal.

**Trade-offs**: May pause unnecessarily for minor changes, but prevents wasted work.

### DD5: Review Blocked State

**Decision**: Max iterations and malformed output trigger `review_blocked` rather than `failed`.

**Rationale**: These conditions are recoverable with user intervention. The user can modify specs manually and resume. Using `failed` would require restarting the entire workflow.

**Trade-offs**: Requires user action, but preserves progress.

## Future Enhancements

1. **Partial Rollback**: Allow rolling back to a previous spec version when drift is significant
2. **Parallel Drift Assessment**: Assess drift impact on multiple units concurrently
3. **Smart Completion Timing**: Batch completion checks to reduce API calls
4. **Commit Squashing**: Option to squash spec commits into a single commit
5. **Branch Protection Awareness**: Handle protected branch requirements for feature PR

## References

- PRD §9: Feature Workflow State Machine
- PRD §5.3: Commit Specs to Feature Branch
- PRD §5.4: Auto-Triggered Feature Completion
- PRD §2.3: Drift Detection
- PRD §10: Implementation Phases 5, 7, 8
- [ORCHESTRATOR.md](./ORCHESTRATOR.md): Orchestrator integration
- [EVENTS.md](./EVENTS.md): Event emission patterns
