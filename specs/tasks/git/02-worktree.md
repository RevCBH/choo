---
task: 2
status: complete
backpressure: "go test ./internal/git/..."
depends_on: [1]
---

# Worktree Management

**Parent spec**: `/specs/GIT.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement WorktreeManager for creating, removing, and managing git worktrees in `.ralph/worktrees/`.

## Dependencies

### External Specs (must be implemented)
- CLAUDE - provides `*claude.Client` for branch naming

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `gitExec`)

### Package Dependencies
- Standard library (`os`, `path/filepath`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── worktree.go    # CREATE: WorktreeManager implementation
```

### Types to Implement

```go
// WorktreeManager handles creation and removal of git worktrees
type WorktreeManager struct {
    // RepoRoot is the absolute path to the main repository
    RepoRoot string

    // WorktreeBase is the base directory for worktrees (default: .ralph/worktrees/)
    WorktreeBase string

    // SetupCommands are conditional commands to run after worktree creation
    SetupCommands []ConditionalCommand

    // Claude client for branch name generation (may be nil for testing)
    Claude *claude.Client
}

// Worktree represents an active git worktree
type Worktree struct {
    // Path is the absolute path to the worktree directory
    Path string

    // Branch is the branch name checked out in this worktree
    Branch string

    // UnitID is the unit this worktree is associated with
    UnitID string

    // CreatedAt is when this worktree was created
    CreatedAt time.Time
}

// ConditionalCommand runs a command only if a condition file exists
type ConditionalCommand struct {
    // ConditionFile is the file that must exist for the command to run
    ConditionFile string

    // Command is the command to execute
    Command string

    // Args are the command arguments
    Args []string

    // Description is a human-readable description for logging
    Description string
}
```

### Functions to Implement

```go
// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(repoRoot string, claude *claude.Client) *WorktreeManager

// CreateWorktree creates a new worktree for a unit
func (m *WorktreeManager) CreateWorktree(ctx context.Context, unitID, targetBranch string) (*Worktree, error)

// RemoveWorktree removes a worktree and its directory
func (m *WorktreeManager) RemoveWorktree(ctx context.Context, wt *Worktree) error

// ListWorktrees returns all active worktrees managed by ralph
func (m *WorktreeManager) ListWorktrees(ctx context.Context) ([]*Worktree, error)

// CleanupOrphans removes worktrees that no longer have associated units
func (m *WorktreeManager) CleanupOrphans(ctx context.Context) error

// DefaultSetupCommands returns the default conditional setup commands
func DefaultSetupCommands() []ConditionalCommand

// runSetupCommands runs conditional setup commands in the worktree
func (m *WorktreeManager) runSetupCommands(ctx context.Context, worktreePath string) error
```

### Default Setup Commands

```go
func DefaultSetupCommands() []ConditionalCommand {
    return []ConditionalCommand{
        {ConditionFile: "package.json", Command: "npm", Args: []string{"install"}, Description: "Installing npm dependencies"},
        {ConditionFile: "pnpm-lock.yaml", Command: "pnpm", Args: []string{"install"}, Description: "Installing pnpm dependencies"},
        {ConditionFile: "yarn.lock", Command: "yarn", Args: []string{"install"}, Description: "Installing yarn dependencies"},
        {ConditionFile: "Cargo.toml", Command: "cargo", Args: []string{"fetch"}, Description: "Fetching Cargo dependencies"},
        {ConditionFile: "go.mod", Command: "go", Args: []string{"mod", "download"}, Description: "Downloading Go modules"},
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestWorktree
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWorktreeManager_Create` | Creates worktree in `.ralph/worktrees/<unitID>/` |
| `TestWorktreeManager_CreateBranch` | Creates branch based on target |
| `TestWorktreeManager_Remove` | Removes worktree directory and git reference |
| `TestWorktreeManager_List` | Lists all ralph worktrees |
| `TestConditionalCommand_Matching` | Runs first matching setup command |
| `TestConditionalCommand_NoMatch` | Skips when no condition file exists |
| `TestDefaultSetupCommands` | Returns expected default commands |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Temp git repo | Created in test | Test worktree operations |
| package.json | Created in test | Test conditional command |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Worktrees are created at `.ralph/worktrees/<unitID>/`
- Branch naming is delegated to Task #3 (BranchNamer)
- Only the first matching setup command runs
- Setup command failures should fail worktree creation

## NOT In Scope

- Branch name generation logic (Task #3)
- Commit operations (Task #4)
- Merge operations (Task #5, #6)
