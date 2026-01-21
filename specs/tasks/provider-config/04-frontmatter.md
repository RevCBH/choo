---
task: 4
status: complete
backpressure: "go test ./internal/discovery/... -run Frontmatter"
depends_on: []
---

# Unit Frontmatter Provider Field

**Parent spec**: `/specs/PROVIDER-CONFIG.md`
**Task**: #4 of 4 in implementation plan

## Objective

Add the `Provider` field to UnitFrontmatter to allow per-unit provider specification in IMPLEMENTATION_PLAN.md files.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (can be implemented in parallel with other tasks)

### Package Dependencies
- `gopkg.in/yaml.v3` - YAML parsing (already imported)

## Deliverables

### Files to Modify

```
internal/
└── discovery/
    ├── frontmatter.go       # MODIFY: Add Provider field to UnitFrontmatter
    └── frontmatter_test.go  # MODIFY: Add tests for Provider field parsing
```

### Types to Modify

Update `UnitFrontmatter` in `internal/discovery/frontmatter.go`:

```go
// UnitFrontmatter represents the YAML frontmatter in IMPLEMENTATION_PLAN.md
type UnitFrontmatter struct {
    // Required fields
    Unit string `yaml:"unit"`

    // Optional dependency field
    DependsOn []string `yaml:"depends_on"`

    // Provider overrides the default provider for this unit's task execution
    // Valid values: "claude", "codex"
    // Empty means use the resolved default from CLI/env/config
    Provider string `yaml:"provider,omitempty"`

    // Orchestrator-managed fields (may not be present initially)
    OrchStatus      string `yaml:"orch_status"`
    OrchBranch      string `yaml:"orch_branch"`
    OrchWorktree    string `yaml:"orch_worktree"`
    OrchPRNumber    int    `yaml:"orch_pr_number"`
    OrchStartedAt   string `yaml:"orch_started_at"`
    OrchCompletedAt string `yaml:"orch_completed_at"`
}
```

### Tests to Add

Add to `internal/discovery/frontmatter_test.go`:

```go
func TestParseUnitFrontmatter_WithProvider(t *testing.T) {
    input := []byte(`unit: my-feature
provider: codex
depends_on:
  - base-types`)

    uf, err := ParseUnitFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "my-feature", uf.Unit)
    assert.Equal(t, "codex", uf.Provider)
    assert.Equal(t, []string{"base-types"}, uf.DependsOn)
}

func TestParseUnitFrontmatter_WithoutProvider(t *testing.T) {
    input := []byte(`unit: my-feature
depends_on:
  - base-types`)

    uf, err := ParseUnitFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "my-feature", uf.Unit)
    assert.Equal(t, "", uf.Provider) // Empty means use default
    assert.Equal(t, []string{"base-types"}, uf.DependsOn)
}

func TestParseUnitFrontmatter_ProviderClaude(t *testing.T) {
    input := []byte(`unit: claude-optimized
provider: claude
depends_on: []`)

    uf, err := ParseUnitFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "claude-optimized", uf.Unit)
    assert.Equal(t, "claude", uf.Provider)
}

func TestParseUnitFrontmatter_ProviderWithOrchFields(t *testing.T) {
    input := []byte(`unit: my-feature
provider: codex
depends_on: []
orch_status: in_progress
orch_branch: feature/my-feature
orch_pr_number: 42`)

    uf, err := ParseUnitFrontmatter(input)
    require.NoError(t, err)

    assert.Equal(t, "my-feature", uf.Unit)
    assert.Equal(t, "codex", uf.Provider)
    assert.Equal(t, "in_progress", uf.OrchStatus)
    assert.Equal(t, "feature/my-feature", uf.OrchBranch)
    assert.Equal(t, 42, uf.OrchPRNumber)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/discovery/... -run Frontmatter -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestParseUnitFrontmatter_WithProvider` | Provider field correctly parsed as "codex" |
| `TestParseUnitFrontmatter_WithoutProvider` | Missing provider field results in empty string |
| `TestParseUnitFrontmatter_ProviderClaude` | Provider field correctly parsed as "claude" |
| `TestParseUnitFrontmatter_ProviderWithOrchFields` | Provider field works alongside orchestrator fields |

### Test Fixtures

Test cases use inline YAML strings representing unit frontmatter:

```yaml
# With provider specified
unit: my-feature
provider: codex
depends_on:
  - base-types

# Without provider (uses default)
unit: my-feature
depends_on:
  - base-types
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- The Provider field uses `omitempty` so it's not written when empty
- Empty Provider value means "use resolved default from CLI/env/config"
- Provider validation is NOT done during frontmatter parsing (non-fatal)
- Invalid provider values will be caught during provider resolution
- This is a non-breaking change - existing frontmatter without provider field continues to work

## Example Frontmatter

```yaml
---
unit: codex-optimized-feature
provider: codex
depends_on:
  - base-types
---
# This unit always uses codex (unless --force-task-provider overrides)
```

```yaml
---
unit: my-feature
depends_on:
  - base-types
---
# Uses resolved default provider (from CLI/env/config)
```

## NOT In Scope

- Provider validation during parsing (handled by config.ValidateProviderType)
- Provider resolution logic (Task #2)
- CLI flag handling (Task #3)
- Type definitions (Task #1)
