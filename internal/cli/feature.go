package cli

import (
	"github.com/spf13/cobra"
)

// NewFeatureCmd creates the feature parent command
func NewFeatureCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage feature development workflows",
		Long: `Manage feature development workflows including creating feature branches,
generating specifications with review, generating tasks, and managing
feature workflow state.`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewFeatureStartCmd(app),
		NewFeatureStatusCmd(app),
		NewFeatureResumeCmd(app),
	)

	return cmd
}
