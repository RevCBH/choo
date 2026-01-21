---
task: 2
status: pending
backpressure: "go test ./internal/config/... -run TestCodeReview"
depends_on: [1]
---

# CodeReviewConfig Validation and Integration

**Parent spec**: `/specs/REVIEW-CONFIG.md`
**Task**: #2 of 2 in implementation plan

## Objective

Implement validation logic for `CodeReviewConfig` and ensure proper integration with the config loading system for backwards compatibility.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: CodeReviewConfig struct, ReviewProviderType, DefaultCodeReviewConfig)

### Package Dependencies
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    ├── validate.go      # MODIFY: Add CodeReviewConfig validation
    ├── validate_test.go # MODIFY: Add CodeReviewConfig validation tests
    ├── config_test.go   # MODIFY: Add DefaultCodeReviewConfig tests
    └── defaults_test.go # MODIFY: Add CodeReviewConfig default tests (optional)
```

### Functions to Implement

```go
// internal/config/config.go (or validate.go)

// Validate checks that the CodeReviewConfig is valid.
// Only validates provider when review is enabled.
func (c *CodeReviewConfig) Validate() error {
    if c.Enabled {
        switch c.Provider {
        case ReviewProviderCodex, ReviewProviderClaude:
            // Valid
        default:
            return fmt.Errorf("invalid review provider: %q (must be 'codex' or 'claude')", c.Provider)
        }
    }

    if c.MaxFixIterations < 0 {
        return fmt.Errorf("max_fix_iterations cannot be negative: %d", c.MaxFixIterations)
    }

    return nil
}
```

### Validation Integration

Add to `validateConfig()` in validate.go:

```go
func validateConfig(cfg *Config) error {
    var errs []error

    // ... existing validation ...

    // CodeReview validation
    if err := cfg.CodeReview.Validate(); err != nil {
        errs = append(errs, &ValidationError{
            Field:   "code_review",
            Value:   cfg.CodeReview.Provider,
            Message: err.Error(),
        })
    }

    // ... rest of existing validation ...
}
```

### Backwards Compatibility

The existing `LoadConfig()` already applies defaults before YAML parsing, so missing `code_review` section will use defaults. Verify this works by testing partial/missing configs.

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -run TestCodeReview
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCodeReviewConfig_Validate_ValidCodex` | No error for enabled=true, provider=codex |
| `TestCodeReviewConfig_Validate_ValidClaude` | No error for enabled=true, provider=claude |
| `TestCodeReviewConfig_Validate_DisabledInvalidProvider` | No error when enabled=false even with invalid provider |
| `TestCodeReviewConfig_Validate_InvalidProvider` | Error for enabled=true, provider="gpt4" |
| `TestCodeReviewConfig_Validate_NegativeIterations` | Error for MaxFixIterations=-1 |
| `TestCodeReviewConfig_Validate_ZeroIterations` | No error for MaxFixIterations=0 (review-only mode) |
| `TestCodeReviewConfig_IsReviewOnlyMode` | Returns true when MaxFixIterations=0, false otherwise |
| `TestDefaultCodeReviewConfig` | Verifies defaults: enabled=true, provider=codex, iterations=1, verbose=true, command="" |
| `TestLoadConfig_MissingCodeReview` | Defaults applied when code_review section missing |
| `TestLoadConfig_PartialCodeReview` | Explicit values used, others get defaults |

### Test File Additions

```go
// internal/config/config_test.go (or new file)

func TestCodeReviewConfig_Validate_ValidCodex(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:          true,
        Provider:         ReviewProviderCodex,
        MaxFixIterations: 1,
    }
    if err := cfg.Validate(); err != nil {
        t.Errorf("Validate() error = %v, want nil", err)
    }
}

func TestCodeReviewConfig_Validate_ValidClaude(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:          true,
        Provider:         ReviewProviderClaude,
        MaxFixIterations: 3,
    }
    if err := cfg.Validate(); err != nil {
        t.Errorf("Validate() error = %v, want nil", err)
    }
}

func TestCodeReviewConfig_Validate_DisabledInvalidProvider(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:  false,
        Provider: "invalid",
    }
    if err := cfg.Validate(); err != nil {
        t.Errorf("Validate() error = %v, want nil (invalid provider allowed when disabled)", err)
    }
}

func TestCodeReviewConfig_Validate_InvalidProvider(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:  true,
        Provider: "gpt4",
    }
    err := cfg.Validate()
    if err == nil {
        t.Error("Validate() error = nil, want error for invalid provider")
    }
    if !strings.Contains(err.Error(), "invalid review provider") {
        t.Errorf("error should mention 'invalid review provider', got: %v", err)
    }
}

func TestCodeReviewConfig_Validate_NegativeIterations(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:          true,
        Provider:         ReviewProviderCodex,
        MaxFixIterations: -1,
    }
    err := cfg.Validate()
    if err == nil {
        t.Error("Validate() error = nil, want error for negative iterations")
    }
    if !strings.Contains(err.Error(), "negative") {
        t.Errorf("error should mention 'negative', got: %v", err)
    }
}

func TestCodeReviewConfig_Validate_ZeroIterations(t *testing.T) {
    cfg := CodeReviewConfig{
        Enabled:          true,
        Provider:         ReviewProviderCodex,
        MaxFixIterations: 0,
    }
    if err := cfg.Validate(); err != nil {
        t.Errorf("Validate() error = %v, want nil (0 iterations is valid review-only mode)", err)
    }
}

func TestCodeReviewConfig_IsReviewOnlyMode(t *testing.T) {
    tests := []struct {
        iterations int
        want       bool
    }{
        {0, true},
        {1, false},
        {5, false},
    }
    for _, tt := range tests {
        t.Run(fmt.Sprintf("iterations=%d", tt.iterations), func(t *testing.T) {
            cfg := CodeReviewConfig{MaxFixIterations: tt.iterations}
            if got := cfg.IsReviewOnlyMode(); got != tt.want {
                t.Errorf("IsReviewOnlyMode() = %v, want %v", got, tt.want)
            }
        })
    }
}

func TestDefaultCodeReviewConfig(t *testing.T) {
    cfg := DefaultCodeReviewConfig()

    if !cfg.Enabled {
        t.Error("expected Enabled to be true by default")
    }
    if cfg.Provider != ReviewProviderCodex {
        t.Errorf("expected Provider to be %q, got %q", ReviewProviderCodex, cfg.Provider)
    }
    if cfg.MaxFixIterations != 1 {
        t.Errorf("expected MaxFixIterations to be 1, got %d", cfg.MaxFixIterations)
    }
    if !cfg.Verbose {
        t.Error("expected Verbose to be true by default (noisy)")
    }
    if cfg.Command != "" {
        t.Errorf("expected Command to be empty, got %q", cfg.Command)
    }
}
```

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Provider validation only runs when `Enabled=true` (invalid provider is fine when disabled)
- `MaxFixIterations=0` is valid (review-only mode) but negative is invalid
- The existing LoadConfig pattern pre-fills defaults, so YAML unmarshaling overlays explicit values
- Use existing `ValidationError` type for consistency with other validation errors
- Test both the `Validate()` method directly and integration through `validateConfig()`

## NOT In Scope

- Orchestrator's resolveReviewer() method (separate spec: REVIEW-PROVIDER)
- CLI command for review (review runs as part of `choo run` workflow)
- Actual review execution logic (separate specs: REVIEW-PROVIDER, REVIEW-LOOP)
