---
task: 5
status: pending
backpressure: "go test ./internal/cli/... -run FeatureStatus"
depends_on: [1, 2, 3]
---

# Feature Status Command

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the `choo feature status` command that displays feature workflow status.

## Dependencies

### External Specs (must be implemented)
- None additional

### Task Dependencies (within this unit)
- Task #1 provides: `FeatureStatus`, `FeatureState`
- Task #2 provides: `PRDStore`, `PRDMetadata`
- Task #3 provides: `NewFeatureCmd`

### Package Dependencies
- `github.com/spf13/cobra`
- `encoding/json`

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── feature_status.go    # CREATE: Status command implementation
```

### Types to Implement

```go
// FeatureStatusOptions holds flags for the feature status command
type FeatureStatusOptions struct {
    PRDID string // optional, shows all if empty
    JSON  bool
}

// FeatureStatusOutput represents JSON output format
type FeatureStatusOutput struct {
    Features []FeatureStatusItem `json:"features"`
    Summary  StatusSummary       `json:"summary"`
}

// FeatureStatusItem represents a single feature's status
type FeatureStatusItem struct {
    PRDID            string `json:"prd_id"`
    Status           string `json:"status"`
    Branch           string `json:"branch"`
    StartedAt        string `json:"started_at,omitempty"`
    ReviewIterations int    `json:"review_iterations"`
    MaxReviewIter    int    `json:"max_review_iter"`
    LastFeedback     string `json:"last_feedback,omitempty"`
    SpecCount        int    `json:"spec_count,omitempty"`
    TaskCount        int    `json:"task_count,omitempty"`
    NextAction       string `json:"next_action,omitempty"`
}

// StatusSummary provides aggregate counts
type StatusSummary struct {
    Total      int `json:"total"`
    Ready      int `json:"ready"`
    Blocked    int `json:"blocked"`
    InProgress int `json:"in_progress"`
}
```

### Functions to Implement

```go
// NewFeatureStatusCmd creates the feature status command
func NewFeatureStatusCmd(app *App) *cobra.Command {
    // Create command with:
    //   Use: "status [prd-id]"
    //   Short: "Show status of feature workflows"
    //   Args: cobra.MaximumNArgs(1)
    // Add flags:
    //   --json (default: false)
}

// ShowFeatureStatus displays feature workflow status
func (a *App) ShowFeatureStatus(opts FeatureStatusOptions) error {
    // Load features from PRD directory
    // If specific PRDID provided, filter to that one
    // If JSON flag, output as JSON
    // Otherwise, render formatted output
}

// loadFeatures scans PRD directory for feature state
func (a *App) loadFeatures(prdDir string) ([]FeatureState, error) {
    // List all .md files in prdDir
    // Parse each for feature state
    // Filter to only those with feature_status set
    // Return sorted list
}

// renderFeatureStatus outputs formatted status display
func (a *App) renderFeatureStatus(features []FeatureState) {
    // Print header with box drawing characters
    // For each feature:
    //   Print [prd-id] status
    //   Print branch, started time, review iterations
    //   Based on status:
    //     - specs_committed: print spec/task counts, ready command
    //     - review_blocked: print feedback, action needed, resume command
    //     - others: print current step
    // Print summary footer
}

// determineNextAction returns the actionable next step for a status
func determineNextAction(state FeatureState) string {
    // Return appropriate action based on status
}
```

### Command Structure

```
choo feature status [prd-id]

Show status of feature workflows.

Arguments:
  prd-id    Specific PRD to check (optional, shows all if omitted)

Flags:
  --json    Output as JSON

Examples:
  choo feature status                    # Show all in-progress features
  choo feature status streaming-events   # Show specific feature
  choo feature status --json             # Output as JSON
```

### Expected Output Format

```
===============================================================
Feature Workflows
===============================================================

 [streaming-events] specs_committed
   Branch: feature/streaming-events
   Started: 2026-01-20 14:30:00
   Review iterations: 2/3
   Specs: 4 units, 18 tasks
   Ready for: choo run --feature streaming-events

 [multi-repo] review_blocked
   Branch: feature/multi-repo
   Started: 2026-01-20 10:15:00
   Review iterations: 3/3 (exhausted)
   Last feedback: "Missing error handling in merger spec"
   Action: Manual intervention required
   Resume with: choo feature resume multi-repo --skip-review

---------------------------------------------------------------
 Features: 2 | Ready: 1 | Blocked: 1 | In Progress: 0
===============================================================
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run FeatureStatus
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestFeatureStatusCmd_NoArgs` | Shows all features when no arg provided |
| `TestFeatureStatusCmd_WithPRDID` | Shows only specified feature |
| `TestFeatureStatusCmd_JSON` | Outputs valid JSON |
| `TestShowFeatureStatus_NoFeatures` | Handles empty feature list gracefully |
| `TestShowFeatureStatus_Blocked` | Shows resume instructions for blocked |
| `TestShowFeatureStatus_Ready` | Shows run command for ready features |
| `TestDetermineNextAction` | Returns correct action for each status |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `in-progress.md` | `internal/cli/testdata/prds/` | PRD with generating_specs status |
| `blocked.md` | `internal/cli/testdata/prds/` | PRD with review_blocked status |
| `ready.md` | `internal/cli/testdata/prds/` | PRD with specs_committed status |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use unicode box drawing characters for borders (or ASCII fallback)
- JSON output should match the FeatureStatusOutput structure exactly
- Sort features by started_at (newest first) for display
- For review_blocked, always show the last_feedback and resume command
- Time formatting should use "2006-01-02 15:04:05" layout

## NOT In Scope

- Workflow execution (FEATURE-WORKFLOW spec)
- Resume functionality (task #6)
- Start functionality (task #4)
