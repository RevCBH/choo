# Task 06: CLI Integration

```yaml
task: 06-cli
unit: daemon
depends_on: [03-client, 05-sse]
backpressure: "go test ./cmd/choo/... -run TestDaemon -v && go build ./cmd/choo"
```

## Objective

Integrate daemon start/connect logic into CLI commands so `choo run` automatically uses the daemon.

## Requirements

1. Add daemon subcommand to CLI:

   ```go
   // cmd/choo/daemon.go
   var daemonCmd = &cobra.Command{
       Use:   "daemon",
       Short: "Manage the choo daemon",
   }

   var daemonStartCmd = &cobra.Command{
       Use:   "start",
       Short: "Start the daemon in the foreground",
       RunE:  runDaemonStart,
   }

   var daemonStopCmd = &cobra.Command{
       Use:   "stop",
       Short: "Stop the running daemon",
       RunE:  runDaemonStop,
   }

   var daemonStatusCmd = &cobra.Command{
       Use:   "status",
       Short: "Check daemon status",
       RunE:  runDaemonStatus,
   }
   ```

2. Modify `choo run` command:
   - Add `--no-daemon` flag to disable daemon integration
   - By default, auto-start daemon if not running
   - Connect to daemon and use it for event persistence
   - Pass daemon client to orchestrator/handler

3. Auto-start behavior:
   ```go
   func ensureDaemon() (*daemon.Client, error) {
       client, err := daemon.Connect()
       if err == nil {
           return client, nil // Already running
       }

       // Start daemon in background
       if err := startDaemonBackground(); err != nil {
           return nil, fmt.Errorf("failed to start daemon: %w", err)
       }

       // Wait for daemon to be ready (with timeout)
       return waitForDaemon(5 * time.Second)
   }
   ```

4. Background daemon start:
   - Fork a new process running `choo daemon start`
   - Redirect stdout/stderr to log file
   - Detach from terminal (daemonize)
   - Return after daemon is healthy

5. Update history handler:
   - Modify `internal/history/handler.go` to use daemon client
   - Handler sends events via HTTP instead of direct DB writes

## Acceptance Criteria

- [ ] `choo daemon start` runs daemon in foreground
- [ ] `choo daemon stop` gracefully stops daemon
- [ ] `choo daemon status` shows daemon health
- [ ] `choo run` auto-starts daemon if needed
- [ ] `choo run --no-daemon` works without daemon
- [ ] Events are persisted via daemon during runs

## Files to Create/Modify

- `cmd/choo/daemon.go` (create)
- `cmd/choo/run.go` (modify - add daemon integration)
- `internal/history/handler.go` (modify - use daemon client)
