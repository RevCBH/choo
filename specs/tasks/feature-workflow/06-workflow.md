---
task: 6
status: pending
backpressure: "go test ./internal/feature/... -run TestWorkflow"
depends_on: [1, 2, 3, 4, 5]
---

# Workflow Orchestration

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #6 of 6 in implementation plan

## Objective

Wire all components together into the main Workflow struct that orchestrates the feature lifecycle state machine.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides `Discovery` for unit discovery
- FEATURE-BRANCH - provides `BranchManager` for branch operations
- SPEC-REVIEW - provides `review.Reviewer` for spec review
- GIT - provides `git.Client` for git operations
- GITHUB - provides `github.Client` for PR operations
- EVENTS - provides `events.Bus` for event emission

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`, `CanTransition`)
- Task #2 must be complete (provides: `CommitSpecs`)
- Task #3 must be complete (provides: `DriftDetector`)
- Task #4 must be complete (provides: `CompletionChecker`)
- Task #5 must be complete (provides: `ReviewCycle`)

### Package Dependencies
- `internal/git` - git operations
- `internal/github` - GitHub PR operations
- `internal/events` - event bus
- `internal/review` - spec review

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── workflow.go    # CREATE: Main workflow orchestration
```

### Types to Implement

```go
// Workflow manages the feature development lifecycle
type Workflow struct {
    prd                 *PRD
    branchMgr           *BranchManager
    reviewer            *review.Reviewer
    discovery           *Discovery
    events              *events.Bus
    git                 *git.Client
    github              *github.Client
    drift               *DriftDetector
    completion          *CompletionChecker
    reviewCycle         *ReviewCycle
    maxReviewIterations int
    status              FeatureStatus
}

// WorkflowConfig holds configuration for the workflow
type WorkflowConfig struct {
    MaxReviewIterations int
    PushRetries         int
    DriftCheckInterval  time.Duration
}

// WorkflowDeps holds external dependencies
type WorkflowDeps struct {
    BranchMgr  *BranchManager
    Reviewer   *review.Reviewer
    Discovery  *Discovery
    Events     *events.Bus
    Git        *git.Client
    GitHub     *github.Client
    Claude     *claude.Client
}
```

### Functions to Implement

```go
// NewWorkflow creates a new feature workflow manager
func NewWorkflow(prd *PRD, cfg WorkflowConfig, deps WorkflowDeps) *Workflow

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

// transitionTo changes state with validation
func (w *Workflow) transitionTo(status FeatureStatus) error

// escalate notifies user of unrecoverable issue
func (w *Workflow) escalate(msg string, err error) error
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestWorkflow
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWorkflow_StartFromPending` | Transitions to `generating_specs` |
| `TestWorkflow_InvalidStartState` | Returns error if not pending |
| `TestWorkflow_FullCycle` | Transitions through all states to `specs_committed` |
| `TestWorkflow_TransitionValidation` | Rejects invalid transitions |
| `TestWorkflow_Resume` | Resumes from `review_blocked` state |
| `TestWorkflow_ResumeInvalidState` | Returns error if not resumable |
| `TestWorkflow_CurrentStatus` | Returns current state |
| `TestWorkflow_Escalation` | Escalation emits event |
| `TestWorkflow_DriftCheckIntegration` | Drift detection pauses work |
| `TestWorkflow_CompletionCheckIntegration` | Completion triggers PR |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `workflow_prd.json` | `internal/feature/testdata/` | Sample PRD for workflow tests |

### CI Compatibility

- [x] No external API keys required (all deps mocked)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- State transitions must complete within 100ms (excluding external operations)
- All state changes emit events via `events.Bus`
- State machine must be persisted to survive process restarts
- Transitions validate against `ValidTransitions` map

State machine flow:
```
pending -> generating_specs -> reviewing_specs <-> updating_specs
                                    |
                              review_blocked (resumable)
                                    |
                              validating_specs -> generating_tasks
                                    |
                              specs_committed -> in_progress
                                    |
                              units_complete -> pr_open -> complete
```

- Any state can transition to `failed` on unrecoverable error
- Only `review_blocked` can resume (back to `reviewing_specs`)

## NOT In Scope

- State definitions (Task #1 - already complete)
- Commit operations (Task #2 - already complete)
- Drift detection (Task #3 - already complete)
- Completion logic (Task #4 - already complete)
- Review cycle (Task #5 - already complete)
