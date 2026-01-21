package cli

import (
	"fmt"

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
// Starts the daemon in foreground mode, blocking until shutdown
func newDaemonStartCmd(a *App) *cobra.Command {
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
			c, err := client.New(defaultSocketPath())
			if err != nil {
				return err
			}
			defer c.Close()
			return c.Shutdown(cmd.Context(), waitForJobs, timeout)
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
