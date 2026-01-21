---
task: 3
status: complete
backpressure: "go build ./internal/cli/..."
depends_on: []
---

# Feature Parent Command

**Parent spec**: `/specs/FEATURE-CLI.md`
**Task**: #3 of 6 in implementation plan

## Objective

Create the `choo feature` parent command that groups all feature workflow subcommands.

## Dependencies

### External Specs (must be implemented)
- CLI spec must be implemented (provides: `App` struct, `NewRootCmd`)

### Task Dependencies (within this unit)
- None (parallel with task #1)

### Package Dependencies
- `github.com/spf13/cobra`

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── feature.go    # CREATE: Feature parent command
```

### Functions to Implement

```go
// NewFeatureCmd creates the feature parent command
func NewFeatureCmd(app *App) *cobra.Command {
    // Create command with:
    //   Use: "feature"
    //   Short: "Manage feature development workflows"
    //   Long: Extended description
    // Return configured command (subcommands added separately)
}
```

### Command Structure

```
choo feature
  Manage feature development workflows

Usage:
  choo feature [command]

Available Commands:
  start       Create feature branch, generate specs with review, generate tasks
  status      Show status of feature workflows
  resume      Resume a blocked feature workflow

Flags:
  -h, --help   help for feature

Use "choo feature [command] --help" for more information about a command.
```

## Backpressure

### Validation Command

```bash
go build ./internal/cli/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewFeatureCmd` | `cmd.Use == "feature"` |
| `TestNewFeatureCmd_Short` | `cmd.Short` contains "feature" and "workflow" |
| `TestFeatureCmd_NoRunE` | Parent command has no RunE (subcommand required) |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The parent command should not have a `RunE` function (requires subcommand)
- Subcommands will be added in their respective tasks (#4, #5, #6)
- Registration with root command happens in the main wiring (separate from this task)
- Keep `Long` description concise but informative

## NOT In Scope

- Subcommand implementations (tasks #4-6)
- Root command registration (wiring task)
- State types (task #1)
- PRD operations (task #2)
