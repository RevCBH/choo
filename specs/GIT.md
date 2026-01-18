# GIT - Git Operations for Ralph Orchestrator

## Overview

The GIT package provides git operations for choo, including worktree management, branch operations, commits, and merge serialization. It enables parallel unit execution through isolated worktrees and ensures safe merging of concurrent PRs through mutex-based serialization.

The package consists of four main components: WorktreeManager (creates/removes worktrees in `.ralph/worktrees/`), Branch (naming and creation via Claude CLI), Commit (staging and committing with `--no-verify`), and Merge (mutex-based FCFS serialization with conflict resolution).

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              GIT Package                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐      │
│   │  WorktreeManager │  │      Branch      │  │      Merge       │      │
│   │                  │  │                  │  │                  │      │
│   │ - Create worktree│  │ - Name generation│  │ - Mutex lock     │      │
│   │ - Remove worktree│  │ - Claude haiku   │  │ - Rebase         │      │
│   │ - Setup commands │  │ - Short suffix   │  │ - Conflict res   │      │
│   │ - Conditional run│  │                  │  │ - Force push     │      │
│   └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘      │
│            │                     │                     │                 │
│            └─────────────────────┼─────────────────────┘                 │
│                                  ▼                                       │
│                         ┌──────────────────┐                             │
│                         │    Git CLI       │                             │
│                         │   (subprocess)   │                             │
│                         └──────────────────┘                             │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Create isolated git worktrees in `.ralph/worktrees/<unit-id>/`
2. Generate branch names using Claude CLI with haiku model for creativity
3. Run conditional setup commands after worktree creation (e.g., `npm install` only if `package.json` exists)
4. Remove worktrees cleanly on completion or cleanup
5. Commit changes with `--no-verify` flag during task execution
6. Serialize merges using mutex-based FCFS (First-Come-First-Served)
7. Rebase onto target branch before merge to ensure fast-forward
8. Resolve merge conflicts using Claude CLI, with up to 3 attempts
9. Force-push with lease after conflict resolution
10. Delete merged branches only after full batch is merged

### Performance Requirements

| Metric | Target |
|--------|--------|
| Worktree creation | <5s (excluding setup commands) |
| Branch name generation | <2s |
| Merge lock acquisition | <100ms (excluding wait time) |
| Worktree removal | <2s |

### Constraints

- Depends on: `internal/claude` (for branch naming and conflict resolution)
- Requires git 2.15+ for worktree features
- Worktrees must be on same filesystem as main repo
- Branch names must be valid git refs (no spaces, special chars limited)

## Design

### Module Structure

```
internal/git/
├── worktree.go      # WorktreeManager implementation
├── branch.go        # Branch naming and creation
├── commit.go        # Staging and committing
├── merge.go         # Merge serialization and conflict resolution
└── exec.go          # Git command execution utilities
```

### Core Types

```go
// internal/git/worktree.go

// WorktreeManager handles creation and removal of git worktrees
type WorktreeManager struct {
    // RepoRoot is the absolute path to the main repository
    RepoRoot string

    // WorktreeBase is the base directory for worktrees (default: .ralph/worktrees/)
    WorktreeBase string

    // SetupCommands are conditional commands to run after worktree creation
    SetupCommands []ConditionalCommand

    // ClaudeClient for branch name generation
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
    // Relative to worktree root
    ConditionFile string

    // Command is the command to execute
    Command string

    // Args are the command arguments
    Args []string

    // Description is a human-readable description for logging
    Description string
}
```

```go
// internal/git/branch.go

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

```go
// internal/git/merge.go

// MergeManager handles serialized merging of branches
type MergeManager struct {
    // mutex ensures only one merge at a time
    mutex sync.Mutex

    // RepoRoot is the main repository path
    RepoRoot string

    // Claude client for conflict resolution
    Claude *claude.Client

    // MaxConflictAttempts is the max retries for conflict resolution
    MaxConflictAttempts int

    // PendingDeletes tracks branches to delete after batch completes
    PendingDeletes []string
}

// MergeResult contains the outcome of a merge operation
type MergeResult struct {
    // Success indicates if the merge completed
    Success bool

    // ConflictsResolved is the number of conflicts that were resolved
    ConflictsResolved int

    // Attempts is how many conflict resolution attempts were made
    Attempts int

    // Error is set if the merge failed
    Error error
}
```

```go
// internal/git/commit.go

// CommitOptions configures a commit operation
type CommitOptions struct {
    // Message is the commit message
    Message string

    // NoVerify skips pre-commit hooks (default: true during tasks)
    NoVerify bool

    // AllowEmpty permits commits with no changes
    AllowEmpty bool
}
```

### API Surface

```go
// internal/git/worktree.go

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(repoRoot string, claude *claude.Client) *WorktreeManager

// CreateWorktree creates a new worktree for a unit
// Returns the created worktree or error
func (m *WorktreeManager) CreateWorktree(ctx context.Context, unitID, targetBranch string) (*Worktree, error)

// RemoveWorktree removes a worktree and its directory
func (m *WorktreeManager) RemoveWorktree(ctx context.Context, wt *Worktree) error

// ListWorktrees returns all active worktrees managed by ralph
func (m *WorktreeManager) ListWorktrees(ctx context.Context) ([]*Worktree, error)

// CleanupOrphans removes worktrees that no longer have associated units
func (m *WorktreeManager) CleanupOrphans(ctx context.Context) error

// SetupCommands returns the default conditional setup commands
func DefaultSetupCommands() []ConditionalCommand
```

```go
// internal/git/branch.go

// NewBranchNamer creates a branch namer with the given Claude client
func NewBranchNamer(claude *claude.Client) *BranchNamer

// GenerateName creates a creative branch name for a unit
// Uses Claude CLI with haiku model for short, memorable suffixes
func (n *BranchNamer) GenerateName(ctx context.Context, unitID string) (string, error)

// ValidateBranchName checks if a branch name is valid for git
func ValidateBranchName(name string) error

// SanitizeBranchName converts a string to a valid branch name component
func SanitizeBranchName(s string) string
```

```go
// internal/git/commit.go

// Commit stages and commits changes in a worktree
func Commit(ctx context.Context, worktreePath string, opts CommitOptions) error

// StageAll stages all changes in a worktree
func StageAll(ctx context.Context, worktreePath string) error

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges(ctx context.Context, worktreePath string) (bool, error)
```

```go
// internal/git/merge.go

// NewMergeManager creates a new merge manager
func NewMergeManager(repoRoot string, claude *claude.Client) *MergeManager

// Merge acquires the merge lock, rebases, and merges a branch
// This is the primary entry point for merge operations
func (m *MergeManager) Merge(ctx context.Context, branch *Branch) (*MergeResult, error)

// ScheduleBranchDelete marks a branch for deletion after batch completes
func (m *MergeManager) ScheduleBranchDelete(branchName string)

// FlushDeletes deletes all scheduled branches
// Called after full batch is merged
func (m *MergeManager) FlushDeletes(ctx context.Context) error

// Rebase rebases the current branch onto the target
func Rebase(ctx context.Context, worktreePath, targetBranch string) (hasConflicts bool, err error)

// ResolveConflicts uses Claude to resolve merge conflicts
func (m *MergeManager) ResolveConflicts(ctx context.Context, worktreePath string) error

// ForcePushWithLease pushes with --force-with-lease for safety
func ForcePushWithLease(ctx context.Context, worktreePath string) error
```

### Worktree Creation Flow

```
CreateWorktree(unitID, targetBranch)
    │
    ▼
┌──────────────────────────────┐
│ 1. Generate branch name      │
│    via Claude CLI (haiku)    │
│    "ralph/<unit>-<suffix>"   │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 2. Create worktree directory │
│    .ralph/worktrees/<unit>/  │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 3. git worktree add          │
│    <path> -b <branch>        │
│    <target_branch>           │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 4. Run conditional setup     │
│    commands (see below)      │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 5. Return Worktree           │
└──────────────────────────────┘
```

### Conditional Setup Commands

Setup commands run after worktree creation, but only if their condition file exists:

```go
func DefaultSetupCommands() []ConditionalCommand {
    return []ConditionalCommand{
        {
            ConditionFile: "package.json",
            Command:       "npm",
            Args:          []string{"install"},
            Description:   "Installing npm dependencies",
        },
        {
            ConditionFile: "pnpm-lock.yaml",
            Command:       "pnpm",
            Args:          []string{"install"},
            Description:   "Installing pnpm dependencies",
        },
        {
            ConditionFile: "yarn.lock",
            Command:       "yarn",
            Args:          []string{"install"},
            Description:   "Installing yarn dependencies",
        },
        {
            ConditionFile: "Cargo.toml",
            Command:       "cargo",
            Args:          []string{"fetch"},
            Description:   "Fetching Cargo dependencies",
        },
        {
            ConditionFile: "go.mod",
            Command:       "go",
            Args:          []string{"mod", "download"},
            Description:   "Downloading Go modules",
        },
    }
}
```

The runner checks each condition file and runs the first matching command:

```go
func (m *WorktreeManager) runSetupCommands(ctx context.Context, worktreePath string) error {
    for _, cmd := range m.SetupCommands {
        condPath := filepath.Join(worktreePath, cmd.ConditionFile)
        if _, err := os.Stat(condPath); err == nil {
            // Condition file exists, run the command
            log.Printf("%s", cmd.Description)
            if err := exec.CommandContext(ctx, cmd.Command, cmd.Args...).Run(); err != nil {
                return fmt.Errorf("%s failed: %w", cmd.Description, err)
            }
            return nil // Only run the first matching command
        }
    }
    return nil
}
```

### Branch Naming via Claude

Branch names are generated using a lightweight Claude CLI call with the haiku model:

```go
func (n *BranchNamer) GenerateName(ctx context.Context, unitID string) (string, error) {
    prompt := fmt.Sprintf(`Generate a short, memorable 2-3 word suffix for a git branch.
The branch is for a unit called "%s".
Return ONLY the suffix, lowercase, words separated by hyphens.
Examples: sunset-harbor, quick-fox, blue-mountain
No explanation, just the suffix.`, unitID)

    // Use haiku model for fast, cheap generation
    result, err := n.Claude.Invoke(ctx, claude.InvokeOptions{
        Prompt:   prompt,
        Model:    "claude-3-haiku-20240307",
        MaxTurns: 1,
    })
    if err != nil {
        // Fallback to random suffix on error
        return fmt.Sprintf("%s%s-%s", n.Prefix, unitID, randomSuffix()), nil
    }

    suffix := strings.TrimSpace(result)
    suffix = SanitizeBranchName(suffix)

    return fmt.Sprintf("%s%s-%s", n.Prefix, unitID, suffix), nil
}

func randomSuffix() string {
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, 6)
    for i := range b {
        b[i] = chars[rand.Intn(len(chars))]
    }
    return string(b)
}
```

### Merge Serialization Flow

Merges are serialized using a mutex to prevent concurrent rebases from causing conflicts:

```
Merge(branch)
    │
    ▼
┌──────────────────────────────┐
│ 1. Acquire merge mutex       │
│    (blocks until available)  │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 2. git fetch origin          │
│    <target_branch>           │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 3. git rebase                │
│    origin/<target>           │
└──────────────┬───────────────┘
               │
      ┌────────┴────────┐
      │                 │
      ▼                 ▼
 No Conflicts      Has Conflicts
      │                 │
      │                 ▼
      │      ┌──────────────────────────────┐
      │      │ 4a. Claude conflict resolve  │
      │      │     (up to 3 attempts)       │
      │      └──────────────┬───────────────┘
      │                     │
      │                     ▼
      │      ┌──────────────────────────────┐
      │      │ 4b. git push --force-with-   │
      │      │     lease                    │
      │      └──────────────┬───────────────┘
      │                     │
      └─────────┬───────────┘
                │
                ▼
┌──────────────────────────────┐
│ 5. GitHub API: Merge PR      │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 6. Schedule branch delete    │
│    (defer until batch done)  │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ 7. Release merge mutex       │
└──────────────────────────────┘
```

### Conflict Resolution with Claude

```go
func (m *MergeManager) resolveConflictsWithClaude(ctx context.Context, worktreePath string) error {
    for attempt := 1; attempt <= m.MaxConflictAttempts; attempt++ {
        // Get list of conflicted files
        conflicts, err := getConflictedFiles(ctx, worktreePath)
        if err != nil {
            return err
        }
        if len(conflicts) == 0 {
            return nil // All resolved
        }

        // Build conflict resolution prompt
        prompt := buildConflictPrompt(conflicts, worktreePath)

        // Invoke Claude to resolve
        err = m.Claude.Invoke(ctx, claude.InvokeOptions{
            WorkingDir: worktreePath,
            Prompt:     prompt,
        })
        if err != nil {
            if attempt == m.MaxConflictAttempts {
                return fmt.Errorf("conflict resolution failed after %d attempts: %w",
                    attempt, err)
            }
            continue
        }

        // Check if conflicts remain
        remaining, _ := getConflictedFiles(ctx, worktreePath)
        if len(remaining) == 0 {
            // Continue rebase
            if err := continueRebase(ctx, worktreePath); err != nil {
                return err
            }
            return nil
        }
    }

    return fmt.Errorf("failed to resolve conflicts after %d attempts", m.MaxConflictAttempts)
}

func getConflictedFiles(ctx context.Context, path string) ([]string, error) {
    cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
    cmd.Dir = path
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}
```

### Branch Cleanup Strategy

Branches are NOT deleted immediately after merge. They are scheduled for deletion and only cleaned up after the entire batch of PRs has merged:

```go
// During merge
func (m *MergeManager) Merge(ctx context.Context, branch *Branch) (*MergeResult, error) {
    // ... merge logic ...

    // Schedule for deletion, don't delete yet
    m.ScheduleBranchDelete(branch.Name)

    return &MergeResult{Success: true}, nil
}

// After batch completes (called by orchestrator)
func (m *MergeManager) FlushDeletes(ctx context.Context) error {
    for _, branchName := range m.PendingDeletes {
        // Delete remote branch
        if err := deleteBranch(ctx, m.RepoRoot, branchName, true); err != nil {
            log.Printf("WARN: failed to delete remote branch %s: %v", branchName, err)
        }

        // Delete local branch
        if err := deleteBranch(ctx, m.RepoRoot, branchName, false); err != nil {
            log.Printf("WARN: failed to delete local branch %s: %v", branchName, err)
        }
    }

    m.PendingDeletes = nil
    return nil
}
```

This ensures that if a later merge fails, earlier branches can still be referenced for debugging.

## Implementation Notes

### Git Command Execution

All git commands are executed via subprocess with proper error handling:

```go
// internal/git/exec.go

func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = dir

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git %s failed: %w\nstderr: %s",
            strings.Join(args, " "), err, stderr.String())
    }

    return stdout.String(), nil
}
```

### Force Push Safety

Force pushes always use `--force-with-lease` to prevent overwriting others' changes:

```go
func ForcePushWithLease(ctx context.Context, worktreePath string) error {
    _, err := gitExec(ctx, worktreePath, "push", "--force-with-lease")
    return err
}
```

### Commit with --no-verify

During task execution, commits skip pre-commit hooks to avoid interrupting the automation:

```go
func Commit(ctx context.Context, worktreePath string, opts CommitOptions) error {
    args := []string{"commit", "-m", opts.Message}

    if opts.NoVerify {
        args = append(args, "--no-verify")
    }
    if opts.AllowEmpty {
        args = append(args, "--allow-empty")
    }

    _, err := gitExec(ctx, worktreePath, args...)
    return err
}
```

### Worktree Directory Structure

```
.ralph/
└── worktrees/
    ├── app-shell/           # Worktree for app-shell unit
    │   ├── .git             # Worktree git link
    │   ├── package.json
    │   ├── src/
    │   └── ...
    ├── deck-list/           # Worktree for deck-list unit
    │   └── ...
    └── config/              # Worktree for config unit
        └── ...
```

## Testing Strategy

### Unit Tests

```go
// internal/git/branch_test.go

func TestSanitizeBranchName(t *testing.T) {
    tests := []struct {
        input string
        want  string
    }{
        {"hello world", "hello-world"},
        {"Hello World", "hello-world"},
        {"foo/bar", "foo-bar"},
        {"foo..bar", "foo-bar"},
        {"  spaces  ", "spaces"},
        {"special@#chars!", "special-chars"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := SanitizeBranchName(tt.input)
            if got != tt.want {
                t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}

func TestValidateBranchName(t *testing.T) {
    tests := []struct {
        name    string
        wantErr bool
    }{
        {"ralph/app-shell-sunset", false},
        {"feature/add-login", false},
        {"main", false},
        {"", true},
        {"refs/heads/main", true},  // Cannot start with refs/
        {"branch..name", true},      // Cannot contain ..
        {"branch name", true},       // Cannot contain spaces
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateBranchName(tt.name)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateBranchName(%q) error = %v, wantErr %v",
                    tt.name, err, tt.wantErr)
            }
        })
    }
}
```

```go
// internal/git/worktree_test.go

func TestConditionalCommand_Matching(t *testing.T) {
    tmpDir := t.TempDir()

    // Create package.json
    os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644)

    commands := DefaultSetupCommands()

    // Find which command would run
    var matched *ConditionalCommand
    for i := range commands {
        condPath := filepath.Join(tmpDir, commands[i].ConditionFile)
        if _, err := os.Stat(condPath); err == nil {
            matched = &commands[i]
            break
        }
    }

    if matched == nil {
        t.Fatal("expected to match npm install command")
    }
    if matched.Command != "npm" {
        t.Errorf("expected npm command, got %s", matched.Command)
    }
}
```

```go
// internal/git/merge_test.go

func TestMergeManager_Serialization(t *testing.T) {
    // Test that merges are serialized
    manager := &MergeManager{}

    var wg sync.WaitGroup
    var order []int
    var mu sync.Mutex

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            manager.mutex.Lock()
            defer manager.mutex.Unlock()

            mu.Lock()
            order = append(order, id)
            mu.Unlock()

            time.Sleep(10 * time.Millisecond) // Simulate work
        }(i)
    }

    wg.Wait()

    // All goroutines completed
    if len(order) != 5 {
        t.Errorf("expected 5 completed, got %d", len(order))
    }
}

func TestMergeManager_PendingDeletes(t *testing.T) {
    manager := &MergeManager{}

    manager.ScheduleBranchDelete("ralph/app-shell-sunset")
    manager.ScheduleBranchDelete("ralph/deck-list-harbor")

    if len(manager.PendingDeletes) != 2 {
        t.Errorf("expected 2 pending deletes, got %d", len(manager.PendingDeletes))
    }

    // Flush would normally delete, but we just clear for unit test
    manager.PendingDeletes = nil

    if len(manager.PendingDeletes) != 0 {
        t.Error("expected pending deletes to be cleared")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Worktree creation | Create temp git repo, create worktree, verify branch and files |
| Worktree removal | Create worktree, remove it, verify directory and branch gone |
| Setup command execution | Create worktree with package.json, verify npm install was attempted |
| Rebase without conflicts | Create branch, make commits, rebase onto updated target |
| Rebase with conflicts | Create conflicting changes, verify conflict detection |
| Merge serialization | Attempt concurrent merges, verify they execute serially |

### Manual Testing

- [ ] `CreateWorktree` creates worktree in correct location
- [ ] Branch name generation produces valid, memorable names
- [ ] Conditional setup runs npm install when package.json exists
- [ ] Conditional setup runs pnpm install when pnpm-lock.yaml exists
- [ ] Worktree removal cleans up directory and branch
- [ ] Commit with --no-verify skips pre-commit hooks
- [ ] Concurrent merges block and execute serially
- [ ] Conflict resolution invokes Claude and retries up to 3 times
- [ ] Force push uses --force-with-lease
- [ ] Branch cleanup waits until batch completion

## Design Decisions

### Why .ralph/worktrees/ instead of /tmp?

The PRD (Open Question #5) asks about worktree location. Using `.ralph/worktrees/` in the repo:
- Survives system reboots (unlike `/tmp`)
- Keeps worktrees close to main repo for easy inspection
- Avoids permission issues across filesystem boundaries
- Naturally cleaned up with `choo cleanup`

### Why Claude for Branch Names?

Memorable branch names like `ralph/app-shell-sunset-harbor` are easier to work with than random hashes like `ralph/app-shell-a1b2c3`. Using Claude with haiku model is fast (~1s) and cheap, and produces creative, memorable names. A random fallback ensures robustness if Claude is unavailable.

### Why Mutex-Based FCFS for Merges?

The PRD specifies "Simple mutex-based FCFS" in section 4.5. This approach:
- Is simple to implement and reason about
- Guarantees no concurrent rebase conflicts
- Provides fairness (first-come-first-served)
- Has minimal overhead (single mutex)

Alternatives like optimistic locking or queuing systems add complexity without significant benefit for the expected concurrency levels (4 parallel units).

### Why --force-with-lease?

After conflict resolution, we need to force push. Using `--force-with-lease` instead of `--force` ensures we don't accidentally overwrite changes pushed by someone else. It's a safety net against race conditions.

### Why Delete Branches After Batch?

If we delete branches immediately after merge, and a subsequent merge fails, we lose the ability to inspect the merged branches for debugging. By deferring deletion until the entire batch completes, all branches remain available for troubleshooting if needed.

### Why --no-verify for Commits?

During automated task execution, pre-commit hooks (linting, tests, etc.) should not block progress. Claude's backpressure mechanism already validates the code. The --no-verify flag prevents hooks from interrupting the automation flow.

## Future Enhancements

1. Worktree pooling - Reuse worktrees across units to save setup time
2. Parallel conflict resolution - Allow multiple Claude instances for different conflict types
3. Stacked PRs - Support for dependent PRs that merge in sequence
4. Branch protection bypass - Support for admin merge when branch protection is enabled
5. Merge queue integration - Integrate with GitHub's native merge queue feature

## References

- [PRD Section 4.3: Worker Flow Phase 1](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 4.5: Merge Serialization](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [Git Worktrees Documentation](https://git-scm.com/docs/git-worktree)
- [Git Force Push with Lease](https://git-scm.com/docs/git-push#Documentation/git-push.txt---force-with-leaseltrefnamegt)
