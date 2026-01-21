package cli

import (
	"strings"

	"github.com/RevCBH/choo/internal/client"
	"github.com/spf13/cobra"
)

// NewJobsCmd creates the 'jobs' command for listing all jobs
// Flags: --status (string, comma-separated filter)
func NewJobsCmd(a *App) *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List all jobs",
		Long: `List all jobs managed by the daemon.

Use --status to filter by job status (comma-separated values).
Valid statuses: pending, running, completed, failed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(defaultSocketPath())
			if err != nil {
				return err
			}
			defer c.Close()

			var filter []string
			if statusFilter != "" {
				filter = parseStatusFilter(statusFilter)
			}

			jobs, err := c.ListJobs(cmd.Context(), filter)
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

// parseStatusFilter splits comma-separated status values and trims whitespace
func parseStatusFilter(filter string) []string {
	parts := strings.Split(filter, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
