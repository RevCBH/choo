package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

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

	// Provider contains provider selection and configuration
	Provider ProviderConfig `yaml:"provider"`

	// Claude contains Claude CLI settings (legacy, still supported)
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

// ProviderType represents a supported LLM provider for task execution.
type ProviderType string

const (
	ProviderClaude ProviderType = "claude"
	ProviderCodex  ProviderType = "codex"
)

// ProviderConfig holds settings for provider selection and configuration.
type ProviderConfig struct {
	// Type is the default provider type: "claude" (default) or "codex"
	Type ProviderType `yaml:"type"`

	// Command overrides the default CLI binary path for the primary provider
	// Deprecated: use Providers map instead
	Command string `yaml:"command,omitempty"`

	// Providers contains per-provider settings
	Providers map[ProviderType]ProviderSettings `yaml:"providers,omitempty"`
}

// ProviderSettings holds configuration for a specific provider.
type ProviderSettings struct {
	// Command is the CLI binary path or name for this provider
	Command string `yaml:"command"`
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

// LoadConfig loads configuration from the repository root.
// It applies defaults, then file values, then environment overrides,
// then validates and auto-detects values.
//
// Parameters:
//   - repoRoot: absolute path to the repository root directory
//
// Returns the validated Config or an error if validation fails.
func LoadConfig(repoRoot string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load config file (optional)
	configPath := filepath.Join(repoRoot, ".choo.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	// Note: missing config file is not an error (use defaults)

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Resolve relative paths
	if !filepath.IsAbs(cfg.Worktree.BasePath) {
		cfg.Worktree.BasePath = filepath.Join(repoRoot, cfg.Worktree.BasePath)
	}

	// Auto-detect GitHub owner/repo if set to "auto"
	if cfg.GitHub.Owner == "auto" || cfg.GitHub.Repo == "auto" {
		owner, repo, err := detectGitHubRepo(repoRoot)
		if err != nil {
			return nil, fmt.Errorf("auto-detect github: %w", err)
		}
		if cfg.GitHub.Owner == "auto" {
			cfg.GitHub.Owner = owner
		}
		if cfg.GitHub.Repo == "auto" {
			cfg.GitHub.Repo = repo
		}
	}

	// Validate
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
