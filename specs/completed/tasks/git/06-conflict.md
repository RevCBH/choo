---
task: 6
status: complete
backpressure: "go test ./internal/git/..."
depends_on: [5]
---

# Conflict Resolution with Claude

**Parent spec**: `/specs/GIT.md` **Task**: #6 of 6 in implementation plan

## Objective

Implement conflict resolution using Claude CLI with up to 3 retry attempts.

## Dependencies

### External Specs (must be implemented)

- CLAUDE - provides `*claude.Client` for conflict resolution

### Task Dependencies (within this unit)

- Task #5 must be complete (provides: `MergeManager`, `getConflictedFiles`,
  `continueRebase`)

### Package Dependencies

- `internal/claude`
- Standard library

## Deliverables

### Files to Create/Modify

```
internal/git/
└── merge.go    # MODIFY: Add conflict resolution methods
```

### Functions to Implement

```go
// ResolveConflicts uses Claude to resolve merge conflicts
// Called by Merge when rebase detects conflicts
func (m *MergeManager) ResolveConflicts(ctx context.Context, worktreePath string) error

// resolveConflictsWithClaude is the internal implementation with retry logic
func (m *MergeManager) resolveConflictsWithClaude(ctx context.Context, worktreePath string) error

// buildConflictPrompt creates the prompt for Claude to resolve conflicts
func buildConflictPrompt(conflicts []string, worktreePath string) string

// readConflictFile reads a file with conflict markers
func readConflictFile(path string) (string, error)
```

### Conflict Resolution Flow

```
For attempt 1 to MaxConflictAttempts (3):
    1. Get list of conflicted files
    2. If no conflicts, return success
    3. Build prompt with conflict context
    4. Invoke Claude to resolve
    5. Check if conflicts remain
    6. If resolved, continue rebase
    7. If still conflicts, retry

If all attempts fail, return error
```

### Conflict Prompt Template

```go
func buildConflictPrompt(conflicts []string, worktreePath string) string {
    // Include:
    // - List of conflicted files
    // - File contents with conflict markers
    // - Instructions to resolve and stage
    return fmt.Sprintf(`You are resolving git merge conflicts in %s.

The following files have conflicts:
%s

Please resolve each conflict by:
1. Reading the file with conflict markers
2. Choosing the correct resolution
3. Removing all conflict markers (<<<<<<<, =======, >>>>>>>)
4. Saving the resolved file
5. Staging the file with git add

Resolve all conflicts now.`, worktreePath, strings.Join(conflicts, "\n"))
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestConflict
```

### Must Pass

| Test                               | Assertion                               |
| ---------------------------------- | --------------------------------------- |
| `TestResolveConflicts_Success`     | Resolves conflicts and continues rebase |
| `TestResolveConflicts_Retry`       | Retries up to MaxConflictAttempts times |
| `TestResolveConflicts_MaxAttempts` | Fails after MaxConflictAttempts         |
| `TestResolveConflicts_NoConflicts` | Returns immediately if no conflicts     |
| `TestBuildConflictPrompt`          | Includes all conflicted files in prompt |
| `TestBuildConflictPrompt_Content`  | Includes file contents with markers     |

### Test Fixtures

| Fixture            | Location        | Purpose                     |
| ------------------ | --------------- | --------------------------- |
| Mock Claude client | In test         | Test resolution without API |
| Conflicted files   | Created in test | Files with conflict markers |

### CI Compatibility

- [x] No external API keys required (uses mock)
- [x] No network access required
- [x] Runs in <60 seconds

### Mock Strategy

For CI without Claude API:

```go
type mockClaudeClient struct {
    resolveFunc func(ctx context.Context, worktreePath string) error
}

// In test setup, mock resolves conflicts by removing markers
func mockResolve(ctx context.Context, worktreePath string) error {
    conflicts, _ := getConflictedFiles(ctx, worktreePath)
    for _, f := range conflicts {
        // Read file, keep "ours" side, remove markers
        // Stage the resolved file
    }
    return nil
}
```

## Implementation Notes

- MaxConflictAttempts defaults to 3
- Each attempt is a full Claude invocation
- Claude works in the worktree directory with file access
- After resolution, files must be staged before continuing rebase
- Force push with lease after successful resolution

## NOT In Scope

- Worktree management (Task #2)
- Branch naming (Task #3)
- Commit operations (Task #4)
- GitHub PR API integration (separate unit)
