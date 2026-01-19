# CONFIG - Configuration Loading and Validation

## Overview

The config package handles loading, validating, and providing access to Ralph Orchestrator configuration. Configuration comes from three sources with a clear precedence order: environment variables override config file values, which override built-in defaults.

The configuration system is designed to work out-of-the-box with sensible defaults while allowing customization for different project structures and workflows. GitHub owner/repo are auto-detected from git remotes, worktree paths default to the repository's `.ralph/worktrees/` directory, and baseline checks can be configured per-project.

```
┌─────────────────────────────────────────────────────────────────┐
│                     Configuration Sources                        │
├─────────────────────┬─────────────────────┬─────────────────────┤
│   Environment Vars  │   Config File       │   Defaults          │
│   (highest priority)│   (.choo.yaml)│   (lowest priority) │
└─────────────────────┴─────────────────────┴─────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │   LoadConfig()  │
                    │                 │
                    │  1. Load defaults
                    │  2. Parse YAML   │
                    │  3. Apply env    │
                    │  4. Validate     │
                    │  5. Auto-detect  │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │     Config      │
                    │                 │
                    │  Immutable      │
                    │  Validated      │
                    │  Ready to use   │
                    └─────────────────┘
```

## Requirements

### Functional Requirements

1. Load configuration from `.choo.yaml` in the repository root
2. Support environment variable overrides for sensitive/dynamic values
3. Provide sensible defaults for all optional configuration
4. Auto-detect GitHub owner/repo from git remote origin
5. Validate all configuration values at load time
6. Support conditional worktree setup/teardown commands based on file existence

### Performance Requirements

| Metric | Target |
|--------|--------|
| Config load time | <50ms |
| Validation time | <10ms |

### Constraints

- Config file is optional; system works with defaults and environment
- GitHub auto-detection requires a valid git remote named "origin"
- Environment variables always take precedence over file values
- Config is immutable after loading (no runtime modifications)

## Design

### File Structure

```
internal/
└── config/
    ├── config.go       # Core types and LoadConfig()
    ├── defaults.go     # Default value constants
    ├── env.go          # Environment variable parsing
    ├── validate.go     # Validation logic
    └── github.go       # GitHub owner/repo auto-detection
```

### Core Types

```go
// internal/config/config.go

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
    // These run in the worktree directory.
    SetupCommands []ConditionalCommand `yaml:"setup"`

    // TeardownCommands are executed before worktree removal.
    // These run in the worktree directory.
    TeardownCommands []ConditionalCommand `yaml:"teardown"`
}

// ConditionalCommand is a command that may be conditional on file existence.
type ConditionalCommand struct {
    // Command is the shell command to execute
    Command string `yaml:"command"`

    // If is an optional file path; if set, command only runs if file exists
    // Path is relative to the worktree root
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
    // Empty pattern means always run
    Pattern string `yaml:"pattern,omitempty"`
}

// MergeConfig controls merge behavior.
type MergeConfig struct {
    // MaxConflictRetries is how many times to attempt conflict resolution
    // before failing the unit
    MaxConflictRetries int `yaml:"max_conflict_retries"`
}

// ReviewConfig controls PR review polling.
type ReviewConfig struct {
    // Timeout is the maximum time to wait for review approval.
    // After this duration, the unit is marked as needing attention.
    // Format: Go duration string (e.g., "2h", "30m")
    Timeout string `yaml:"timeout"`

    // PollInterval is how often to check for review status.
    // Format: Go duration string (e.g., "30s", "1m")
    PollInterval string `yaml:"poll_interval"`
}
```

### YAML Configuration File Structure

```yaml
# .choo.yaml - Full example with all options

target_branch: main
parallelism: 4
log_level: info

github:
  owner: auto        # Auto-detect from git remote
  repo: auto         # Auto-detect from git remote

worktree:
  base_path: .ralph/worktrees/   # Relative to repo root

  # Setup commands run after worktree creation
  setup:
    # Global command - always runs
    - command: "npm install"

    # Conditional command - only runs if file exists
    - command: "cd backend && go mod download"
      if: "backend/go.mod"

    - command: "cd frontend && pnpm install"
      if: "frontend/package.json"

  # Teardown commands run before worktree removal
  teardown:
    - command: "rm -rf node_modules"

claude:
  command: claude    # Or absolute path: /usr/local/bin/claude
  max_turns: 0       # 0 = unlimited (Claude's default)

baseline_checks:
  - name: go-fmt
    command: "go fmt ./..."
    pattern: "*.go"

  - name: go-vet
    command: "go vet ./..."
    pattern: "*.go"

  - name: typescript
    command: "pnpm typecheck"
    pattern: "*.ts,*.tsx"

  - name: rust-clippy
    command: "cd src-tauri && cargo clippy -- -D warnings"
    pattern: "*.rs"

merge:
  max_conflict_retries: 3

review:
  timeout: 2h
  poll_interval: 30s
```

### Default Values

```go
// internal/config/defaults.go

const (
    DefaultTargetBranch       = "main"
    DefaultParallelism        = 4
    DefaultWorktreeBasePath   = ".ralph/worktrees/"
    DefaultClaudeCommand      = "claude"
    DefaultClaudeMaxTurns     = 0  // unlimited
    DefaultMaxConflictRetries = 3
    DefaultReviewTimeout      = "2h"
    DefaultReviewPollInterval = "30s"
    DefaultLogLevel           = "info"
)

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
        LogLevel: DefaultLogLevel,
    }
}
```

### Environment Variable Mapping

| Variable | Config Field | Description |
|----------|--------------|-------------|
| `GITHUB_TOKEN` | (used by github package) | GitHub API authentication |
| `RALPH_CLAUDE_CMD` | `Claude.Command` | Claude CLI binary path |
| `RALPH_WORKTREE_BASE` | `Worktree.BasePath` | Worktree directory |
| `RALPH_LOG_LEVEL` | `LogLevel` | Log verbosity |

```go
// internal/config/env.go

// envOverrides maps environment variables to config field setters.
var envOverrides = []struct {
    envVar  string
    apply   func(*Config, string)
}{
    {
        envVar: "RALPH_CLAUDE_CMD",
        apply: func(c *Config, v string) {
            c.Claude.Command = v
        },
    },
    {
        envVar: "RALPH_WORKTREE_BASE",
        apply: func(c *Config, v string) {
            c.Worktree.BasePath = v
        },
    },
    {
        envVar: "RALPH_LOG_LEVEL",
        apply: func(c *Config, v string) {
            c.LogLevel = v
        },
    },
}

// applyEnvOverrides modifies config in place with environment variable values.
func applyEnvOverrides(cfg *Config) {
    for _, override := range envOverrides {
        if val := os.Getenv(override.envVar); val != "" {
            override.apply(cfg, val)
        }
    }
}
```

### API Surface

```go
// internal/config/config.go

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

// ParseDuration parses a duration string from the config.
// This is a helper for config consumers.
func (c *Config) ReviewTimeoutDuration() (time.Duration, error) {
    return time.ParseDuration(c.Review.Timeout)
}

// ReviewPollIntervalDuration returns the poll interval as a Duration.
func (c *Config) ReviewPollIntervalDuration() (time.Duration, error) {
    return time.ParseDuration(c.Review.PollInterval)
}
```

### Validation Logic

```go
// internal/config/validate.go

// ValidationError contains details about what failed validation.
type ValidationError struct {
    Field   string
    Value   any
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("config.%s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// validateConfig checks all config values for validity.
func validateConfig(cfg *Config) error {
    var errs []error

    // Parallelism must be positive
    if cfg.Parallelism < 1 {
        errs = append(errs, &ValidationError{
            Field:   "parallelism",
            Value:   cfg.Parallelism,
            Message: "must be at least 1",
        })
    }

    // GitHub values must not be empty after auto-detection
    if cfg.GitHub.Owner == "" || cfg.GitHub.Owner == "auto" {
        errs = append(errs, &ValidationError{
            Field:   "github.owner",
            Value:   cfg.GitHub.Owner,
            Message: "must be set or auto-detectable",
        })
    }
    if cfg.GitHub.Repo == "" || cfg.GitHub.Repo == "auto" {
        errs = append(errs, &ValidationError{
            Field:   "github.repo",
            Value:   cfg.GitHub.Repo,
            Message: "must be set or auto-detectable",
        })
    }

    // Claude command must not be empty
    if cfg.Claude.Command == "" {
        errs = append(errs, &ValidationError{
            Field:   "claude.command",
            Value:   cfg.Claude.Command,
            Message: "must not be empty",
        })
    }

    // MaxTurns must be non-negative
    if cfg.Claude.MaxTurns < 0 {
        errs = append(errs, &ValidationError{
            Field:   "claude.max_turns",
            Value:   cfg.Claude.MaxTurns,
            Message: "must be non-negative (0 = unlimited)",
        })
    }

    // MaxConflictRetries must be positive
    if cfg.Merge.MaxConflictRetries < 1 {
        errs = append(errs, &ValidationError{
            Field:   "merge.max_conflict_retries",
            Value:   cfg.Merge.MaxConflictRetries,
            Message: "must be at least 1",
        })
    }

    // Review timeout must be valid duration
    if _, err := time.ParseDuration(cfg.Review.Timeout); err != nil {
        errs = append(errs, &ValidationError{
            Field:   "review.timeout",
            Value:   cfg.Review.Timeout,
            Message: fmt.Sprintf("invalid duration: %v", err),
        })
    }

    // Review poll interval must be valid duration
    if _, err := time.ParseDuration(cfg.Review.PollInterval); err != nil {
        errs = append(errs, &ValidationError{
            Field:   "review.poll_interval",
            Value:   cfg.Review.PollInterval,
            Message: fmt.Sprintf("invalid duration: %v", err),
        })
    }

    // Log level must be valid
    validLogLevels := map[string]bool{
        "debug": true, "info": true, "warn": true, "error": true,
    }
    if !validLogLevels[cfg.LogLevel] {
        errs = append(errs, &ValidationError{
            Field:   "log_level",
            Value:   cfg.LogLevel,
            Message: "must be one of: debug, info, warn, error",
        })
    }

    // Baseline checks must have name and command
    for i, check := range cfg.BaselineChecks {
        if check.Name == "" {
            errs = append(errs, &ValidationError{
                Field:   fmt.Sprintf("baseline_checks[%d].name", i),
                Value:   check.Name,
                Message: "must not be empty",
            })
        }
        if check.Command == "" {
            errs = append(errs, &ValidationError{
                Field:   fmt.Sprintf("baseline_checks[%d].command", i),
                Value:   check.Command,
                Message: "must not be empty",
            })
        }
    }

    // Setup commands must have command
    for i, cmd := range cfg.Worktree.SetupCommands {
        if cmd.Command == "" {
            errs = append(errs, &ValidationError{
                Field:   fmt.Sprintf("worktree.setup[%d].command", i),
                Value:   cmd.Command,
                Message: "must not be empty",
            })
        }
    }

    // Teardown commands must have command
    for i, cmd := range cfg.Worktree.TeardownCommands {
        if cmd.Command == "" {
            errs = append(errs, &ValidationError{
                Field:   fmt.Sprintf("worktree.teardown[%d].command", i),
                Value:   cmd.Command,
                Message: "must not be empty",
            })
        }
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

### GitHub Auto-Detection

```go
// internal/config/github.go

import (
    "fmt"
    "os/exec"
    "regexp"
    "strings"
)

// detectGitHubRepo extracts owner and repo from the git remote origin URL.
// Supports both HTTPS and SSH URL formats.
func detectGitHubRepo(repoRoot string) (owner, repo string, err error) {
    cmd := exec.Command("git", "remote", "get-url", "origin")
    cmd.Dir = repoRoot
    out, err := cmd.Output()
    if err != nil {
        return "", "", fmt.Errorf("get git remote: %w", err)
    }

    url := strings.TrimSpace(string(out))
    return parseGitHubURL(url)
}

// parseGitHubURL extracts owner/repo from a GitHub URL.
// Supports:
//   - https://github.com/owner/repo.git
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo
//   - git@github.com:owner/repo
func parseGitHubURL(url string) (owner, repo string, err error) {
    // HTTPS format
    httpsRe := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
    if m := httpsRe.FindStringSubmatch(url); m != nil {
        return m[1], m[2], nil
    }

    // SSH format
    sshRe := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
    if m := sshRe.FindStringSubmatch(url); m != nil {
        return m[1], m[2], nil
    }

    return "", "", fmt.Errorf("unrecognized GitHub URL format: %s", url)
}
```

## Implementation Notes

### Config File Location

The config file is always `.choo.yaml` in the repository root. We explicitly do not support:
- XDG config directories
- Home directory config
- Multiple config file names

This keeps the configuration simple and project-scoped.

### Path Resolution

Relative paths in the config (like `worktree.base_path`) are resolved relative to the repository root, not the current working directory. This ensures consistent behavior regardless of where the CLI is invoked from.

### Conditional Commands

The `if` field in setup/teardown commands supports simple file existence checks. The path is relative to the worktree root. This enables polyglot repositories where different setup is needed depending on which parts of the codebase exist.

Example flow:
```go
func (w *Worker) runSetupCommands(worktreePath string) error {
    for _, cmd := range w.config.Worktree.SetupCommands {
        // Check condition if present
        if cmd.If != "" {
            checkPath := filepath.Join(worktreePath, cmd.If)
            if _, err := os.Stat(checkPath); os.IsNotExist(err) {
                // Skip this command - condition file doesn't exist
                continue
            }
        }

        // Run the command
        if err := w.runCommand(worktreePath, cmd.Command); err != nil {
            return fmt.Errorf("setup command failed: %s: %w", cmd.Command, err)
        }
    }
    return nil
}
```

### Environment Variable Precedence

Environment variables are applied after the config file is loaded, ensuring they always take precedence. This is the standard pattern for 12-factor apps and allows:
- CI/CD to override values without modifying files
- Local development to use different paths
- Sensitive values (like custom Claude paths) to stay out of version control

### Immutability

The Config struct is treated as immutable after `LoadConfig()` returns. There are no setter methods. If runtime configuration changes are needed in the future, we would introduce a separate mechanism (like a ConfigWatcher).

## Testing Strategy

### Unit Tests

```go
// internal/config/config_test.go

func TestLoadConfig_Defaults(t *testing.T) {
    // Create temp repo with no config file
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot, "https://github.com/test/repo.git")

    cfg, err := LoadConfig(repoRoot)
    require.NoError(t, err)

    assert.Equal(t, "main", cfg.TargetBranch)
    assert.Equal(t, 4, cfg.Parallelism)
    assert.Equal(t, "claude", cfg.Claude.Command)
    assert.Equal(t, 0, cfg.Claude.MaxTurns)
    assert.Equal(t, 3, cfg.Merge.MaxConflictRetries)
    assert.Equal(t, "2h", cfg.Review.Timeout)
    assert.Equal(t, "test", cfg.GitHub.Owner)
    assert.Equal(t, "repo", cfg.GitHub.Repo)
}

func TestLoadConfig_FileOverrides(t *testing.T) {
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot, "https://github.com/test/repo.git")

    configYAML := `
target_branch: develop
parallelism: 8
claude:
  max_turns: 50
`
    writeFile(t, filepath.Join(repoRoot, ".choo.yaml"), configYAML)

    cfg, err := LoadConfig(repoRoot)
    require.NoError(t, err)

    assert.Equal(t, "develop", cfg.TargetBranch)
    assert.Equal(t, 8, cfg.Parallelism)
    assert.Equal(t, 50, cfg.Claude.MaxTurns)
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot, "https://github.com/test/repo.git")

    configYAML := `
claude:
  command: file-claude
`
    writeFile(t, filepath.Join(repoRoot, ".choo.yaml"), configYAML)

    t.Setenv("RALPH_CLAUDE_CMD", "/custom/claude")

    cfg, err := LoadConfig(repoRoot)
    require.NoError(t, err)

    // Env should override file
    assert.Equal(t, "/custom/claude", cfg.Claude.Command)
}

func TestLoadConfig_PathResolution(t *testing.T) {
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot, "https://github.com/test/repo.git")

    configYAML := `
worktree:
  base_path: .ralph/worktrees/
`
    writeFile(t, filepath.Join(repoRoot, ".choo.yaml"), configYAML)

    cfg, err := LoadConfig(repoRoot)
    require.NoError(t, err)

    // Should be resolved to absolute path
    expected := filepath.Join(repoRoot, ".ralph/worktrees/")
    assert.Equal(t, expected, cfg.Worktree.BasePath)
}
```

```go
// internal/config/validate_test.go

func TestValidation_Parallelism(t *testing.T) {
    tests := []struct {
        name        string
        parallelism int
        wantErr     bool
    }{
        {"zero", 0, true},
        {"negative", -1, true},
        {"one", 1, false},
        {"many", 100, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := DefaultConfig()
            cfg.Parallelism = tt.parallelism
            cfg.GitHub.Owner = "test"
            cfg.GitHub.Repo = "repo"

            err := validateConfig(cfg)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), "parallelism")
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestValidation_LogLevel(t *testing.T) {
    tests := []struct {
        level   string
        wantErr bool
    }{
        {"debug", false},
        {"info", false},
        {"warn", false},
        {"error", false},
        {"DEBUG", true},  // Case-sensitive
        {"trace", true},
        {"", true},
    }

    for _, tt := range tests {
        t.Run(tt.level, func(t *testing.T) {
            cfg := DefaultConfig()
            cfg.LogLevel = tt.level
            cfg.GitHub.Owner = "test"
            cfg.GitHub.Repo = "repo"

            err := validateConfig(cfg)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestValidation_Duration(t *testing.T) {
    tests := []struct {
        timeout string
        wantErr bool
    }{
        {"2h", false},
        {"30m", false},
        {"1h30m", false},
        {"30s", false},
        {"invalid", true},
        {"", true},
        {"2", true},  // Needs unit
    }

    for _, tt := range tests {
        t.Run(tt.timeout, func(t *testing.T) {
            cfg := DefaultConfig()
            cfg.Review.Timeout = tt.timeout
            cfg.GitHub.Owner = "test"
            cfg.GitHub.Repo = "repo"

            err := validateConfig(cfg)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestValidation_BaselineChecks(t *testing.T) {
    cfg := DefaultConfig()
    cfg.GitHub.Owner = "test"
    cfg.GitHub.Repo = "repo"
    cfg.BaselineChecks = []BaselineCheck{
        {Name: "", Command: "go fmt"},  // Missing name
    }

    err := validateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "baseline_checks[0].name")
}
```

```go
// internal/config/github_test.go

func TestParseGitHubURL(t *testing.T) {
    tests := []struct {
        url       string
        wantOwner string
        wantRepo  string
        wantErr   bool
    }{
        {
            url:       "https://github.com/anthropics/choo.git",
            wantOwner: "anthropics",
            wantRepo:  "choo",
        },
        {
            url:       "https://github.com/anthropics/choo",
            wantOwner: "anthropics",
            wantRepo:  "choo",
        },
        {
            url:       "git@github.com:anthropics/choo.git",
            wantOwner: "anthropics",
            wantRepo:  "choo",
        },
        {
            url:       "git@github.com:anthropics/choo",
            wantOwner: "anthropics",
            wantRepo:  "choo",
        },
        {
            url:     "https://gitlab.com/owner/repo.git",
            wantErr: true,
        },
        {
            url:     "not-a-url",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.url, func(t *testing.T) {
            owner, repo, err := parseGitHubURL(tt.url)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.wantOwner, owner)
                assert.Equal(t, tt.wantRepo, repo)
            }
        })
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| No config file | Empty git repo with remote, verify defaults |
| Full config file | Complete `.choo.yaml`, verify all fields |
| Partial config file | Only some fields set, verify merge with defaults |
| Environment overrides | Set env vars, verify precedence over file |
| SSH remote URL | Git repo with SSH origin, verify detection |
| HTTPS remote URL | Git repo with HTTPS origin, verify detection |

### Manual Testing

- [ ] Create `.choo.yaml` with all fields, run `choo status`
- [ ] Remove config file, verify defaults work
- [ ] Set `RALPH_CLAUDE_CMD` to invalid path, verify error message
- [ ] Set invalid `review.timeout` value, verify validation error
- [ ] Test with SSH remote URL in fresh clone
- [ ] Test with HTTPS remote URL in fresh clone

## Design Decisions

### Why YAML Over TOML or JSON?

YAML was chosen because:
- Human-readable with support for comments
- Common in DevOps tooling (GitHub Actions, Docker Compose)
- Good support for nested structures
- `gopkg.in/yaml.v3` is mature and widely used

TOML was considered but rejected because:
- Less common in the Go ecosystem
- Nesting syntax is less intuitive for deep structures

JSON was rejected because:
- No comment support
- Verbose for configuration files

### Why Auto-Detection for GitHub?

Requiring explicit GitHub owner/repo configuration would:
- Create friction for new users
- Duplicate information already in git config
- Break when repos are forked

Auto-detection from git remote is the standard pattern used by GitHub CLI and similar tools.

### Why File Existence for Conditional Commands?

The `if` field supports only file existence, not arbitrary shell conditions, because:
- Simple to implement and reason about
- Covers the main use case (polyglot repos with optional components)
- Shell conditions would be a security concern
- More complex conditions can use the shell itself: `if: "Makefile"` with `command: "test -f something && make setup"`

### Why Immutable Config?

The Config struct is immutable after loading because:
- Eliminates race conditions in concurrent code
- Makes reasoning about config state simpler
- Matches the principle of loading once at startup
- If hot-reload is needed later, it can be a separate feature with explicit API

## Future Enhancements

1. **Config validation CLI command**: `choo config validate` to check config without running
2. **Config generation**: `choo config init` to generate a starter config file
3. **Per-unit overrides**: Allow units to override global config (e.g., different baseline checks)
4. **Profile support**: Named configuration profiles for different environments
5. **Remote config**: Support loading config from a URL or gist

## References

- [PRD Section 9: Configuration](./MVP%20DESIGN%20SPEC.md#9-configuration)
- [Go yaml.v3 documentation](https://pkg.go.dev/gopkg.in/yaml.v3)
- [12-Factor App: Config](https://12factor.net/config)
