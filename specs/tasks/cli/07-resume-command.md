---
task: 7
status: pending
backpressure: "go test ./internal/cli/... -run Resume"
depends_on: [1, 6]
---

# Resume Command

**Parent spec**: `/specs/CLI.md`
**Task**: #7 of 9 in implementation plan

## Objective

Implement the resume command that continues orchestration from the last saved state.

## Dependencies

### External Specs (must be implemented)
- CONFIG - provides `Config` type
- DISCOVERY - provides `Discovery` type with state loading

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)
- Task #6 must be complete (provides: `RunOptions`, `RunOrchestrator`)

### Package Dependencies
- None beyond standard library and internal packages

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── resume.go    # CREATE: Resume command implementation
```

### Functions to Implement

```go
// NewResumeCmd creates the resume command
func NewResumeCmd(app *App) *cobra.Command {
    // Create command with Use: "resume [tasks-dir]"
    // Inherit run command flags (parallelism, target, etc.)
    // Call ResumeOrchestrator in RunE
}

// ResumeOrchestrator continues from the last saved state
func (a *App) ResumeOrchestrator(ctx context.Context, opts RunOptions) error {
    // Validate options
    // Load existing state from frontmatter
    // Validate state is resumable (not all complete, not corrupted)
    // Continue with RunOrchestrator from saved state
}

// validateResumeState checks if state can be resumed
func validateResumeState(disc *discovery.Discovery) error {
    // Check if there are incomplete units
    // Check if state is consistent
    // Return error if nothing to resume
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Resume -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestResumeCmd_InheritsRunFlags` | Has same flags as run command |
| `TestResumeOrchestrator_NoState` | Returns error when no previous state |
| `TestResumeOrchestrator_AllComplete` | Returns error when all units complete |
| `TestResumeOrchestrator_PartialState` | Resumes from incomplete unit |
| `TestValidateResumeState_Valid` | Accepts state with incomplete units |
| `TestValidateResumeState_Complete` | Rejects fully complete state |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Partial state fixture | testdata/resume/ | Test resume from various states |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Resume reads state from YAML frontmatter in task specs
- State includes:
  - Unit status (pending, in_progress, complete)
  - Task status (pending, in_progress, complete, failed)
  - Current branch name
  - PR number (if created)

- Resume should:
  1. Load discovery with current frontmatter state
  2. Validate there's work to resume
  3. Skip completed tasks
  4. Continue from first incomplete task

- Error messages should be helpful:
  - "Nothing to resume: all units complete"
  - "Nothing to resume: no previous orchestration state found"
  - "Cannot resume: state corrupted (unit X has completed tasks after pending tasks)"

## NOT In Scope

- State repair/recovery
- Interactive state selection
- Resume from specific checkpoint
