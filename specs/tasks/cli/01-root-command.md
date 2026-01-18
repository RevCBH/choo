---
task: 1
status: pending
backpressure: "go build ./internal/cli/..."
depends_on: []
---

# Root Command and App Struct

**Parent spec**: `/specs/CLI.md`
**Task**: #1 of 9 in implementation plan

## Objective

Create the root Cobra command and App struct that serves as the foundation for all CLI subcommands.

## Dependencies

### External Specs (must be implemented)
- None for this task (other packages stubbed or mocked)

### Task Dependencies (within this unit)
- None (this is the foundation)

### Package Dependencies
- `github.com/spf13/cobra` v1.8+

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── cli.go    # CREATE: Root command and App struct
```

### Types to Implement

```go
// App represents the CLI application with all wired dependencies
type App struct {
    // Root command
    rootCmd *cobra.Command

    // Configuration (initialized lazily)
    config *config.Config

    // Runtime state
    verbose  bool
    cancel   context.CancelFunc
    shutdown chan struct{}
}
```

### Functions to Implement

```go
// New creates a new CLI application
func New() *App {
    // Create App instance
    // Setup root command with persistent flags
    // Return configured app
}

// Execute runs the CLI application
func (a *App) Execute() error {
    // Execute root command
    // Handle errors appropriately
}

// SetVersion sets the version string for the version command
func (a *App) SetVersion(version, commit, date string) {
    // Store version info for version command
}

// setupRootCmd configures the root Cobra command
func (a *App) setupRootCmd() {
    // Create root command with Use, Short, Long
    // Set SilenceUsage and SilenceErrors to true
    // Add persistent flags: -v/--verbose
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/cli/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNew` | `New() != nil` |
| `TestApp_RootCommand` | `app.rootCmd != nil` |
| `TestApp_VerboseFlag` | Verbose flag is registered on root command |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `SilenceUsage: true` and `SilenceErrors: true` on root command to prevent Cobra from printing usage on errors
- The verbose flag should be a persistent flag so it applies to all subcommands
- Root command's `Use` should be "choo"
- Subcommands will be added by their respective tasks via `a.rootCmd.AddCommand()`

## NOT In Scope

- Subcommand implementations (tasks #4-8)
- Signal handling setup (task #3)
- Component wiring (task #9)
- Display formatting (task #2)
