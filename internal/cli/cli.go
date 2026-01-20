package cli

import (
	"github.com/spf13/cobra"
)

// VersionInfo holds version metadata
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

// App represents the CLI application with all wired dependencies
type App struct {
	// Root command
	rootCmd *cobra.Command

	// Runtime state
	verbose  bool
	shutdown chan struct{}

	// Version information
	versionInfo VersionInfo
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
	a.versionInfo = VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
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

	// Add subcommands
	a.rootCmd.AddCommand(
		NewVersionCmd(a),
		NewStatusCmd(a),
		NewRunCmd(a),
		NewResumeCmd(a),
		NewCleanupCmd(a),
		NewArchiveCmd(a),
		NewWebCmd(a),
		NewPromptCmd(a),
	)
}
