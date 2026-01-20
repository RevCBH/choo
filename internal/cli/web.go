package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/RevCBH/choo/internal/web"
)

// NewWebCmd creates the web command.
// Usage: choo web [--port PORT] [--socket PATH]
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
