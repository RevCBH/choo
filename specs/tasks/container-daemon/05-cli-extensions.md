---
task: 5
status: complete
backpressure: "go build ./cmd/choo/..."
depends_on: [1]
---

# CLI Extensions

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #5 of 6 in implementation plan

## Objective

Extend CLI commands with container mode flags for daemon start and run commands.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides container config types)

### Package Dependencies
- `github.com/spf13/cobra` (existing dependency)

## Deliverables

### Files to Create/Modify

```
internal/cli/
├── daemon.go  # MODIFY: Add container mode flags to daemon start
└── run.go     # MODIFY: Add --clone-url and --json-events flags
```

### Types to Implement

```go
// internal/cli/daemon.go - Extend existing struct

// DaemonStartOptions holds flags for daemon start command.
type DaemonStartOptions struct {
    // Existing fields
    Foreground bool
    Verbose    bool

    // Container mode additions
    ContainerMode    bool   // Enable container isolation
    ContainerImage   string // Image to use for jobs
    ContainerRuntime string // Runtime selection: auto, docker, podman
}
```

```go
// internal/cli/run.go - Extend existing struct

// RunOptions holds flags for the run command.
type RunOptions struct {
    // Existing fields
    Parallelism  int
    TargetBranch string
    DryRun       bool
    NoPR         bool
    Unit         string
    SkipReview   bool
    TasksDir     string

    // Container mode additions
    CloneURL   string // URL to clone before running (used in container)
    JSONEvents bool   // Emit events as JSON to stdout (for daemon parsing)
}
```

### Functions to Implement

```go
// internal/cli/daemon.go

// registerDaemonStartFlags adds flags to the daemon start command.
func registerDaemonStartFlags(cmd *cobra.Command, opts *DaemonStartOptions) {
    // Existing flags
    cmd.Flags().BoolVarP(&opts.Foreground, "foreground", "f", false,
        "Run daemon in foreground (don't daemonize)")
    cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false,
        "Enable verbose logging")

    // Container mode flags
    cmd.Flags().BoolVar(&opts.ContainerMode, "container-mode", false,
        "Enable container isolation for job execution")
    cmd.Flags().StringVar(&opts.ContainerImage, "container-image", "choo:latest",
        "Container image to use for jobs")
    cmd.Flags().StringVar(&opts.ContainerRuntime, "container-runtime", "auto",
        "Container runtime: auto, docker, or podman")
}

// buildDaemonConfig creates a daemon.Config from CLI options.
func buildDaemonConfig(opts DaemonStartOptions) (*daemon.Config, error) {
    cfg, err := daemon.DefaultConfig()
    if err != nil {
        return nil, err
    }

    // Apply container mode options
    cfg.ContainerMode = opts.ContainerMode
    cfg.ContainerImage = opts.ContainerImage
    cfg.ContainerRuntime = opts.ContainerRuntime

    return cfg, nil
}
```

```go
// internal/cli/run.go

// registerRunFlags adds flags to the run command.
func registerRunFlags(cmd *cobra.Command, opts *RunOptions) {
    // Existing flags
    cmd.Flags().IntVarP(&opts.Parallelism, "parallelism", "p", 1,
        "Maximum number of units to execute concurrently")
    cmd.Flags().StringVarP(&opts.TargetBranch, "target", "t", "main",
        "Target branch for PRs")
    cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false,
        "Show what would be executed without running")
    cmd.Flags().BoolVar(&opts.NoPR, "no-pr", false,
        "Skip PR creation")
    cmd.Flags().StringVarP(&opts.Unit, "unit", "u", "",
        "Run only the specified unit")
    cmd.Flags().BoolVar(&opts.SkipReview, "skip-review", false,
        "Skip review polling")
    cmd.Flags().StringVar(&opts.TasksDir, "tasks", "specs/tasks",
        "Path to tasks directory")

    // Container mode flags
    cmd.Flags().StringVar(&opts.CloneURL, "clone-url", "",
        "Clone repository from URL before running (container mode)")
    cmd.Flags().BoolVar(&opts.JSONEvents, "json-events", false,
        "Emit events as JSON to stdout (for daemon parsing)")
}

// isContainerMode returns true if running in container mode.
func (opts RunOptions) isContainerMode() bool {
    return opts.CloneURL != "" || opts.JSONEvents
}
```

```go
// internal/cli/archive.go - Add command registration

// NewArchiveCmd creates the archive command.
func NewArchiveCmd() *cobra.Command {
    opts := ArchiveOptions{}

    cmd := &cobra.Command{
        Use:   "archive",
        Short: "Move completed specs to specs/completed/",
        Long: `Archive moves spec files with "status: complete" in their
frontmatter to the specs/completed/ directory.

This command is typically run automatically after all units in a
feature have completed, but can be run manually to clean up specs.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            archived, err := Archive(opts)
            if err != nil {
                return err
            }

            if len(archived) == 0 {
                fmt.Println("No specs to archive")
            } else {
                fmt.Printf("Archived %d specs\n", len(archived))
            }
            return nil
        },
    }

    cmd.Flags().StringVar(&opts.SpecsDir, "specs", "specs",
        "Path to specs directory")
    cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false,
        "Show what would be archived without moving files")
    cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false,
        "Enable verbose output")

    return cmd
}
```

## Backpressure

### Validation Command

```bash
go build ./cmd/choo/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Binary compiles without errors |
| `--container-mode` | Flag is recognized by daemon start |
| `--container-image` | Flag accepts string argument |
| `--container-runtime` | Flag accepts auto/docker/podman |
| `--clone-url` | Flag is recognized by run command |
| `--json-events` | Flag is recognized by run command |
| `choo archive --help` | Command is registered and shows help |

### Manual Verification

```bash
# Verify daemon start flags
./choo daemon start --help | grep -E "(container-mode|container-image|container-runtime)"

# Verify run flags
./choo run --help | grep -E "(clone-url|json-events)"

# Verify archive command
./choo archive --help
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Container runtime defaults to "auto" which auto-detects docker or podman
- Container image defaults to "choo:latest"
- CloneURL is used in container mode to clone repo before running
- JSONEvents outputs structured events for daemon parsing
- Archive command can be run standalone or as part of orchestrator completion

## NOT In Scope

- Container creation logic (Task #3)
- Log streaming (Task #2)
- Orchestrator integration (Task #6)
- Event emission implementation (JSON-EVENTS spec)
