---
task: 5
status: complete
backpressure: "go test ./internal/cli/... -run TestStopJob"
depends_on: []
---

# Stop Command

**Parent spec**: `specs/DAEMON-CLI.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the `choo stop` command for stopping a running job with optional force flag.

## Dependencies

### Task Dependencies (within this unit)
- None (uses only defaultSocketPath from task #1, but that's a simple function)

### Package Dependencies
- `github.com/RevCBH/choo/internal/client` - Client for stopping jobs
- `github.com/spf13/cobra` - CLI framework

## Deliverables

### Files to Create/Modify

```
internal/cli/
└── stop_job.go    # CREATE: Stop job command (named to avoid conflict with existing stop.go)
```

### Functions to Implement

```go
// NewStopJobCmd creates the 'stop' command for stopping a running job
// Args: job-id (required)
// Flags: --force/-f (bool, default: false) - force immediate termination
func NewStopJobCmd(a *App) *cobra.Command {
    var force bool

    cmd := &cobra.Command{
        Use:   "stop <job-id>",
        Short: "Stop a running job",
        Long: `Stop a running job managed by the daemon.

By default, the job will complete its current task before stopping.
Use --force to immediately terminate the job without waiting.`,
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            jobID := args[0]
            return stopJob(cmd.Context(), jobID, force)
        },
    }

    cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop without waiting")

    return cmd
}

// stopJob connects to the daemon and stops the specified job
func stopJob(ctx context.Context, jobID string, force bool) error {
    c, err := client.New(defaultSocketPath())
    if err != nil {
        return err
    }
    defer c.Close()

    if err := c.StopJob(ctx, jobID, force); err != nil {
        return err
    }

    if force {
        fmt.Printf("Job %s force stopped\n", jobID)
    } else {
        fmt.Printf("Job %s stop requested\n", jobID)
    }
    return nil
}
```

### Registration

Add to `setupRootCmd()` in `cli.go`:

```go
a.rootCmd.AddCommand(NewStopJobCmd(a))
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestStopJob
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestStopJobCmd_RequiresJobID` | Verifies command fails without job-id argument |
| `TestStopJobCmd_ForceFlag` | Verifies --force and -f flags exist with default false |
| `TestStopJobCmd_AcceptsJobID` | Verifies command parses job-id correctly |
| `TestStopJob_SuccessMessage` | Verifies appropriate message printed on success |
| `TestStopJob_ForceMessage` | Verifies "force stopped" message when --force used |

## NOT In Scope

- Client implementation (in daemon-client)
- Stopping multiple jobs at once (future enhancement)
- Confirmation prompt (future enhancement)
- Job status verification before stop (handled by daemon)
