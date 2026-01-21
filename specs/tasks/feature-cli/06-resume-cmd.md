---
task: 6
status: complete
backpressure: "go test ./internal/cli/... -run FeatureResume"
depends_on: [1, 2, 3, 4]
---

# Feature Resume Command

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement the `choo feature resume` command that continues a blocked feature workflow.

## Dependencies

### External Specs (must be implemented)
- FEATURE-WORKFLOW provides: `Workflow`, `ResumeOptions`, resume execution

### Task Dependencies (within this unit)
- Task #1 provides: `FeatureStatus`, `FeatureState`, `StatusReviewBlocked`
- Task #2 provides: `PRDStore`, `PRDMetadata`
- Task #3 provides: `NewFeatureCmd`
- Task #4 provides: Command pattern and workflow integration

### Package Dependencies
- `github.com/spf13/cobra`

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── feature_resume.go    # CREATE: Resume command implementation
```

### Types to Implement

```go
// FeatureResumeOptions holds flags for the feature resume command
type FeatureResumeOptions struct {
    PRDID          string
    SkipReview     bool
    FromValidation bool
    FromTasks      bool
}
```

### Functions to Implement

```go
// NewFeatureResumeCmd creates the feature resume command
func NewFeatureResumeCmd(app *App) *cobra.Command {
    // Create command with:
    //   Use: "resume <prd-id>"
    //   Short: "Resume a blocked feature workflow"
    //   Args: cobra.ExactArgs(1)
    // Add flags:
    //   --skip-review (default: false)
    //   --from-validation (default: false)
    //   --from-tasks (default: false)
}

// RunFeatureResume continues a blocked feature workflow
func (a *App) RunFeatureResume(ctx context.Context, opts FeatureResumeOptions) error {
    // Validate PRD exists
    // Load current state
    // Validate state is resumable (review_blocked)
    // Validate flag combinations (mutually exclusive options)
    // Initialize workflow from FEATURE-WORKFLOW
    // Execute workflow.Resume() with options
    // Handle blocked states (print next steps)
    // Handle success (print completion message)
}

// validateResumeState checks if feature can be resumed
func validateResumeState(state FeatureState) error {
    // Only StatusReviewBlocked can be resumed
    // Return descriptive error for other states
}

// validateResumeOptions checks flag combinations
func validateResumeOptions(opts FeatureResumeOptions) error {
    // --from-validation and --from-tasks are mutually exclusive
    // --skip-review can combine with others
}
```

### Command Structure

```
choo feature resume <prd-id> [flags]

Resume a feature workflow from a blocked state.

Arguments:
  prd-id    The PRD ID to resume

Flags:
  --skip-review           Skip remaining review iterations and proceed to validation
  --from-validation       Resume from spec validation (after manual spec edits)
  --from-tasks            Resume from task generation (after manual spec edits)

Examples:
  choo feature resume streaming-events
  choo feature resume streaming-events --skip-review
  choo feature resume streaming-events --from-validation
```

### Error Messages

```
Error: cannot resume feature "streaming-events"
  Current status: generating_specs
  Only features in "review_blocked" state can be resumed.

Error: cannot resume feature "streaming-events"
  Current status: specs_committed
  This feature has already completed spec generation.
  Use "choo run --feature streaming-events" to execute units.

Error: invalid flag combination
  --from-validation and --from-tasks are mutually exclusive
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run FeatureResume
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestFeatureResumeCmd_Args` | Requires exactly 1 argument |
| `TestFeatureResumeCmd_Flags` | All flags registered correctly |
| `TestRunFeatureResume_NotBlocked` | Returns error for non-blocked state |
| `TestRunFeatureResume_PRDNotFound` | Returns error when PRD doesn't exist |
| `TestValidateResumeState_Blocked` | Allows resume from review_blocked |
| `TestValidateResumeState_Other` | Rejects resume from other states |
| `TestValidateResumeOptions_MutuallyExclusive` | Rejects conflicting flags |
| `TestRunFeatureResume_SkipReview` | Skips to validation when flag set |
| `TestRunFeatureResume_FromValidation` | Resumes from validation step |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `blocked.md` | `internal/cli/testdata/prds/` | PRD in review_blocked state |
| `in-progress.md` | `internal/cli/testdata/prds/` | PRD in non-resumable state |
| `ready.md` | `internal/cli/testdata/prds/` | PRD in completed state |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses mocked workflow)
- [x] Runs in <60 seconds

## Implementation Notes

- Resume is only valid from `review_blocked` state
- `--skip-review` bypasses remaining review iterations, goes to validation
- `--from-validation` assumes user manually edited specs, re-validates
- `--from-tasks` assumes validation passed, regenerates tasks only
- Print clear error messages with current state and valid next steps
- On successful resume, continue workflow to completion or next block

## NOT In Scope

- Actual workflow resume logic (FEATURE-WORKFLOW spec)
- Status display (task #5)
- Start functionality (task #4)
- State transition logic (FEATURE-WORKFLOW spec)
