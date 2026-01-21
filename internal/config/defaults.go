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
)

// DefaultProviderType is the default provider when none is specified
var DefaultProviderType ProviderType = ProviderClaude

// DefaultProviderConfig returns provider config with default values.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Type: DefaultProviderType,
		Providers: map[ProviderType]ProviderSettings{
			ProviderClaude: {Command: DefaultClaudeCommand},
			ProviderCodex:  {Command: DefaultCodexCommand},
		},
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
			BasePath: DefaultWorktreeBasePath,
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
		Feature:  DefaultFeatureConfig(),
		LogLevel: DefaultLogLevel,
	}
}
