---
task: 1
status: pending
backpressure: "go build ./internal/config/..."
depends_on: []
---

# CodeReviewConfig Types and Defaults

**Parent spec**: `/specs/REVIEW-CONFIG.md`
**Task**: #1 of 2 in implementation plan

## Objective

Define the `CodeReviewConfig` struct, `ReviewProviderType` type, default values, and helper methods for the advisory code review system.

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
    ├── config.go       # MODIFY: Add CodeReviewConfig to Config struct
    └── defaults.go     # MODIFY: Add DefaultCodeReviewConfig() and constants
```

### Types to Implement

```go
// internal/config/config.go

// ReviewProviderType represents a code review provider.
// Note: This is separate from ProviderType (task execution) to allow
// independent provider selection for review vs task execution.
type ReviewProviderType string

const (
    // ReviewProviderCodex uses OpenAI Codex for code review.
    ReviewProviderCodex ReviewProviderType = "codex"

    // ReviewProviderClaude uses Anthropic Claude for code review.
    ReviewProviderClaude ReviewProviderType = "claude"
)

// CodeReviewConfig controls the advisory code review system.
type CodeReviewConfig struct {
    // Enabled controls whether code review runs. Default: true.
    // When enabled, review runs after each unit completes AND after
    // all units merge to the feature branch (before final rebase/merge).
    Enabled bool `yaml:"enabled"`

    // Provider specifies which reviewer to use: "codex" or "claude".
    // Default: "codex".
    Provider ReviewProviderType `yaml:"provider"`

    // MaxFixIterations limits how many times the system attempts fixes
    // per review cycle. Default: 1 (single review-fix cycle).
    // Set to 0 to disable fix attempts (review-only mode).
    MaxFixIterations int `yaml:"max_fix_iterations"`

    // Verbose controls output verbosity. Default: true (noisy).
    // When true, review findings are printed to stderr even when passing.
    // When false, only issues requiring attention are printed.
    Verbose bool `yaml:"verbose"`

    // Command overrides the CLI path for the reviewer.
    // Default: "" (uses system PATH to find "codex" or "claude").
    Command string `yaml:"command,omitempty"`
}

// IsReviewOnlyMode returns true if fixes are disabled (MaxFixIterations == 0).
func (c *CodeReviewConfig) IsReviewOnlyMode() bool {
    return c.MaxFixIterations == 0
}
```

Add `CodeReview` field to the existing `Config` struct:

```go
// Config holds all configuration for the Ralph Orchestrator.
type Config struct {
    // ... existing fields ...

    // CodeReview configures the advisory code review system.
    CodeReview CodeReviewConfig `yaml:"code_review"`

    // ... rest of existing fields ...
}
```

### Constants to Add

```go
// internal/config/defaults.go

const (
    // ... existing constants ...

    DefaultCodeReviewEnabled          = true
    DefaultCodeReviewProvider         = ReviewProviderCodex
    DefaultCodeReviewMaxFixIterations = 1
    DefaultCodeReviewVerbose          = true
    DefaultCodeReviewCommand          = ""
)
```

### Functions to Implement

```go
// internal/config/defaults.go

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
```

Update `DefaultConfig()` to include CodeReview:

```go
// DefaultConfig returns a Config with all default values applied.
func DefaultConfig() *Config {
    return &Config{
        // ... existing field initializations ...
        CodeReview: DefaultCodeReviewConfig(),
        // ... rest of existing field initializations ...
    }
}
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
| Type completeness | All CodeReviewConfig fields have yaml tags |
| ReviewProviderType | Both constants (codex, claude) are defined |
| IsReviewOnlyMode | Method exists on CodeReviewConfig |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- `ReviewProviderType` is deliberately separate from `ProviderType` to allow independent selection
- The YAML tag for CodeReview is `code_review` (snake_case) per spec
- `Verbose` defaults to true for "default on and noisy" behavior per spec
- Provider defaults to "codex" per spec (different from task execution default of "claude")
- Place CodeReview field in Config struct after Feature field (near end)

## NOT In Scope

- Validation logic (Task #2)
- Integration with LoadConfig (Task #2)
- Orchestrator's resolveReviewer() method (separate spec: REVIEW-PROVIDER)
