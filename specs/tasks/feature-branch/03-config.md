---
task: 3
status: pending
backpressure: "go build ./internal/config/..."
depends_on: [1]
---

# Feature Config

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/oslo/specs/FEATURE-BRANCH.md`
**Task**: #3 of 4 in implementation plan

## Objective

Add FeatureConfig struct to the config package with PRD directory, specs directory, and branch prefix settings.

## Dependencies

### External Specs (must be implemented)
- CONFIG - provides config loading infrastructure from `internal/config/`

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: Feature types for context)

### Package Dependencies
- `gopkg.in/yaml.v3` - YAML parsing (already in use)

## Deliverables

### Files to Create/Modify

```
internal/
└── config/
    └── config.go    # MODIFY: Add FeatureConfig struct and defaults
```

### Types to Implement

```go
// FeatureConfig holds configuration for PRD-driven feature workflow
type FeatureConfig struct {
    PRDDir       string `yaml:"prd_dir"`       // Directory containing PRD files
    SpecsDir     string `yaml:"specs_dir"`     // Directory for generated specs
    BranchPrefix string `yaml:"branch_prefix"` // Prefix for feature branches
}
```

### Functions to Implement

```go
// DefaultFeatureConfig returns sensible defaults for feature configuration
func DefaultFeatureConfig() FeatureConfig
```

### Default Values

| Field | Default Value |
|-------|---------------|
| PRDDir | `"docs/prds"` |
| SpecsDir | `"specs"` |
| BranchPrefix | `"feature/"` |

### Config File Integration

Add `Feature FeatureConfig` field to the main Config struct:

```go
type Config struct {
    // ... existing fields ...
    Feature FeatureConfig `yaml:"feature"`
}
```

In the config loading logic, merge defaults:

```go
func loadConfig() *Config {
    cfg := &Config{
        // ... existing defaults ...
        Feature: DefaultFeatureConfig(),
    }
    // ... load from file and override ...
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
| Build succeeds | No compilation errors |
| `DefaultFeatureConfig().PRDDir` | Returns `"docs/prds"` |
| `DefaultFeatureConfig().SpecsDir` | Returns `"specs"` |
| `DefaultFeatureConfig().BranchPrefix` | Returns `"feature/"` |

### Test Fixtures

None required - pure type definitions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- FeatureConfig is optional in oslo.yaml - defaults are used if not specified
- BranchPrefix should always end with "/" for consistent branch naming
- PRDDir and SpecsDir are relative to repository root
- These values are read-only after config load (no runtime modification)

### Example oslo.yaml

```yaml
feature:
  prd_dir: "docs/prds"
  specs_dir: "specs"
  branch_prefix: "feature/"
```

## NOT In Scope

- PRD parsing logic (handled by feature-discovery)
- Validation of directory existence
- CLI flag parsing (Task #4)
- Runtime configuration updates
