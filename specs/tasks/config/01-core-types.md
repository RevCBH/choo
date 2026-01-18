---
task: 1
status: complete
backpressure: "go build ./internal/config/..."
depends_on: []
---

# Core Config Types

**Parent spec**: `/specs/CONFIG.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define all configuration struct types used throughout the Ralph Orchestrator.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- None (standard library only for types)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── config.go    # CREATE: Core type definitions
```

### Types to Implement

```go
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
```

### Helper Methods to Implement

```go
// ReviewTimeoutDuration parses the review timeout as a Duration.
func (c *Config) ReviewTimeoutDuration() (time.Duration, error)

// ReviewPollIntervalDuration returns the poll interval as a Duration.
func (c *Config) ReviewPollIntervalDuration() (time.Duration, error)
```

## Backpressure

### Validation Command

```bash
go build ./internal/config/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| Type completeness | All struct fields have yaml tags |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All struct fields must have `yaml` tags for YAML unmarshaling
- Use `omitempty` for optional fields
- Duration helper methods should use `time.ParseDuration`
- Package declaration: `package config`

## NOT In Scope

- LoadConfig function (Task #6)
- Default values (Task #2)
- Environment variable handling (Task #3)
- Validation logic (Task #4)
- GitHub detection (Task #5)
