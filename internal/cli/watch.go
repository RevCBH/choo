package cli

import (
	"context"

	"github.com/RevCBH/choo/internal/client"
	"github.com/spf13/cobra"
)

// NewWatchCmd creates the 'watch' command for attaching to job event streams
// Args: job-id (required)
// Flags: --from (int, default: 0) - sequence number to resume from
func NewWatchCmd(a *App) *cobra.Command {
	var fromSequence int

	cmd := &cobra.Command{
		Use:   "watch <job-id>",
		Short: "Attach to a running job's event stream",
		Long: `Watch events from a running job in real-time.

Use --from to resume from a specific sequence number, allowing
reconnection after network interruption without missing events.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			return watchJob(cmd.Context(), jobID, fromSequence)
		},
	}

	cmd.Flags().IntVar(&fromSequence, "from", 0, "Resume from sequence number")

	return cmd
}

// watchJob connects to the daemon and streams events for the specified job
func watchJob(ctx context.Context, jobID string, fromSequence int) error {
	c, err := client.New(defaultSocketPath())
	if err != nil {
		return err
	}
	defer c.Close()

	// WatchJob accepts fromSequence parameter (0 = from beginning)
	return c.WatchJob(ctx, jobID, fromSequence, displayEvent)
}
