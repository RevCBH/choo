---
task: 2
status: complete
backpressure: "go test ./internal/config/... -run TestDefault"
depends_on: [1]
---

# Config Defaults

**Parent spec**: `/specs/CONFIG.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement default value constants and the DefaultConfig() factory function.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: all Config types)

### Package Dependencies
- None (standard library only)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── defaults.go    # CREATE: Default constants and factory
```

### Constants to Implement

```go
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
```

### Functions to Implement

```go
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

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run TestDefault
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDefaultConfig_TargetBranch` | `cfg.TargetBranch == "main"` |
| `TestDefaultConfig_Parallelism` | `cfg.Parallelism == 4` |
| `TestDefaultConfig_GitHubAuto` | `cfg.GitHub.Owner == "auto" && cfg.GitHub.Repo == "auto"` |
| `TestDefaultConfig_WorktreePath` | `cfg.Worktree.BasePath == ".ralph/worktrees/"` |
| `TestDefaultConfig_Claude` | `cfg.Claude.Command == "claude" && cfg.Claude.MaxTurns == 0` |
| `TestDefaultConfig_Merge` | `cfg.Merge.MaxConflictRetries == 3` |
| `TestDefaultConfig_Review` | `cfg.Review.Timeout == "2h" && cfg.Review.PollInterval == "30s"` |
| `TestDefaultConfig_LogLevel` | `cfg.LogLevel == "info"` |

### Test File to Create

```
internal/
└── config/
    └── defaults_test.go    # CREATE: Tests for defaults
```

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- DefaultConfig returns a pointer to avoid copying large struct
- GitHub defaults to "auto" which triggers detection in LoadConfig
- Empty slices for SetupCommands, TeardownCommands, BaselineChecks are fine (nil is valid)

## NOT In Scope

- LoadConfig function (Task #6)
- Environment variable handling (Task #3)
- Validation logic (Task #4)
- GitHub detection (Task #5)
