---
task: 3
status: pending
backpressure: "go test ./internal/config/... -run TestEnv"
depends_on: [1]
---

# Environment Variable Overrides

**Parent spec**: `/specs/CONFIG.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement environment variable parsing and application to override config file values.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: Config types)

### Package Dependencies
- `os` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── env.go    # CREATE: Environment variable handling
```

### Environment Variable Mapping

| Variable | Config Field | Description |
|----------|--------------|-------------|
| `RALPH_CLAUDE_CMD` | `Claude.Command` | Claude CLI binary path |
| `RALPH_WORKTREE_BASE` | `Worktree.BasePath` | Worktree directory |
| `RALPH_LOG_LEVEL` | `LogLevel` | Log verbosity |

### Code to Implement

```go
// envOverrides maps environment variables to config field setters.
var envOverrides = []struct {
    envVar string
    apply  func(*Config, string)
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

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run TestEnv
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestEnvOverrides_ClaudeCmd` | When `RALPH_CLAUDE_CMD=/custom/claude`, `cfg.Claude.Command == "/custom/claude"` |
| `TestEnvOverrides_WorktreeBase` | When `RALPH_WORKTREE_BASE=/tmp/worktrees`, `cfg.Worktree.BasePath == "/tmp/worktrees"` |
| `TestEnvOverrides_LogLevel` | When `RALPH_LOG_LEVEL=debug`, `cfg.LogLevel == "debug"` |
| `TestEnvOverrides_EmptyNoChange` | When env var is empty, original value preserved |
| `TestEnvOverrides_MultipleVars` | Multiple env vars apply correctly |

### Test File to Create

```
internal/
└── config/
    └── env_test.go    # CREATE: Tests for env overrides
```

### Test Implementation Notes

Use `t.Setenv()` for setting environment variables in tests - it automatically cleans up after each test.

```go
func TestEnvOverrides_ClaudeCmd(t *testing.T) {
    cfg := &Config{Claude: ClaudeConfig{Command: "original"}}
    t.Setenv("RALPH_CLAUDE_CMD", "/custom/claude")

    applyEnvOverrides(cfg)

    assert.Equal(t, "/custom/claude", cfg.Claude.Command)
}
```

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Environment variables always override file values (applied after YAML parsing)
- Empty string env vars are treated as "not set" (do not override)
- Function modifies config in place (pointer receiver pattern)

## NOT In Scope

- LoadConfig function (Task #6)
- Default values (handled in Task #2)
- Validation logic (Task #4)
- GitHub detection (Task #5)
- GITHUB_TOKEN handling (used by github package, not config)
