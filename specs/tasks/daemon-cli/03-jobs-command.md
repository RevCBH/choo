---
task: 3
status: pending
backpressure: "go test ./internal/cli/... -run TestJobs"
depends_on: [1]
---

# Jobs Command

**Parent spec**: `specs/DAEMON-CLI.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement the `choo jobs` command for listing all jobs with optional status filtering.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete for `displayJobs` and `defaultSocketPath` helpers

### Package Dependencies
- `github.com/RevCBH/choo/internal/client` - Client for listing jobs
- `github.com/spf13/cobra` - CLI framework
- `strings` - For parsing comma-separated status filter

## Deliverables

### Files to Create/Modify

```
internal/cli/
└── jobs.go    # CREATE: Jobs listing command
```

### Functions to Implement

```go
// NewJobsCmd creates the 'jobs' command for listing all jobs
// Flags: --status (string, comma-separated filter)
func NewJobsCmd(a *App) *cobra.Command {
    var statusFilter string

    cmd := &cobra.Command{
        Use:   "jobs",
        Short: "List all jobs",
        Long: `List all jobs managed by the daemon.

Use --status to filter by job status (comma-separated values).
Valid statuses: pending, running, completed, failed`,
        RunE: func(cmd *cobra.Command, args []string) error {
            c, err := client.New(defaultSocketPath())
            if err != nil {
                return err
            }
            defer c.Close()

            var filter []string
            if statusFilter != "" {
                filter = parseStatusFilter(statusFilter)
            }

            jobs, err := c.ListJobs(cmd.Context(), filter)
            if err != nil {
                return err
            }

            displayJobs(jobs)
            return nil
        },
    }

    cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (comma-separated)")

    return cmd
}

// parseStatusFilter splits comma-separated status values and trims whitespace
func parseStatusFilter(filter string) []string {
    parts := strings.Split(filter, ",")
    result := make([]string, 0, len(parts))
    for _, p := range parts {
        trimmed := strings.TrimSpace(p)
        if trimmed != "" {
            result = append(result, trimmed)
        }
    }
    return result
}
```

### Registration

Add to `setupRootCmd()` in `cli.go`:

```go
a.rootCmd.AddCommand(NewJobsCmd(a))
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestJobs
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestJobsCmd_NoFilter` | Verifies jobs command works without filter |
| `TestJobsCmd_StatusFlag` | Verifies --status flag exists |
| `TestParseStatusFilter_Single` | Verifies single status parsed correctly |
| `TestParseStatusFilter_Multiple` | Verifies "running,completed" parsed as two values |
| `TestParseStatusFilter_Whitespace` | Verifies " running , completed " trims whitespace |
| `TestParseStatusFilter_Empty` | Verifies empty string returns empty slice |

## NOT In Scope

- Client implementation (in daemon-client)
- Job detail display (could be future `jobs info` subcommand)
- JSON output format (future enhancement)
- Date range filtering (future enhancement)
