---
task: 4
status: pending
backpressure: "go test ./internal/config/... -run TestValidat"
depends_on: [1]
---

# Config Validation

**Parent spec**: `/specs/CONFIG.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement configuration validation logic with structured error reporting.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: Config types)

### Package Dependencies
- `errors` (standard library)
- `fmt` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── validate.go    # CREATE: Validation logic
```

### Types to Implement

```go
// ValidationError contains details about what failed validation.
type ValidationError struct {
    Field   string
    Value   any
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("config.%s: %s (got: %v)", e.Field, e.Message, e.Value)
}
```

### Functions to Implement

```go
// validateConfig checks all config values for validity.
// Returns nil if valid, or joined errors for all validation failures.
func validateConfig(cfg *Config) error
```

### Validation Rules

| Field | Rule |
|-------|------|
| `Parallelism` | Must be >= 1 |
| `GitHub.Owner` | Must not be empty or "auto" after detection |
| `GitHub.Repo` | Must not be empty or "auto" after detection |
| `Claude.Command` | Must not be empty |
| `Claude.MaxTurns` | Must be >= 0 (0 = unlimited) |
| `Merge.MaxConflictRetries` | Must be >= 1 |
| `Review.Timeout` | Must be valid Go duration string |
| `Review.PollInterval` | Must be valid Go duration string |
| `LogLevel` | Must be one of: debug, info, warn, error |
| `BaselineChecks[].Name` | Must not be empty |
| `BaselineChecks[].Command` | Must not be empty |
| `Worktree.SetupCommands[].Command` | Must not be empty |
| `Worktree.TeardownCommands[].Command` | Must not be empty |

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run TestValidat
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestValidation_Parallelism_Zero` | Error contains "parallelism" when value is 0 |
| `TestValidation_Parallelism_Negative` | Error contains "parallelism" when value is -1 |
| `TestValidation_Parallelism_Valid` | No error when value is 1 or higher |
| `TestValidation_GitHubOwner_Empty` | Error contains "github.owner" when empty |
| `TestValidation_GitHubOwner_Auto` | Error contains "github.owner" when still "auto" |
| `TestValidation_GitHubRepo_Empty` | Error contains "github.repo" when empty |
| `TestValidation_ClaudeCommand_Empty` | Error contains "claude.command" when empty |
| `TestValidation_MaxTurns_Negative` | Error contains "claude.max_turns" when negative |
| `TestValidation_MaxConflictRetries_Zero` | Error contains "max_conflict_retries" when 0 |
| `TestValidation_ReviewTimeout_Invalid` | Error contains "review.timeout" for invalid duration |
| `TestValidation_ReviewPollInterval_Invalid` | Error contains "review.poll_interval" for invalid duration |
| `TestValidation_LogLevel_Invalid` | Error contains "log_level" for invalid level |
| `TestValidation_LogLevel_CaseSensitive` | "DEBUG" is invalid (must be lowercase) |
| `TestValidation_BaselineCheck_EmptyName` | Error contains "baseline_checks[0].name" |
| `TestValidation_BaselineCheck_EmptyCommand` | Error contains "baseline_checks[0].command" |
| `TestValidation_SetupCommand_Empty` | Error contains "worktree.setup[0].command" |
| `TestValidation_TeardownCommand_Empty` | Error contains "worktree.teardown[0].command" |
| `TestValidation_MultipleErrors` | Multiple errors are joined together |
| `TestValidation_ValidConfig` | No error for fully valid config |

### Test File to Create

```
internal/
└── config/
    └── validate_test.go    # CREATE: Tests for validation
```

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `errors.Join()` to collect multiple validation errors
- ValidationError implements `error` interface
- Log levels are case-sensitive (lowercase only)
- Duration validation uses `time.ParseDuration()`
- Validate all fields even if early ones fail (collect all errors)

## NOT In Scope

- LoadConfig function (Task #6)
- Default values (Task #2)
- Environment variable handling (Task #3)
- GitHub detection (Task #5)
