package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd creates the version command
func NewVersionCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			version := app.versionInfo.Version
			commit := app.versionInfo.Commit
			date := app.versionInfo.Date

			// Use default values if not set
			if version == "" {
				version = "dev"
			}
			if commit == "" {
				commit = "unknown"
			}
			if date == "" {
				date = "unknown"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "choo version %s\n", version)
			fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", commit)
			fmt.Fprintf(cmd.OutOrStdout(), "built: %s\n", date)

			return nil
		},
	}

	return cmd
}
