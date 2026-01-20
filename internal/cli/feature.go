package cli

import (
	"github.com/spf13/cobra"
)

// NewFeatureCmd creates the feature parent command
func NewFeatureCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage feature development workflows",
		Long: `Commands for starting, monitoring, and resuming feature workflows from PRDs.

The feature workflow orchestrates the end-to-end process from PRD to committed
specs and tasks, managing state transitions and providing resume capability for
blocked states.`,
	}

	// Subcommands will be added in their respective tasks (#4, #5, #6)
	cmd.AddCommand(
		NewFeatureStartCmd(app),
		NewFeatureStatusCmd(app),
		NewFeatureResumeCmd(app),
	)

	return cmd
}
