package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// App represents the CLI application with all wired dependencies
type App struct {
	// Root command
	rootCmd *cobra.Command

	// Configuration (initialized lazily)
	config interface{} // Placeholder for *config.Config

	// Runtime state
	verbose  bool
	cancel   context.CancelFunc
	shutdown chan struct{}

	// Version information
	version string
	commit  string
	date    string
}

// New creates a new CLI application
func New() *App {
	app := &App{
		shutdown: make(chan struct{}),
	}
	app.setupRootCmd()
	return app
}

// Execute runs the CLI application
func (a *App) Execute() error {
	return a.rootCmd.Execute()
}

// SetVersion sets the version string for the version command
func (a *App) SetVersion(version, commit, date string) {
	a.version = version
	a.commit = commit
	a.date = date
}

// setupRootCmd configures the root Cobra command
func (a *App) setupRootCmd() {
	a.rootCmd = &cobra.Command{
		Use:   "choo",
		Short: "Parallel development task orchestrator",
		Long: `Ralph Orchestrator executes development units in parallel,
managing git worktrees and the full PR lifecycle.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add persistent flags
	a.rootCmd.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false,
		"Verbose output")
}
