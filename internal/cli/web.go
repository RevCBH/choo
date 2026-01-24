package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/web"
	"github.com/spf13/cobra"
)

// NewWebCmd creates the web command.
// Usage: choo web [tasks-dir] [--port PORT] [--socket PATH]
func NewWebCmd(app *App) *cobra.Command {
	var port string
	var socketPath string
	var tasksDir string

	cmd := &cobra.Command{
		Use:   "web [tasks-dir]",
		Short: "Start the web monitoring server",
		Long: `Starts a web server that displays real-time orchestration status.

The server receives events from 'choo run' via Unix socket and
broadcasts them to connected browsers via Server-Sent Events.

Open http://localhost:8080 in your browser to view the dashboard.

If a tasks directory is available, the dependency graph is preloaded so it can
be viewed before a run starts.

Press Ctrl+C to stop the server.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				tasksDir = args[0]
			}

			addr := ":" + port

			store := web.NewStore()
			workdir, _ := os.Getwd()
			if tasksDir != "" {
				resolvedTasksDir, err := resolveTasksDir(tasksDir)
				if err != nil {
					return fmt.Errorf("resolve tasks dir: %w", err)
				}
				repoRoot, branch := resolveRepoInfo(resolvedTasksDir, workdir)
				store.SetWorkspaceInfo(workdir, repoRoot, branch)
				graph, units, err := buildWebState(resolvedTasksDir, repoRoot)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load dependency graph from %s: %v\n", resolvedTasksDir, err)
				} else if graph != nil && len(graph.Nodes) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: no units found under %s\n", resolvedTasksDir)
				} else {
					store.SeedState(graph, units)
				}
			}

			cfg := web.Config{
				Addr:       addr,
				SocketPath: socketPath,
			}

			srv, err := web.NewWithStore(cfg, store)
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
	cmd.Flags().StringVar(&tasksDir, "tasks", "specs/tasks", "Path to tasks directory")

	return cmd
}

func resolveTasksDir(tasksDir string) (string, error) {
	if tasksDir == "" {
		return "", nil
	}
	if filepath.IsAbs(tasksDir) {
		return tasksDir, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	candidate := filepath.Join(wd, tasksDir)
	if dirExists(candidate) {
		return candidate, nil
	}

	dir := wd
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
		candidate = filepath.Join(dir, tasksDir)
		if dirExists(candidate) {
			return candidate, nil
		}
	}

	return filepath.Join(wd, tasksDir), nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func resolveRepoInfo(tasksDir, workdir string) (string, string) {
	dir := tasksDir
	if dir == "" {
		dir = workdir
	}
	if dir == "" {
		return "", ""
	}

	ctx := context.Background()
	runner := git.DefaultRunner()

	repoRoot, err := runner.Exec(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ""
	}
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "", ""
	}

	branch, err := runner.Exec(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return repoRoot, ""
	}
	branch = strings.TrimSpace(branch)
	return repoRoot, branch
}
