---
task: 3
status: pending
backpressure: "go test ./internal/git/..."
depends_on: [1]
---

# Branch Naming via Claude

**Parent spec**: `/specs/GIT.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement BranchNamer that generates creative, memorable branch names using Claude haiku model.

## Dependencies

### External Specs (must be implemented)
- CLAUDE - provides `*claude.Client` for LLM invocation

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `gitExec` for validation)

### Package Dependencies
- `internal/claude` (Claude client)
- Standard library (`strings`, `regexp`, `math/rand`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── branch.go    # CREATE: Branch naming and validation
```

### Types to Implement

```go
// Branch represents a git branch with its metadata
type Branch struct {
    // Name is the full branch name (e.g., "ralph/deck-list-sunset-harbor")
    Name string

    // UnitID is the unit this branch is for
    UnitID string

    // TargetBranch is the branch this will merge into
    TargetBranch string
}

// BranchNamer generates creative branch names using Claude
type BranchNamer struct {
    // Claude client for name generation
    Claude *claude.Client

    // Prefix for all branch names (default: "ralph/")
    Prefix string
}
```

### Functions to Implement

```go
// NewBranchNamer creates a branch namer with the given Claude client
func NewBranchNamer(claude *claude.Client) *BranchNamer

// GenerateName creates a creative branch name for a unit
// Uses Claude CLI with haiku model for short, memorable suffixes
// Falls back to random suffix if Claude fails
func (n *BranchNamer) GenerateName(ctx context.Context, unitID string) (string, error)

// ValidateBranchName checks if a branch name is valid for git
func ValidateBranchName(name string) error

// SanitizeBranchName converts a string to a valid branch name component
func SanitizeBranchName(s string) string

// randomSuffix generates a random 6-character alphanumeric suffix
func randomSuffix() string
```

### Claude Prompt

```go
prompt := fmt.Sprintf(`Generate a short, memorable 2-3 word suffix for a git branch.
The branch is for a unit called "%s".
Return ONLY the suffix, lowercase, words separated by hyphens.
Examples: sunset-harbor, quick-fox, blue-mountain
No explanation, just the suffix.`, unitID)
```

### Branch Name Format

- Prefix: `ralph/`
- UnitID: sanitized unit identifier
- Suffix: Claude-generated or random fallback
- Example: `ralph/app-shell-sunset-harbor`

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestBranch
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestSanitizeBranchName_Spaces` | `SanitizeBranchName("hello world") == "hello-world"` |
| `TestSanitizeBranchName_Case` | `SanitizeBranchName("Hello World") == "hello-world"` |
| `TestSanitizeBranchName_Slashes` | `SanitizeBranchName("foo/bar") == "foo-bar"` |
| `TestSanitizeBranchName_Dots` | `SanitizeBranchName("foo..bar") == "foo-bar"` |
| `TestSanitizeBranchName_Special` | `SanitizeBranchName("special@#chars!") == "special-chars"` |
| `TestValidateBranchName_Valid` | Valid names pass validation |
| `TestValidateBranchName_Empty` | Empty name returns error |
| `TestValidateBranchName_Refs` | Names starting with `refs/` return error |
| `TestValidateBranchName_DoubleDot` | Names with `..` return error |
| `TestValidateBranchName_Spaces` | Names with spaces return error |
| `TestBranchNamer_GenerateName` | Returns valid branch name with prefix |
| `TestBranchNamer_Fallback` | Falls back to random suffix on Claude error |
| `TestRandomSuffix` | Returns 6-character alphanumeric string |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock Claude client | In test | Test generation without API |

### CI Compatibility

- [x] No external API keys required (uses mock)
- [x] No network access required
- [x] Runs in <60 seconds

### Mock Strategy

For CI without Claude API:

```go
type mockClaudeClient struct {
    response string
    err      error
}

func (m *mockClaudeClient) Invoke(ctx context.Context, opts claude.InvokeOptions) (string, error) {
    return m.response, m.err
}
```

## Implementation Notes

- Use haiku model (`claude-3-haiku-20240307`) for fast, cheap generation
- Branch name generation should complete in <2s
- Always sanitize Claude's output before using
- Random fallback ensures robustness

## NOT In Scope

- Worktree creation (Task #2)
- Commit operations (Task #4)
- Merge operations (Task #5, #6)
