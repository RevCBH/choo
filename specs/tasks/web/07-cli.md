---
task: 7
status: pending
backpressure: "go build ./... && ./choo web --help"
depends_on: [6]
---

# Implement CLI Command

**Parent spec**: `/specs/WEB.md`
**Task**: #7 of 7 in implementation plan

## Objective

Implement the `choo web` CLI command that starts the web server daemon with proper signal handling for graceful shutdown.

## Dependencies

### Task Dependencies (within this unit)
- #6 (server.go) - Server with Start/Stop lifecycle

### Package Dependencies
- `context`
- `fmt`
- `os`
- `os/signal`
- `syscall`
- `github.com/spf13/cobra`

## Deliverables

### Files to Create

```
internal/cli/
└── web.go    # CREATE: CLI command for choo web
```

### Functions to Implement

```go
// web.go
package cli

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/spf13/cobra"
    "little-rock/internal/web"
)

// NewWebCmd creates the web command.
// Usage: choo web [--port PORT] [--socket PATH]
func NewWebCmd(app *App) *cobra.Command
```

### Command Specification

```
choo web - Start the web monitoring server

USAGE:
    choo web [flags]

FLAGS:
    --port string     HTTP port to listen on (default "8080")
    --socket string   Unix socket path (default ~/.choo/web.sock)
    -h, --help        Help for web

DESCRIPTION:
    Starts a web server that displays real-time orchestration status.
    The server receives events from 'choo run' via Unix socket and
    broadcasts them to connected browsers via Server-Sent Events.

    Open http://localhost:8080 in your browser to view the dashboard.

    Press Ctrl+C to stop the server.

EXAMPLES:
    # Start with defaults
    choo web

    # Use custom port
    choo web --port 3000

    # Use custom socket path
    choo web --socket /tmp/choo.sock
```

### Tests to Implement

No separate test file needed - the backpressure command validates the implementation:

```bash
go build ./... && ./choo web --help
```

Additional manual verification:
- `choo web` starts and prints listening address
- Ctrl+C triggers graceful shutdown
- Socket file is cleaned up on exit

## Backpressure

### Validation Command

```bash
go build ./... && ./choo web --help
```

### Must Pass
- Build succeeds
- `choo web --help` shows usage
- Command is registered in root command

### CI Compatibility
- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Command Implementation

```go
func NewWebCmd(app *App) *cobra.Command {
    var port string
    var socketPath string

    cmd := &cobra.Command{
        Use:   "web",
        Short: "Start the web monitoring server",
        Long: `Starts a web server that displays real-time orchestration status.

The server receives events from 'choo run' via Unix socket and
broadcasts them to connected browsers via Server-Sent Events.

Open http://localhost:8080 in your browser to view the dashboard.

Press Ctrl+C to stop the server.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            addr := ":" + port

            cfg := web.Config{
                Addr:       addr,
                SocketPath: socketPath,
            }

            srv, err := web.New(cfg)
            if err != nil {
                return fmt.Errorf("create server: %w", err)
            }

            if err := srv.Start(); err != nil {
                return fmt.Errorf("start server: %w", err)
            }

            fmt.Printf("Web server listening on http://localhost%s\n", addr)
            fmt.Printf("Unix socket: %s\n", srv.SocketPath())
            fmt.Println("Press Ctrl+C to stop")

            // Wait for interrupt signal
            sigCh := make(chan os.Signal, 1)
            signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
            <-sigCh

            fmt.Println("\nShutting down...")

            // Graceful shutdown with timeout
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            if err := srv.Stop(ctx); err != nil {
                return fmt.Errorf("stop server: %w", err)
            }

            fmt.Println("Server stopped")
            return nil
        },
    }

    cmd.Flags().StringVar(&port, "port", "8080", "HTTP port to listen on")
    cmd.Flags().StringVar(&socketPath, "socket", "", "Unix socket path (default ~/.choo/web.sock)")

    return cmd
}
```

### Register in Root Command

Add to `internal/cli/cli.go` in the appropriate place:

```go
rootCmd.AddCommand(NewWebCmd(app))
```

### Signal Handling

- Use `signal.Notify` to catch SIGINT (Ctrl+C) and SIGTERM
- On signal, call `srv.Stop()` with a timeout context
- 10 second timeout allows in-flight requests to complete
- Print status messages so user knows what's happening

### Output Format

Keep output minimal and informative:

```
Web server listening on http://localhost:8080
Unix socket: /Users/alice/.choo/web.sock
Press Ctrl+C to stop

^C
Shutting down...
Server stopped
```

## NOT In Scope

- Daemonization (background process)
- PID file management
- Log file configuration
- TLS/HTTPS support
