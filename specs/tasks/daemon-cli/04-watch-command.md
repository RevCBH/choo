---
task: 4
status: pending
backpressure: "go test ./internal/cli/... -run TestWatch"
depends_on: [1]
---

# Watch Command

**Parent spec**: `specs/DAEMON-CLI.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the `choo watch` command for attaching to a running job's event stream with optional resume capability.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete for `displayEvent` and `defaultSocketPath` helpers

### Package Dependencies
- `github.com/RevCBH/choo/internal/client` - Client for watching jobs
- `github.com/RevCBH/choo/internal/events` - Event types
- `github.com/spf13/cobra` - CLI framework

## Deliverables

### Files to Create/Modify

```
internal/cli/
└── watch.go    # CREATE: Watch command for event streaming
```

### Functions to Implement

```go
// NewWatchCmd creates the 'watch' command for attaching to job event streams
// Args: job-id (required)
// Flags: --from (int, default: 0) - sequence number to resume from
func NewWatchCmd(a *App) *cobra.Command {
    var fromSequence int

    cmd := &cobra.Command{
        Use:   "watch <job-id>",
        Short: "Attach to a running job's event stream",
        Long: `Watch events from a running job in real-time.

Use --from to resume from a specific sequence number, allowing
reconnection after network interruption without missing events.`,
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            jobID := args[0]
            return watchJob(cmd.Context(), jobID, fromSequence)
        },
    }

    cmd.Flags().IntVar(&fromSequence, "from", 0, "Resume from sequence number")

    return cmd
}

// watchJob connects to the daemon and streams events for the specified job
func watchJob(ctx context.Context, jobID string, fromSequence int) error {
    c, err := client.New(defaultSocketPath())
    if err != nil {
        return err
    }
    defer c.Close()

    // WatchJob accepts fromSequence parameter (0 = from beginning)
    return c.WatchJob(ctx, jobID, fromSequence, displayEvent)
}
```

### Registration

Add to `setupRootCmd()` in `cli.go`:

```go
a.rootCmd.AddCommand(NewWatchCmd(a))
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestWatch
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWatchCmd_RequiresJobID` | Verifies command fails without job-id argument |
| `TestWatchCmd_FromFlag` | Verifies --from flag exists with default 0 |
| `TestWatchCmd_AcceptsJobID` | Verifies command parses job-id correctly |
| `TestWatchJob_ContextCancellation` | Verifies watch respects context cancellation |

## NOT In Scope

- Client implementation (in daemon-client)
- Event filtering by type (future enhancement)
- Event replay/history from database (future enhancement)
- JSON output format (future enhancement)
