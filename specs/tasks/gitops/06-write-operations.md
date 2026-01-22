---
task: 6
status: pending
backpressure: "go test ./internal/git/... -run TestGitOps_Write -v"
depends_on: [3, 4]
---

# Write Operations

**Parent spec**: `/specs/GITOPS.md`
**Task**: #6 of 7 in implementation plan

## Objective

Implement non-destructive write operations: Add, AddAll, Reset, Commit, CheckoutBranch.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #3 must be complete (provides: getRepoLock)
- Task #4 must be complete (provides: gitOps struct, validateRuntime)

### Package Dependencies
- Standard library (`context`, `fmt`, `strings`, `time`)

## Deliverables

### Files to Modify

```
internal/git/
├── gitops.go      # MODIFY: Add write operation methods
└── gitops_test.go # MODIFY: Add write operation tests
```

### Functions to Implement

```go
// validateBranchGuard enforces branch/remote constraints before write operations.
func (g *gitOps) validateBranchGuard(ctx context.Context) error {
    if g.opts.BranchGuard == nil {
        return nil
    }

    branch, err := g.currentBranchInternal(ctx)
    if err != nil {
        return fmt.Errorf("branch guard: %w", err)
    }

    guard := g.opts.BranchGuard

    // Check exact match
    if guard.ExpectedBranch != "" && branch != guard.ExpectedBranch {
        return fmt.Errorf("%w: expected=%s, actual=%s", ErrUnexpectedBranch, guard.ExpectedBranch, branch)
    }

    // Check prefix match
    if len(guard.AllowedBranchPrefixes) > 0 {
        allowed := false
        for _, prefix := range guard.AllowedBranchPrefixes {
            if strings.HasPrefix(branch, prefix) {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("%w: branch=%s, allowed=%v", ErrUnexpectedBranch, branch, guard.AllowedBranchPrefixes)
        }
    }

    // Check protected branches
    protected := guard.ProtectedBranches
    if len(protected) == 0 {
        protected = []string{"main", "master"}
    }
    for _, p := range protected {
        if branch == p {
            return fmt.Errorf("%w: %s", ErrProtectedBranch, branch)
        }
    }

    return nil
}

// Add stages the specified files.
func (g *gitOps) Add(ctx context.Context, paths ...string) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    args := append([]string{"add", "--"}, paths...)
    _, err := g.exec(ctx, args...)
    return err
}

// AddAll stages all changes (git add -A).
func (g *gitOps) AddAll(ctx context.Context) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    _, err := g.exec(ctx, "add", "-A")
    return err
}

// Reset unstages the specified files (or all if none specified).
// Acquires per-repo lock.
func (g *gitOps) Reset(ctx context.Context, paths ...string) error {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Reset", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"reset", "HEAD"}
    if len(paths) > 0 {
        args = append(args, "--")
        args = append(args, paths...)
    }
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "Reset", RepoPath: g.path, Args: args,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// Commit creates a commit with the given message.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Commit", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Commit", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"commit", "-m", msg}
    if opts.NoVerify {
        args = append(args, "--no-verify")
    }
    if opts.Author != "" {
        args = append(args, "--author="+opts.Author)
    }
    if opts.AllowEmpty {
        args = append(args, "--allow-empty")
    }

    branch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "Commit", RepoPath: g.path, Branch: branch,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// CheckoutBranch switches to a branch, optionally creating it.
// Acquires per-repo lock to prevent concurrent branch operations.
func (g *gitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "CheckoutBranch", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"checkout"}
    if create {
        args = append(args, "-b")
    }
    args = append(args, branch)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "CheckoutBranch", RepoPath: g.path,
        Branch: branch, Args: args, SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestGitOps_Write -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGitOps_WriteAdd` | File appears in staged after Add |
| `TestGitOps_WriteAddAll` | All changes staged after AddAll |
| `TestGitOps_WriteReset` | Staged files unstaged after Reset |
| `TestGitOps_WriteCommit` | Commit created with correct message |
| `TestGitOps_WriteCommit_NoVerify` | --no-verify flag passed when set |
| `TestGitOps_WriteCommit_ProtectedBranch` | Returns ErrProtectedBranch on main |
| `TestGitOps_WriteCheckoutBranch_Create` | New branch created and checked out |
| `TestGitOps_WriteCheckoutBranch_Existing` | Existing branch checked out |

### Test Implementation

```go
func TestGitOps_WriteAdd(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    err := ops.Add(context.Background(), "file.txt")

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    status, _ := ops.Status(context.Background())
    if len(status.Staged) != 1 {
        t.Errorf("expected 1 staged file, got %d", len(status.Staged))
    }
}

func TestGitOps_WriteCommit_ProtectedBranch(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", "-b", "main", dir).Run()
    os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
    exec.Command("git", "-C", dir, "add", ".").Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot: true,
        BranchGuard:   &BranchGuard{}, // Uses default protected: main, master
    })

    err := ops.Commit(context.Background(), "test", CommitOpts{})

    if !errors.Is(err, ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch, got %v", err)
    }
}

func TestGitOps_WriteCheckoutBranch_Create(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", "-b", "main", dir).Run()
    exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    err := ops.CheckoutBranch(context.Background(), "feature/test", true)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    branch, _ := ops.CurrentBranch(context.Background())
    if branch != "feature/test" {
        t.Errorf("expected feature/test, got %s", branch)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Write operations acquire per-repo lock before execution
- Branch guard is checked before Commit (not Add/AddAll which are staging-only)
- Audit entries are created for operations that modify repo state
- Add/AddAll don't require lock (safe concurrent staging)

## NOT In Scope

- Destructive operations: CheckoutFiles, Clean, ResetHard (Task #7)
- Remote operations: Fetch, Push (Task #7)
- Merge operations: Merge, MergeAbort (Task #7)
