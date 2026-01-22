package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/RevCBH/choo/internal/client"
	"github.com/RevCBH/choo/internal/daemon"
	"github.com/spf13/cobra"
)

// NewDaemonCmd creates the daemon command group with start, stop, status, logs subcommands
func NewDaemonCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the choo daemon",
	}

	cmd.AddCommand(newDaemonStartCmd(a))
	cmd.AddCommand(newDaemonStopCmd(a))
	cmd.AddCommand(newDaemonStatusCmd(a))
	cmd.AddCommand(newDaemonLogsCmd(a))

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
// The child process is started in its own process group and session,
// so it won't be affected by signals sent to the parent (e.g., Ctrl+C).
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
	cmd.Stdin = nil

	// Create a new process group and session so the daemon is fully detached
	// from the terminal. This prevents Ctrl+C from killing the daemon.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // Child becomes process group leader
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Record the PID for later verification
	pid := cmd.Process.Pid

	// Release the process - the daemon runs independently now.
	// We don't call cmd.Wait() because that would block until the daemon exits.
	// Instead, we detach completely and let the daemon manage itself.
	if err := cmd.Process.Release(); err != nil {
		// Non-fatal - the process is already running
		fmt.Fprintf(os.Stderr, "Warning: failed to release process: %v\n", err)
	}

	// Close the log file handle in the parent - the child has its own copy
	logFile.Close()

	// Poll with retries to verify daemon started successfully.
	// Use exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms (total ~3s)
	// This handles slower systems while still being responsive on fast ones.
	const maxRetries = 5
	delay := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		time.Sleep(delay)
		if isDaemonRunning() {
			fmt.Printf("Daemon started (PID: %d)\n", pid)
			fmt.Printf("Logs: %s\n", logPath)
			return nil
		}
		delay *= 2
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

// newDaemonLogsCmd creates the 'daemon logs' command
// Shows daemon log output with optional follow mode
func newDaemonLogsCmd(a *App) *cobra.Command {
	var (
		follow bool
		lines  int
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show daemon logs",
		Long:  `Display daemon log output. Use -f to follow logs in real-time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			logPath := filepath.Join(home, ".choo", "daemon.log")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				return fmt.Errorf("no daemon logs found at %s", logPath)
			}

			if follow {
				return followLogs(cmd, logPath, lines)
			}
			return showLogs(logPath, lines)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (like tail -f)")
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "Number of lines to show (0 for all)")

	return cmd
}

// showLogs displays the last N lines of the log file.
// Uses an efficient approach that reads from the end of the file
// instead of loading the entire file into memory.
func showLogs(logPath string, lines int) error {
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if lines == 0 {
		// Show all lines
		_, err = io.Copy(os.Stdout, f)
		return err
	}

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return nil // Empty file
	}

	// Read from end to find the position of the Nth newline from the end.
	// We use a buffer to read chunks from the end.
	const bufSize = 8192
	buf := make([]byte, bufSize)
	newlineCount := 0
	var startPos int64 = 0

	// Start from end of file and work backwards
	pos := fileSize
	for pos > 0 && newlineCount <= lines {
		// Calculate how much to read
		readSize := int64(bufSize)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize

		// Seek and read
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek: %w", err)
		}
		n, err := f.Read(buf[:readSize])
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read: %w", err)
		}

		// Count newlines from end of buffer
		for i := n - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				newlineCount++
				if newlineCount > lines {
					// Found the position - start after this newline
					startPos = pos + int64(i) + 1
					break
				}
			}
		}
	}

	// Seek to start position and copy to stdout
	if _, err := f.Seek(startPos, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	_, err = io.Copy(os.Stdout, f)
	return err
}

// followLogs tails the log file, showing new content as it's written
func followLogs(cmd *cobra.Command, logPath string, initialLines int) error {
	// First show the last N lines
	if initialLines > 0 {
		if err := showLogs(logPath, initialLines); err != nil {
			return err
		}
	}

	// Open file for following
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	// Seek to end of file
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Follow new content
	reader := bufio.NewReader(f)
	for {
		select {
		case <-cmd.Context().Done():
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// No new content, wait a bit
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return fmt.Errorf("error reading log file: %w", err)
			}
			fmt.Print(line)
		}
	}
}
