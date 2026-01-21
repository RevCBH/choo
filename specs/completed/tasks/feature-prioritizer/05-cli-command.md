---
task: 5
status: complete
backpressure: "go test ./internal/cli/... -run TestNextFeature"
depends_on: [1, 3, 4]
---

# CLI Command

**Parent spec**: `/specs/FEATURE-PRIORITIZER.md`
**Task**: #5 of 5 in implementation plan

## Objective

Implement the `choo next-feature` CLI command with flags for explanation mode, top N filtering, and JSON output.

## Dependencies

### External Specs (must be implemented)
- CLI spec - provides `App` struct and Cobra patterns

### Task Dependencies (within this unit)
- Task #1 (provides: `PriorityResult`, `Recommendation`, `PrioritizeOptions`)
- Task #3 (provides: `Prioritizer`, `NewPrioritizer`)
- Task #4 (provides: `ParsePriorityResponse`)

### Package Dependencies
- `github.com/spf13/cobra` - CLI framework
- `encoding/json` - JSON output

## Deliverables

### Files to Create/Modify

```
internal/cli/
├── next_feature.go       # CREATE
└── next_feature_test.go  # CREATE
```

### Types to Implement

```go
// NextFeatureOptions holds flags for the next-feature command
type NextFeatureOptions struct {
    PRDDir  string
    Explain bool
    TopN    int
    JSON    bool
}
```

### Functions to Implement

```go
// NewNextFeatureCmd creates the next-feature command
func NewNextFeatureCmd(app *App) *cobra.Command

// RunNextFeature executes the prioritization and displays results
func (a *App) RunNextFeature(ctx context.Context, opts NextFeatureOptions) error

// formatStandardOutput formats results for terminal display
func formatStandardOutput(result *PriorityResult, explain bool) string

// formatJSONOutput formats results as JSON
func formatJSONOutput(result *PriorityResult) (string, error)
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestNextFeature -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewNextFeatureCmd_Defaults` | PRDDir="docs/prd", TopN=3, Explain=false, JSON=false |
| `TestNewNextFeatureCmd_Flags` | All flags registered and parseable |
| `TestFormatStandardOutput_Basic` | Formats rank, ID, title correctly |
| `TestFormatStandardOutput_Explain` | Includes full reasoning text |
| `TestFormatJSONOutput_Valid` | Produces valid JSON |
| `TestRunNextFeature_NoPRDDir` | Returns helpful error message |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Command Flags

- `--explain`: Show detailed reasoning (default: false)
- `--top N`: Number of recommendations (default: 3)
- `--json`: Output as JSON (default: false)
- Positional arg: custom PRD directory (default: "docs/prd")

### Output Formats

Standard output shows: rank, PRD ID, title, brief reasoning, dependencies, enables.

With `--explain`: adds full reasoning text and priority category (FOUNDATION/ENABLER/etc).

JSON output: raw `PriorityResult` struct serialized with `json.MarshalIndent`.

### Error Messages

- Missing directory: `"PRD directory not found: %s"`
- Empty directory: `"No PRD files found in %s"`
- Agent failure: `"Failed to analyze PRDs: %v"`

### Integration

Register command in `cli.go` via `a.rootCmd.AddCommand(NewNextFeatureCmd(a))`.

## NOT In Scope

- Agent invocation implementation (uses interface from Task #3)
- PRD loading logic (Task #2)
- Response parsing logic (Task #4)
- Configuration file integration
