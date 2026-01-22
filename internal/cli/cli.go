package cli

import (
	"github.com/RevCBH/choo/internal/feature"
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
	debug    bool
	shutdown chan struct{}

	// Version information
	versionInfo VersionInfo

	// Agent invoker for Claude-powered features (e.g., next-feature)
	agentInvoker feature.AgentInvoker
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

// SetAgentInvoker sets the Claude agent invoker for features that require it
func (a *App) SetAgentInvoker(invoker feature.AgentInvoker) {
	a.agentInvoker = invoker
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
	a.rootCmd.PersistentFlags().BoolVar(&a.debug, "debug", false, "Debug output (includes assistant text in streaming mode)")

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
		NewNextFeatureCmd(a),
		NewFeatureCmd(a),
		NewDaemonCmd(a),
		NewJobsCmd(a),
		NewWatchCmd(a),
		NewStopJobCmd(a),
	)
}
