---
task: 4
status: pending
backpressure: "go test ./internal/cli/... -run FeatureStart"
depends_on: [1, 2, 3]
---

# Feature Start Command

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the `choo feature start` command that initiates the feature workflow from a PRD.

## Dependencies

### External Specs (must be implemented)
- FEATURE-WORKFLOW provides: `Workflow`, `StartOptions`, workflow execution

### Task Dependencies (within this unit)
- Task #1 provides: `FeatureStatus`, `FeatureState`
- Task #2 provides: `PRDStore`, `PRDMetadata`
- Task #3 provides: `NewFeatureCmd`

### Package Dependencies
- `github.com/spf13/cobra`

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── feature_start.go    # CREATE: Start command implementation
```

### Types to Implement

```go
// FeatureStartOptions holds flags for the feature start command
type FeatureStartOptions struct {
    PRDID          string
    PRDDir         string
    SpecsDir       string
    SkipSpecReview bool
    MaxReviewIter  int
    DryRun         bool
}
```

### Functions to Implement

```go
// NewFeatureStartCmd creates the feature start command
func NewFeatureStartCmd(app *App) *cobra.Command {
    // Create command with:
    //   Use: "start <prd-id>"
    //   Short: "Create feature branch, generate specs with review, generate tasks"
    //   Args: cobra.ExactArgs(1)
    // Add flags:
    //   --prd-dir (default: "docs/prds")
    //   --specs-dir (default: "specs/tasks")
    //   --skip-spec-review (default: false)
    //   --max-review-iter (default: 3)
    //   --dry-run (default: false)
    // Set RunE to execute workflow
}

// RunFeatureStart executes the feature start workflow
func (a *App) RunFeatureStart(ctx context.Context, opts FeatureStartOptions) error {
    // If dry-run, call runDryRun and return
    // Validate PRD exists
    // Initialize workflow from FEATURE-WORKFLOW
    // Execute workflow.Start()
    // Handle blocked states (print resume instructions)
    // Handle success (print completion message)
}

// runDryRun prints planned actions without executing
func (a *App) runDryRun(opts FeatureStartOptions) error {
    // Print numbered list of planned actions:
    // 1. Read PRD
    // 2. Create branch
    // 3. Generate specs
    // 4. Review specs (if not --skip-spec-review)
    // 5. Validate specs
    // 6. Generate tasks
    // 7. Commit to branch
}
```

### Command Structure

```
choo feature start <prd-id> [flags]

Create feature branch, generate specs with review, generate tasks, commit to branch.

Arguments:
  prd-id    The PRD ID to start (e.g., "streaming-events")

Flags:
  --prd-dir string        PRDs directory (default: docs/prds)
  --specs-dir string      Output specs directory (default: specs/tasks)
  --skip-spec-review      Skip automated spec review loop
  --max-review-iter int   Max spec review iterations (default: 3)
  --dry-run               Show plan without executing

Examples:
  choo feature start streaming-events
  choo feature start streaming-events --skip-spec-review
  choo feature start streaming-events --dry-run
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run FeatureStart
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestFeatureStartCmd_Args` | Requires exactly 1 argument |
| `TestFeatureStartCmd_Flags` | All flags registered with correct defaults |
| `TestFeatureStartCmd_DryRun` | Dry run prints plan without executing |
| `TestRunFeatureStart_PRDNotFound` | Returns error when PRD doesn't exist |
| `TestRunFeatureStart_ValidatesInput` | Validates PRD ID format |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `valid-prd.md` | `internal/cli/testdata/prds/` | Valid PRD for start tests |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses mocked workflow)
- [x] Runs in <60 seconds

## Implementation Notes

- PRD ID is the filename without `.md` extension (e.g., "streaming-events")
- Dry-run should print a clear numbered list of planned actions
- On blocked state, print the specific resume command the user should run
- Workflow initialization depends on FEATURE-WORKFLOW spec being implemented
- For testing, mock the workflow executor

## NOT In Scope

- Actual workflow execution logic (FEATURE-WORKFLOW spec)
- Status display (task #5)
- Resume functionality (task #6)
- Git operations (FEATURE-WORKFLOW spec)
