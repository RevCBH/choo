package cli

import (
	"context"
	"fmt"

	"github.com/RevCBH/choo/internal/client"
	"github.com/spf13/cobra"
)

// NewStopJobCmd creates the 'stop' command for stopping a running job
// Args: job-id (required)
// Flags: --force/-f (bool, default: false) - force immediate termination
func NewStopJobCmd(a *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop <job-id>",
		Short: "Stop a running job",
		Long: `Stop a running job managed by the daemon.

By default, the job will complete its current task before stopping.
Use --force to immediately terminate the job without waiting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			return stopJob(cmd.Context(), jobID, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop without waiting")

	return cmd
}

// stopJob connects to the daemon and stops the specified job
func stopJob(ctx context.Context, jobID string, force bool) error {
	c, err := client.New(defaultSocketPath())
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.StopJob(ctx, jobID, force); err != nil {
		return err
	}

	if force {
		fmt.Printf("Job %s force stopped\n", jobID)
	} else {
		fmt.Printf("Job %s stop requested\n", jobID)
	}
	return nil
}
