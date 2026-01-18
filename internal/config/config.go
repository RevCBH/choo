package config

import "time"

// Config holds all configuration for the Ralph Orchestrator.
// It is immutable after creation via LoadConfig().
type Config struct {
	// TargetBranch is the branch PRs will be merged into
	TargetBranch string `yaml:"target_branch"`

	// Parallelism is the maximum number of units to execute concurrently
	Parallelism int `yaml:"parallelism"`

	// GitHub contains repository identification
	GitHub GitHubConfig `yaml:"github"`

	// Worktree contains worktree management settings
	Worktree WorktreeConfig `yaml:"worktree"`

	// Claude contains Claude CLI settings
	Claude ClaudeConfig `yaml:"claude"`

	// BaselineChecks are validation commands run after all tasks complete
	BaselineChecks []BaselineCheck `yaml:"baseline_checks"`

	// Merge contains merge behavior settings
	Merge MergeConfig `yaml:"merge"`

	// Review contains review polling settings
	Review ReviewConfig `yaml:"review"`

	// LogLevel controls log verbosity (debug, info, warn, error)
	LogLevel string `yaml:"log_level"`
}

// GitHubConfig identifies the GitHub repository.
// Values of "auto" trigger detection from git remote.
type GitHubConfig struct {
	// Owner is the GitHub organization or user (e.g., "anthropics")
	Owner string `yaml:"owner"`

	// Repo is the repository name (e.g., "choo")
	Repo string `yaml:"repo"`
}

// WorktreeConfig controls git worktree creation and lifecycle.
type WorktreeConfig struct {
	// BasePath is the directory where worktrees are created.
	// Relative paths are resolved from the repository root.
	BasePath string `yaml:"base_path"`

	// SetupCommands are executed after worktree creation.
	SetupCommands []ConditionalCommand `yaml:"setup"`

	// TeardownCommands are executed before worktree removal.
	TeardownCommands []ConditionalCommand `yaml:"teardown"`
}

// ConditionalCommand is a command that may be conditional on file existence.
type ConditionalCommand struct {
	// Command is the shell command to execute
	Command string `yaml:"command"`

	// If is an optional file path; if set, command only runs if file exists
	If string `yaml:"if,omitempty"`
}

// ClaudeConfig controls Claude CLI invocation.
type ClaudeConfig struct {
	// Command is the path or name of the Claude CLI binary
	Command string `yaml:"command"`

	// MaxTurns limits Claude's agentic loop iterations (0 = unlimited)
	MaxTurns int `yaml:"max_turns"`
}

// BaselineCheck is a validation command run after unit completion.
type BaselineCheck struct {
	// Name identifies this check in logs and errors
	Name string `yaml:"name"`

	// Command is the shell command to execute
	Command string `yaml:"command"`

	// Pattern is a glob pattern; check only runs if matching files changed
	Pattern string `yaml:"pattern,omitempty"`
}

// MergeConfig controls merge behavior.
type MergeConfig struct {
	// MaxConflictRetries is how many times to attempt conflict resolution
	MaxConflictRetries int `yaml:"max_conflict_retries"`
}

// ReviewConfig controls PR review polling.
type ReviewConfig struct {
	// Timeout is the maximum time to wait for review approval
	Timeout string `yaml:"timeout"`

	// PollInterval is how often to check for review status
	PollInterval string `yaml:"poll_interval"`
}

// ReviewTimeoutDuration parses the review timeout as a Duration.
func (c *Config) ReviewTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(c.Review.Timeout)
}

// ReviewPollIntervalDuration returns the poll interval as a Duration.
func (c *Config) ReviewPollIntervalDuration() (time.Duration, error) {
	return time.ParseDuration(c.Review.PollInterval)
}
