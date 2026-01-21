package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/RevCBH/choo/internal/client"
	"github.com/RevCBH/choo/internal/daemon"
	"github.com/spf13/cobra"
)

// NewDaemonCmd creates the daemon command group with start, stop, status subcommands
func NewDaemonCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the choo daemon",
	}

	cmd.AddCommand(newDaemonStartCmd(a))
	cmd.AddCommand(newDaemonStopCmd(a))
	cmd.AddCommand(newDaemonStatusCmd(a))

	return cmd
}

// newDaemonStartCmd creates the 'daemon start' command
// By default, starts the daemon in the background after checking if it's already running.
// Use --foreground to run in blocking mode (useful for debugging or process managers).
func newDaemonStartCmd(a *App) *cobra.Command {
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if daemon is already running
			if isDaemonRunning() {
				fmt.Println("Daemon is already running")
				return nil
			}

			if foreground {
				// Run in foreground (original behavior)
				cfg, err := daemon.DefaultConfig()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}
				d, err := daemon.New(cfg)
				if err != nil {
					return err
				}
				return d.Start(cmd.Context())
			}

			// Start daemon in background
			return startDaemonBackground()
		},
	}

	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run daemon in foreground (blocking)")

	return cmd
}

// isDaemonRunning checks if the daemon is already running by checking
// the PID file and verifying the process exists.
func isDaemonRunning() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	pidPath := filepath.Join(home, ".choo", "daemon.pid")

	pid, err := daemon.ReadPID(pidPath)
	if err != nil {
		return false
	}

	return daemon.IsProcessRunning(pid)
}

// startDaemonBackground spawns the daemon process in the background.
func startDaemonBackground() error {
	// Get the path to the current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Get log file path
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	chooDir := filepath.Join(home, ".choo")
	if err := os.MkdirAll(chooDir, 0700); err != nil {
		return fmt.Errorf("failed to create .choo directory: %w", err)
	}
	logPath := filepath.Join(chooDir, "daemon.log")

	// Open log file for daemon output
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Start daemon process with --foreground flag
	cmd := exec.Command(exe, "daemon", "start", "--foreground")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Detach from parent process group
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process - let it run in background
	// Release the process so it doesn't become a zombie
	go func() {
		cmd.Wait()
		logFile.Close()
	}()

	// Wait briefly and verify the daemon started successfully
	time.Sleep(500 * time.Millisecond)
	if isDaemonRunning() {
		fmt.Printf("Daemon started (PID: %d)\n", cmd.Process.Pid)
		fmt.Printf("Logs: %s\n", logPath)
		return nil
	}

	return fmt.Errorf("daemon failed to start - check %s for details", logPath)
}

// newDaemonStopCmd creates the 'daemon stop' command
// Flags: --wait (bool, default: true), --timeout (int, default: 30)
func newDaemonStopCmd(a *App) *cobra.Command {
	var (
		waitForJobs bool
		timeout     int
	)

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon gracefully",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if daemon is running first
			if !isDaemonRunning() {
				fmt.Println("Daemon is not running")
				return nil
			}

			c, err := client.New(defaultSocketPath())
			if err != nil {
				// Connection failed - daemon probably not running
				fmt.Println("Daemon is not running")
				return nil
			}
			defer c.Close()

			if err := c.Shutdown(cmd.Context(), waitForJobs, timeout); err != nil {
				return err
			}
			fmt.Println("Daemon stopped")
			return nil
		},
	}

	cmd.Flags().BoolVar(&waitForJobs, "wait", true, "Wait for running jobs to complete")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Shutdown timeout in seconds")

	return cmd
}

// newDaemonStatusCmd creates the 'daemon status' command
// Displays: Daemon Status, Active Jobs, Version
func newDaemonStatusCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(defaultSocketPath())
			if err != nil {
				return fmt.Errorf("daemon not running: %w", err)
			}
			defer c.Close()

			health, err := c.Health(cmd.Context())
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
