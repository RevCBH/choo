package config

const (
	DefaultTargetBranch       = "main"
	DefaultParallelism        = 4
	DefaultWorktreeBasePath   = ".ralph/worktrees/"
	DefaultClaudeCommand      = "claude"
	DefaultClaudeMaxTurns     = 0 // unlimited
	DefaultMaxConflictRetries = 3
	DefaultReviewTimeout      = "2h"
	DefaultReviewPollInterval = "30s"
	DefaultLogLevel           = "info"
	DefaultPRDDir             = "docs/prd"
	DefaultSpecsDir           = "specs"
	DefaultBranchPrefix       = "feature/"
)

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
