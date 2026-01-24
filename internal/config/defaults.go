package config

const (
	DefaultTargetBranch       = "main"
	DefaultParallelism        = 4
	DefaultWorktreeBasePath   = ".ralph/worktrees/"
	DefaultClaudeCommand      = "claude"
	DefaultClaudeMaxTurns     = 0 // unlimited
	DefaultCodexCommand       = "codex"
	DefaultMaxConflictRetries = 3
	DefaultReviewTimeout      = "2h"
	DefaultReviewPollInterval = "30s"
	DefaultLogLevel           = "info"
	DefaultPRDDir             = "docs/prd"
	DefaultSpecsDir           = "specs"
	DefaultBranchPrefix       = "feature/"

	DefaultCodeReviewEnabled          = true
	DefaultCodeReviewProvider         = ReviewProviderCodex
	DefaultCodeReviewMaxFixIterations = 1
	DefaultCodeReviewVerbose          = true
	DefaultCodeReviewCommand          = ""

	DefaultSpecRepairCommand = ""
	DefaultSpecRepairModel   = ""
	DefaultSpecRepairTimeout = "30s"
)

// DefaultProviderType is the default provider when none is specified
var DefaultProviderType ProviderType = ProviderClaude

// DefaultProviderConfig returns provider config with default values.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Type:      DefaultProviderType,
		Providers: make(map[ProviderType]ProviderSettings),
	}
}

// DefaultFeatureConfig returns sensible defaults for feature configuration.
func DefaultFeatureConfig() FeatureConfig {
	return FeatureConfig{
		PRDDir:       DefaultPRDDir,
		SpecsDir:     DefaultSpecsDir,
		BranchPrefix: DefaultBranchPrefix,
	}
}

// DefaultCodeReviewConfig returns sensible defaults for code review.
// Defaults are: enabled, codex provider, 1 fix iteration, verbose output.
func DefaultCodeReviewConfig() CodeReviewConfig {
	return CodeReviewConfig{
		Enabled:          DefaultCodeReviewEnabled,
		Provider:         DefaultCodeReviewProvider,
		MaxFixIterations: DefaultCodeReviewMaxFixIterations,
		Verbose:          DefaultCodeReviewVerbose,
		Command:          DefaultCodeReviewCommand,
	}
}

// DefaultSpecRepairConfig returns defaults for spec repair.
func DefaultSpecRepairConfig() SpecRepairConfig {
	return SpecRepairConfig{
		Provider: DefaultProviderType,
		Command:  DefaultSpecRepairCommand,
		Model:    DefaultSpecRepairModel,
		Timeout:  DefaultSpecRepairTimeout,
	}
}

// DefaultConfig returns a Config with all default values applied.
func DefaultConfig() *Config {
	return &Config{
		TargetBranch: DefaultTargetBranch,
		Parallelism:  DefaultParallelism,
		GitHub: GitHubConfig{
			Owner: "auto",
			Repo:  "auto",
		},
		Worktree: WorktreeConfig{
			BasePath:   DefaultWorktreeBasePath,
			ResetOnRun: false,
		},
		Provider: DefaultProviderConfig(),
		Claude: ClaudeConfig{
			Command:  DefaultClaudeCommand,
			MaxTurns: DefaultClaudeMaxTurns,
		},
		Merge: MergeConfig{
			MaxConflictRetries: DefaultMaxConflictRetries,
		},
		Review: ReviewConfig{
			Timeout:      DefaultReviewTimeout,
			PollInterval: DefaultReviewPollInterval,
		},
		Feature:    DefaultFeatureConfig(),
		CodeReview: DefaultCodeReviewConfig(),
		SpecRepair: DefaultSpecRepairConfig(),
		LogLevel:   DefaultLogLevel,
	}
}
