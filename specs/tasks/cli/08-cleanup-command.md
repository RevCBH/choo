---
task: 8
status: pending
backpressure: "go test ./internal/cli/... -run Cleanup"
depends_on: [1]
---

# Cleanup Command

**Parent spec**: `/specs/CLI.md`
**Task**: #8 of 9 in implementation plan

## Objective

Implement the cleanup command that removes worktrees and optionally resets orchestration state.

## Dependencies

### External Specs (must be implemented)
- GIT - provides `WorktreeManager` for worktree operations
- DISCOVERY - provides frontmatter update capability

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)

### Package Dependencies
- None beyond standard library and internal packages

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── cleanup.go    # CREATE: Cleanup command implementation
```

### Types to Implement

```go
// CleanupOptions holds flags for the cleanup command
type CleanupOptions struct {
    TasksDir   string // Path to specs/tasks/ directory
    ResetState bool   // Also reset frontmatter status
}
```

### Functions to Implement

```go
// NewCleanupCmd creates the cleanup command
func NewCleanupCmd(app *App) *cobra.Command {
    // Create command with Use: "cleanup [tasks-dir]"
    // Add --reset-state flag
    // Call Cleanup in RunE
}

// Cleanup removes worktrees and optionally resets state
func (a *App) Cleanup(opts CleanupOptions) error {
    // Find all orchestrator worktrees
    // Remove each worktree
    // If reset-state, update frontmatter to pending
    // Report what was cleaned
}

// resetFrontmatterState resets all task/unit status to pending
func resetFrontmatterState(tasksDir string) error {
    // Find all task spec files
    // Update status to pending in frontmatter
    // Clear orchestrator-managed fields
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Cleanup -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCleanupCmd_DefaultFlags` | ResetState defaults to false |
| `TestCleanupCmd_ResetStateFlag` | --reset-state flag is recognized |
| `TestCleanup_NoWorktrees` | Handles case with no worktrees gracefully |
| `TestCleanup_RemovesWorktrees` | Worktrees are removed |
| `TestCleanup_WithResetState` | Frontmatter reset to pending |
| `TestResetFrontmatterState` | All tasks set to pending status |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock worktree list | In-memory | Test worktree removal |
| Task specs with state | testdata/cleanup/ | Test frontmatter reset |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Worktree location: `.ralph/worktrees/` (or from config)
- Cleanup should be idempotent (safe to run multiple times)
- Print summary of what was cleaned:
  ```
  Removed 3 worktrees:
    - .ralph/worktrees/unit-a
    - .ralph/worktrees/unit-b
    - .ralph/worktrees/unit-c
  Reset state for 5 units (12 tasks)
  ```

- If `--reset-state` is used, reset these frontmatter fields:
  - `status: pending`
  - `orch_status: pending` (in IMPLEMENTATION_PLAN.md)
  - `orch_branch: null`
  - `orch_pr_number: null`
  - `orch_started_at: null`
  - `orch_completed_at: null`

- Error handling:
  - Continue cleaning even if one worktree fails to remove
  - Report all errors at the end
  - Return error if any cleanup failed

## NOT In Scope

- Selective cleanup (specific units only)
- Branch deletion (handled separately)
- PR closing (handled separately)
