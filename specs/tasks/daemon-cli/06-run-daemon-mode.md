---
task: 6
status: pending
backpressure: "go test ./internal/cli/... -run TestRunDaemon"
depends_on: [1, 4]
---

# Run Command Daemon Mode

**Parent spec**: `specs/DAEMON-CLI.md`
**Task**: #6 of 6 in implementation plan

## Objective

Extend the existing run command to support daemon mode execution with automatic event stream attachment.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete for `displayEvent` and `defaultSocketPath` helpers
- Task #4 must be complete for watch functionality patterns

### Package Dependencies
- `github.com/RevCBH/choo/internal/client` - Client for starting jobs and watching events
- `github.com/spf13/cobra` - CLI framework
- Existing `internal/cli/run.go` - Run command to modify

## Deliverables

### Files to Create/Modify

```
internal/cli/
└── run.go    # MODIFY: Add daemon mode support to existing run command
```

### Changes to Implement

Add new flag to existing run command:

```go
// Add to existing flag variables
var useDaemon bool

// Add flag registration in NewRunCmd
cmd.Flags().BoolVar(&useDaemon, "use-daemon", true, "Use daemon mode")
```

Add daemon mode execution function:

```go
// runWithDaemon executes a job via the daemon and attaches to event stream
func runWithDaemon(ctx context.Context, tasksDir string, parallelism int, target, feature string) error {
    c, err := client.New(defaultSocketPath())
    if err != nil {
        return fmt.Errorf("failed to connect to daemon: %w (is daemon running?)", err)
    }
    defer c.Close()

    repoPath, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("failed to get working directory: %w", err)
    }

    jobID, err := c.StartJob(ctx, client.JobConfig{
        TasksDir:      tasksDir,
        TargetBranch:  target,
        FeatureBranch: feature,
        Parallelism:   parallelism,
        RepoPath:      repoPath,
    })
    if err != nil {
        return err
    }

    fmt.Printf("Started job %s\n", jobID)

    // Attach to event stream and display events
    return c.WatchJob(ctx, jobID, displayEvent)
}
```

Modify run command's RunE to dispatch based on mode:

```go
// In the existing RunE function, add mode dispatch
RunE: func(cmd *cobra.Command, args []string) error {
    tasksDir := "specs/tasks"
    if len(args) > 0 {
        tasksDir = args[0]
    }

    if useDaemon {
        return runWithDaemon(cmd.Context(), tasksDir, parallelism, targetBranch, featureBranch)
    }
    // Existing inline execution path
    return runInline(cmd.Context(), tasksDir, parallelism, targetBranch)
}
```

Rename existing inline execution to `runInline`:

```go
// runInline executes jobs directly without daemon (existing behavior)
func runInline(ctx context.Context, tasksDir string, parallelism int, target string) error {
    // Move existing run logic here
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestRunDaemon
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunCmd_UseDaemonFlag` | Verifies --use-daemon flag exists with default true |
| `TestRunCmd_UseDaemonFalse` | Verifies --use-daemon=false uses inline mode |
| `TestRunWithDaemon_ConnectionError` | Verifies helpful error message when daemon not running |
| `TestRunWithDaemon_JobStart` | Verifies job ID printed on successful start |
| `TestRunCmd_PreservesExistingFlags` | Verifies -p, -t, --feature flags still work |

## NOT In Scope

- Client implementation (in daemon-client)
- Daemon auto-start if not running (future enhancement)
- Job ID output format options (future enhancement)
- Detached mode without event streaming (future enhancement)
