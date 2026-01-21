# DAEMON-CLI — CLI Commands for Daemon Management and Job Control

## Overview

DAEMON-CLI provides the command-line interface for interacting with the choo daemon. It enables users to start, stop, and monitor the daemon process, as well as manage jobs including starting new jobs, watching event streams, listing jobs, and stopping running jobs.

The CLI operates as a thin client that communicates with the daemon over a Unix socket. All heavy lifting (orchestration, git operations, event streaming) happens in the daemon process.

## Requirements

### Functional Requirements

1. **Daemon Management**
   - Start daemon in foreground mode
   - Stop daemon gracefully with configurable wait behavior
   - Query daemon health and status information

2. **Job Lifecycle**
   - Start new jobs via daemon with automatic event stream attachment
   - Watch running jobs by attaching to their event streams
   - Stop running jobs with optional force flag
   - List all jobs with optional status filtering

3. **Event Streaming**
   - Real-time display of job events during execution
   - Resume capability from specific sequence numbers
   - Automatic attachment after job start

4. **Backward Compatibility**
   - Support inline execution mode without daemon
   - Default to daemon mode with `--use-daemon` flag

### Performance Requirements

1. Client connection to daemon should complete within 100ms
2. Event stream latency should be under 50ms from daemon to CLI display
3. Job list queries should return within 500ms regardless of job count

### Constraints

1. Unix socket path must be deterministic and consistent across CLI invocations
2. CLI must handle daemon unavailability gracefully with clear error messages
3. All commands must respect context cancellation for clean shutdown

## Design

### Module Structure

```
internal/cli/
├── daemon.go      # Daemon management commands (start, stop, status)
├── run.go         # Run command with daemon/inline modes
├── jobs.go        # Job listing command
├── watch.go       # Event stream attachment
├── stop.go        # Job stop command
└── display.go     # Event and job display helpers
```

### Core Types

```go
// internal/cli/display.go

// displayEvent renders an event to the terminal
func displayEvent(e events.Event) {
    // Format and print event based on type
}

// displayJobs renders a job list in tabular format
func displayJobs(jobs []client.JobInfo) {
    // Format and print job table
}

// boolToStatus converts health boolean to display string
func boolToStatus(healthy bool) string {
    if healthy {
        return "healthy"
    }
    return "unhealthy"
}

// defaultSocketPath returns the standard daemon socket location.
// Uses ~/.choo/ for consistency with daemon config.
func defaultSocketPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".choo", "daemon.sock")
}
```

### API Surface

#### Daemon Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `choo daemon start` | Start daemon in foreground | None |
| `choo daemon stop` | Graceful shutdown | `--wait` (bool, default: true), `--timeout` (int, default: 30) |
| `choo daemon status` | Show daemon info | None |

#### Job Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `choo run [dir]` | Start job via daemon | `--parallelism/-p` (int, default: 4), `--target/-t` (string, default: "main"), `--feature` (string), `--use-daemon` (bool, default: true) |
| `choo watch <job-id>` | Attach to event stream | `--from` (int, default: 0) |
| `choo jobs` | List all jobs | `--status` (string, comma-separated) |
| `choo stop <job-id>` | Stop a running job | `--force/-f` (bool, default: false) |

## Implementation Notes

### Run Command

The run command supports two execution modes:

```go
// internal/cli/run.go

func runCmd() *cobra.Command {
    var (
        parallelism   int
        targetBranch  string
        featureBranch string
        useDaemon     bool
    )

    cmd := &cobra.Command{
        Use:   "run [tasks-dir]",
        Short: "Execute units in parallel",
        RunE: func(cmd *cobra.Command, args []string) error {
            tasksDir := "specs/tasks"
            if len(args) > 0 {
                tasksDir = args[0]
            }

            if useDaemon {
                return runWithDaemon(cmd.Context(), tasksDir, parallelism, targetBranch, featureBranch)
            }
            return runInline(cmd.Context(), tasksDir, parallelism, targetBranch)
        },
    }

    cmd.Flags().IntVarP(&parallelism, "parallelism", "p", 4, "Max concurrent units")
    cmd.Flags().StringVarP(&targetBranch, "target", "t", "main", "Target branch")
    cmd.Flags().StringVar(&featureBranch, "feature", "", "Feature branch (feature mode)")
    cmd.Flags().BoolVar(&useDaemon, "use-daemon", true, "Use daemon mode")

    return cmd
}

func runWithDaemon(ctx context.Context, tasksDir string, parallelism int, target, feature string) error {
    client, err := client.New(defaultSocketPath())
    if err != nil {
        return fmt.Errorf("failed to connect to daemon: %w (is daemon running?)", err)
    }
    defer client.Close()

    repoPath, _ := os.Getwd()

    jobID, err := client.StartJob(ctx, client.JobConfig{
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

    // Attach to event stream (from beginning)
    return client.WatchJob(ctx, jobID, 0, func(e events.Event) {
        displayEvent(e)
    })
}
```

### Daemon Commands

```go
// internal/cli/daemon.go

func daemonCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "daemon",
        Short: "Manage the choo daemon",
    }

    cmd.AddCommand(daemonStartCmd())
    cmd.AddCommand(daemonStopCmd())
    cmd.AddCommand(daemonStatusCmd())

    return cmd
}

func daemonStartCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "start",
        Short: "Start the daemon in foreground",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg, err := daemon.DefaultConfig()
            if err != nil {
                return fmt.Errorf("failed to load config: %w", err)
            }
            d, err := daemon.New(cfg)
            if err != nil {
                return err
            }
            return d.Start(cmd.Context())
        },
    }
}

func daemonStopCmd() *cobra.Command {
    var (
        waitForJobs bool
        timeout     int
    )

    cmd := &cobra.Command{
        Use:   "stop",
        Short: "Stop the daemon gracefully",
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := client.New(defaultSocketPath())
            if err != nil {
                return err
            }
            defer client.Close()
            return client.Shutdown(cmd.Context(), waitForJobs, timeout)
        },
    }

    cmd.Flags().BoolVar(&waitForJobs, "wait", true, "Wait for running jobs to complete")
    cmd.Flags().IntVar(&timeout, "timeout", 30, "Shutdown timeout in seconds")

    return cmd
}

func daemonStatusCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "status",
        Short: "Show daemon status",
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := client.New(defaultSocketPath())
            if err != nil {
                return fmt.Errorf("daemon not running: %w", err)
            }
            defer client.Close()

            health, err := client.Health(cmd.Context())
            if err != nil {
                return err
            }

            fmt.Printf("Daemon Status: %s\n", boolToStatus(health.Healthy))
            fmt.Printf("Active Jobs: %d\n", health.ActiveJobs)
            fmt.Printf("Version: %s\n", health.Version)
            return nil
        },
    }
}
```

### Jobs Command

```go
// internal/cli/jobs.go

func jobsCmd() *cobra.Command {
    var statusFilter string

    cmd := &cobra.Command{
        Use:   "jobs",
        Short: "List all jobs",
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := client.New(defaultSocketPath())
            if err != nil {
                return err
            }
            defer client.Close()

            var filter []string
            if statusFilter != "" {
                filter = strings.Split(statusFilter, ",")
            }

            jobs, err := client.ListJobs(cmd.Context(), filter)
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
```

### Watch Command

```go
// internal/cli/watch.go

func watchCmd() *cobra.Command {
    var fromSequence int

    cmd := &cobra.Command{
        Use:   "watch <job-id>",
        Short: "Attach to a running job's event stream",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            jobID := args[0]

            client, err := client.New(defaultSocketPath())
            if err != nil {
                return err
            }
            defer client.Close()

            return client.WatchJob(cmd.Context(), jobID, fromSequence, displayEvent)
        },
    }

    cmd.Flags().IntVar(&fromSequence, "from", 0, "Resume from sequence number")

    return cmd
}
```

### Stop Command

```go
// internal/cli/stop.go

func stopCmd() *cobra.Command {
    var force bool

    cmd := &cobra.Command{
        Use:   "stop <job-id>",
        Short: "Stop a running job",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            jobID := args[0]

            client, err := client.New(defaultSocketPath())
            if err != nil {
                return err
            }
            defer client.Close()

            return client.StopJob(cmd.Context(), jobID, force)
        },
    }

    cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop without waiting")

    return cmd
}
```

### Error Handling

All commands should provide clear, actionable error messages:

```go
// Connection errors
"failed to connect to daemon: %w (is daemon running?)"

// Daemon not running
"daemon not running: %w"

// Job not found
"job %s not found"

// Job already stopped
"job %s is not running"
```

## Testing Strategy

### Unit Tests

1. **Flag Parsing**
   - Verify default values for all flags
   - Test custom flag combinations
   - Validate status filter parsing (comma-separated values)

2. **Display Functions**
   - Test event formatting for each event type
   - Test job table rendering with various job states
   - Test status string conversion

3. **Path Resolution**
   - Verify default socket path generation
   - Test tasks directory resolution with and without arguments

### Integration Tests

1. **Daemon Lifecycle**
   - Start daemon, verify socket creation
   - Query status while running
   - Stop daemon with active jobs (wait behavior)
   - Stop daemon with timeout expiration

2. **Job Lifecycle**
   - Start job via CLI, verify daemon receives request
   - Watch job events, verify real-time streaming
   - Stop job gracefully, verify event stream ends
   - Force stop job, verify immediate termination

3. **Multi-Job Scenarios**
   - Start multiple jobs, verify parallel execution
   - List jobs with status filter
   - Watch one job while others run

### Manual Testing

```bash
# Start daemon in one terminal
choo daemon start

# In another terminal, run a job
choo run specs/tasks

# List running jobs
choo jobs --status running

# Watch a specific job
choo watch <job-id>

# Stop a job
choo stop <job-id>

# Check daemon status
choo daemon status

# Stop daemon gracefully
choo daemon stop --timeout 60

# Force stop daemon
choo daemon stop --wait=false
```

## Design Decisions

### Daemon Mode as Default

The `--use-daemon` flag defaults to `true` because:
- Daemon mode provides better resource management across multiple jobs
- Event streaming and job monitoring require the daemon
- Inline mode is preserved for debugging and simple one-off executions

### Foreground Daemon Start

The daemon starts in foreground rather than daemonizing because:
- Simplifies log viewing and debugging
- Integrates better with process supervisors (systemd, launchd)
- Avoids complexity of PID file management
- Users can background with `&` or use screen/tmux if needed

### Graceful Shutdown with Wait

The default `--wait=true` behavior ensures:
- Running jobs complete their current work
- No orphaned git operations or partial commits
- Clean event stream termination for watchers

### Watch Resume Capability

The `--from` flag enables:
- Reconnection after network interruption
- Review of historical events
- Integration with external monitoring tools

## Future Enhancements

1. **Background Daemon Mode**: Add `--daemonize` flag for production deployments
2. **Job Filtering**: Expand `jobs` command with date range and repo filters
3. **Output Formats**: Add `--output json` for scripting and automation
4. **Tab Completion**: Generate shell completions for job IDs
5. **Configuration File**: Support for default flag values in `.chooconfig`

## References

- [DAEMON-SERVICE.md](./DAEMON-SERVICE.md) - Daemon service architecture
- [DAEMON-CLIENT.md](./DAEMON-CLIENT.md) - Client library for daemon communication
- [cobra](https://github.com/spf13/cobra) - CLI framework documentation
