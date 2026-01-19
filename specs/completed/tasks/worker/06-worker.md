---
task: 6
status: complete
backpressure: "go test ./internal/worker/... -run Worker"
depends_on: [1, 4, 5]
---

# Single Unit Worker

**Parent spec**: `/specs/WORKER.md`
**Task**: #6 of 8 in implementation plan

## Objective

Implement the complete Worker.Run() method that orchestrates all phases: worktree setup, task loop, baseline checks, and PR creation.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Unit`, parsing functions
- EVENTS - provides `Bus`, all event types
- GIT - provides `WorktreeManager`
- GITHUB - provides `PRClient`

### Task Dependencies (within this unit)
- Task #1 (types-config) - provides `Worker`, `WorkerConfig`, `WorkerDeps`
- Task #4 (baseline) - provides `RunBaselineChecks`
- Task #5 (loop) - provides `runTaskLoop`, `findReadyTasks`

### Package Dependencies
- `crypto/sha256` - for branch name generation
- `encoding/hex` - for hash encoding
- `fmt` - for formatting

## Deliverables

### Files to Modify

```
internal/worker/
└── worker.go       # Add Run() and helper methods
```

### Functions to Implement

```go
// Run executes the unit through all phases: setup, task loop, baseline, PR
func (w *Worker) Run(ctx context.Context) error {
    // Phase 1: Setup
    // 1. Generate branch name
    // 2. Create worktree via git.WorktreeManager
    // 3. Update unit frontmatter: orch_status=in_progress
    // 4. Emit UnitStarted event

    // Phase 2: Task Loop
    // 5. Call runTaskLoop()
    // 6. If error, emit UnitFailed and cleanup

    // Phase 2.5: Baseline Checks
    // 7. Run baseline checks
    // 8. If fails, invoke Claude with baseline-fix prompt
    // 9. Retry up to MaxBaselineRetries
    // 10. If still fails, emit UnitFailed

    // Phase 3: PR Creation
    // 11. Push branch to remote
    // 12. Create PR via github.PRClient
    // 13. Update unit frontmatter: orch_status=pr_open
    // 14. Emit PRCreated event

    // Phase 4: Cleanup
    // 15. Remove worktree
    // 16. Emit UnitCompleted event
}

// generateBranchName creates a unique branch name for the unit
func (w *Worker) generateBranchName() string {
    // Format: ralph/<unit-id>-<short-hash>
    // Hash includes unit ID and timestamp for uniqueness
}

// setupWorktree creates the isolated worktree for this worker
func (w *Worker) setupWorktree(ctx context.Context) error {
    // 1. Generate branch name
    // 2. Create worktree at config.WorktreeBase/<unit-id>
    // 3. Store worktreePath and branch in worker state
}

// runBaselinePhase executes baseline checks with retry loop
func (w *Worker) runBaselinePhase(ctx context.Context) error {
    // 1. Run baseline checks
    // 2. If pass, return nil
    // 3. If fail:
    //    a. Build baseline fix prompt with failure output
    //    b. Invoke Claude to fix
    //    c. Commit fixes with --no-verify
    //    d. Re-run baseline checks
    //    e. Repeat up to MaxBaselineRetries
    // 4. Return error if still failing
}

// createPR pushes branch and creates pull request
func (w *Worker) createPR(ctx context.Context) error {
    // 1. Push branch via git.WorktreeManager
    // 2. Create PR via github.PRClient
    // 3. Emit PRCreated event
    // Note: Skip if config.NoPR is true
}

// cleanup removes the worktree
func (w *Worker) cleanup(ctx context.Context) error {
    // 1. Remove worktree via git.WorktreeManager
    // 2. Delete local branch if merged
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Worker
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWorker_Run_HappyPath` | All phases complete, events emitted in order |
| `TestWorker_Run_TaskLoopFails` | UnitFailed event emitted, cleanup called |
| `TestWorker_Run_BaselineFails` | Retries baseline fix, eventually fails |
| `TestWorker_Run_NoPR` | Skips PR creation when config.NoPR=true |
| `TestGenerateBranchName` | Format matches `ralph/<unit-id>-<hash>` |
| `TestSetupWorktree` | Worktree created, state updated |
| `TestCleanup` | Worktree removed |

### Test Implementation

```go
func TestWorker_Run_HappyPath(t *testing.T) {
    // Setup mock dependencies
    events := &mockEventBus{}
    git := &mockGitManager{}
    github := &mockGitHub{}
    claude := &mockClaude{}

    unit := &discovery.Unit{
        ID: "test-unit",
        Tasks: []*discovery.Task{
            {Number: 1, Status: discovery.TaskStatusComplete},
        },
    }

    w := NewWorker(unit, WorkerConfig{
        WorktreeBase: t.TempDir(),
    }, WorkerDeps{
        Events: events,
        Git:    git,
        GitHub: github,
        Claude: claude,
    })

    err := w.Run(context.Background())

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    // Verify event sequence
    expectedEvents := []events.EventType{
        events.UnitStarted,
        events.PRCreated,
        events.UnitCompleted,
    }
    // ... verify events emitted in order
}

func TestWorker_Run_NoPR(t *testing.T) {
    // Similar setup with config.NoPR = true
    // Verify PRCreated event NOT emitted
    // Verify github.CreatePR NOT called
}

func TestGenerateBranchName(t *testing.T) {
    unit := &discovery.Unit{ID: "my-unit"}
    w := &Worker{unit: unit}

    branch := w.generateBranchName()

    if !strings.HasPrefix(branch, "ralph/my-unit-") {
        t.Errorf("expected branch to start with 'ralph/my-unit-', got %q", branch)
    }
    // Should have 6 hex chars after the dash
    parts := strings.Split(branch, "-")
    if len(parts[len(parts)-1]) != 6 {
        t.Error("expected 6 char hash suffix")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses mocks)
- [x] Runs in <60 seconds

## Implementation Notes

- Use defer for cleanup to ensure worktree removal even on error
- Emit events at all key state transitions per the spec
- Branch name hash uses SHA256 of unit ID + timestamp, truncated to 6 hex chars
- The baseline fix loop should emit events for each retry attempt
- Handle context cancellation gracefully in all phases

## NOT In Scope

- Pool management (task #7)
- Execute() entry point (task #8)
- PR review handling (separate spec)
