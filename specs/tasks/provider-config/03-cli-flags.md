---
task: 3
status: complete
backpressure: "go build ./cmd/choo/..."
depends_on: [1]
---

# CLI Provider Flags

**Parent spec**: `/specs/PROVIDER-CONFIG.md`
**Task**: #3 of 4 in implementation plan

## Objective

Add `--provider` and `--force-task-provider` CLI flags to the run command for provider selection.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ProviderType, ValidateProviderType)

### Package Dependencies
- `github.com/spf13/cobra` - CLI framework

## Deliverables

### Files to Modify

```
internal/
└── cli/
    └── run.go    # MODIFY: Add Provider and ForceTaskProvider fields and flags
```

### Types to Modify

Update `RunOptions` in `internal/cli/run.go`:

```go
// RunOptions holds flags for the run command
type RunOptions struct {
    Parallelism       int    // Max concurrent units (default: 4)
    TargetBranch      string // Branch PRs target (default: main)
    DryRun            bool   // Show execution plan without running
    NoPR              bool   // Skip PR creation
    Unit              string // Run only specified unit (single-unit mode)
    SkipReview        bool   // Auto-merge without waiting for review
    TasksDir          string // Path to specs/tasks/ directory
    Web               bool   // Enable web UI event forwarding
    WebSocket         string // Custom Unix socket path (optional)
    NoTUI             bool   // Disable TUI even when stdout is a TTY

    // Provider is the default provider for task execution
    // Units without frontmatter override use this provider
    Provider string

    // ForceTaskProvider overrides all provider settings for task inner loops
    // When set, ignores per-unit frontmatter provider field
    ForceTaskProvider string
}
```

### Validation to Add

Update `Validate()` method in `internal/cli/run.go`:

```go
// Validate checks RunOptions for validity
func (opts RunOptions) Validate() error {
    if opts.Parallelism <= 0 {
        return fmt.Errorf("parallelism must be greater than 0, got %d", opts.Parallelism)
    }
    if opts.TasksDir == "" {
        return fmt.Errorf("tasks directory must not be empty")
    }

    // Validate provider flags
    if opts.Provider != "" {
        if err := config.ValidateProviderType(opts.Provider); err != nil {
            return fmt.Errorf("invalid --provider: %w", err)
        }
    }
    if opts.ForceTaskProvider != "" {
        if err := config.ValidateProviderType(opts.ForceTaskProvider); err != nil {
            return fmt.Errorf("invalid --force-task-provider: %w", err)
        }
    }

    return nil
}
```

### Flags to Add

Update `NewRunCmd()` in `internal/cli/run.go` to add flags:

```go
func NewRunCmd(app *App) *cobra.Command {
    opts := RunOptions{
        Parallelism:       4,
        TargetBranch:      "main",
        DryRun:            false,
        NoPR:              false,
        Unit:              "",
        SkipReview:        false,
        TasksDir:          "specs/tasks",
        Provider:          "",           // Empty means use default from config/env
        ForceTaskProvider: "",           // Empty means respect per-unit settings
    }

    cmd := &cobra.Command{
        // ... existing configuration ...
    }

    // ... existing flags ...

    // Provider flags
    cmd.Flags().StringVar(&opts.Provider, "provider", "",
        "Default provider for task execution (claude, codex). Units without frontmatter override use this.")
    cmd.Flags().StringVar(&opts.ForceTaskProvider, "force-task-provider", "",
        "Force provider for ALL task execution, ignoring per-unit frontmatter (claude, codex)")

    return cmd
}
```

### Import to Add

Add import for config package in `internal/cli/run.go`:

```go
import (
    // ... existing imports ...
    "github.com/RevCBH/choo/internal/config"
)
```

## Backpressure

### Validation Command

```bash
go build ./cmd/choo/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | cmd/choo compiles without errors |
| Help output | `choo run --help` shows --provider and --force-task-provider flags |
| Flag parsing | Flags are correctly parsed into RunOptions |

### Manual Verification

```bash
# Verify flags appear in help
go run ./cmd/choo run --help | grep -E "(provider|force-task-provider)"

# Verify invalid provider is rejected
go run ./cmd/choo run --provider=invalid 2>&1 | grep -i "invalid"

# Verify valid provider is accepted (dry-run to avoid execution)
go run ./cmd/choo run --provider=codex --dry-run
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Both flags default to empty string (use existing behavior)
- ValidateProviderType is called during option validation
- Error messages should clearly indicate which flag has the invalid value
- --provider sets a default that can be overridden by unit frontmatter
- --force-task-provider overrides everything including frontmatter
- These flags only affect task execution inner loops, not other LLM operations

## NOT In Scope

- Provider resolution logic (Task #2)
- Passing provider to orchestrator (future integration task)
- Frontmatter parsing (Task #4)
- Environment variable handling (Task #2)
