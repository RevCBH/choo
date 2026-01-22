---
task: 7
status: pending
backpressure: "go test ./internal/git/... -run 'TestGitOps_Destructive|TestGitOps_Remote|TestGitOps_Merge' -v"
depends_on: [3, 6]
---

# Destructive, Remote, and Merge Operations

**Parent spec**: `/specs/GITOPS.md`
**Task**: #7 of 7 in implementation plan

## Objective

Implement destructive operations (CheckoutFiles, Clean, ResetHard), remote operations (Fetch, Push), and merge operations (Merge, MergeAbort).

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #3 must be complete (provides: getRepoLock)
- Task #6 must be complete (provides: validateBranchGuard)

### Package Dependencies
- Standard library (`context`, `fmt`, `time`)

## Deliverables

### Files to Modify

```
internal/git/
├── gitops.go      # MODIFY: Add destructive, remote, merge operations
└── gitops_test.go # MODIFY: Add tests for these operations
```

### Functions to Implement

```go
// CheckoutFiles discards changes to the specified files.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Acquires per-repo lock.
func (g *gitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
    start := time.Now()
    checks := []string{"path_valid", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "CheckoutFiles", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration: time.Since(start),
        })
        return fmt.Errorf("%w: CheckoutFiles", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "CheckoutFiles", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := append([]string{"checkout", "--"}, paths...)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "CheckoutFiles", RepoPath: g.path, Args: args,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// Clean removes untracked files.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Acquires per-repo lock.
func (g *gitOps) Clean(ctx context.Context, opts CleanOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Clean", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration: time.Since(start),
        })
        return fmt.Errorf("%w: Clean", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Clean", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"clean"}
    if opts.Force {
        args = append(args, "-f")
    }
    if opts.Directories {
        args = append(args, "-d")
    }
    if opts.IgnoredOnly {
        args = append(args, "-X")
    } else if opts.IgnoredToo {
        args = append(args, "-x")
    }

    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "Clean", RepoPath: g.path, Args: args,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// ResetHard performs a hard reset to the specified ref.
// DESTRUCTIVE: Requires AllowDestructive=true.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) ResetHard(ctx context.Context, ref string) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard", "destructive_allowed"}

    if !g.opts.AllowDestructive {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "ResetHard", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration: time.Since(start),
        })
        return fmt.Errorf("%w: ResetHard", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "ResetHard", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "ResetHard", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    branch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, "reset", "--hard", ref)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "ResetHard", RepoPath: g.path, Branch: branch,
        Args: []string{ref}, SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// Fetch fetches from a remote.
func (g *gitOps) Fetch(ctx context.Context, remote, ref string) error {
    if err := g.validateRuntime(ctx); err != nil {
        return err
    }
    args := []string{"fetch", remote}
    if ref != "" {
        args = append(args, ref)
    }
    _, err := g.exec(ctx, args...)
    return err
}

// Push pushes to a remote.
// Force/ForceWithLease are DESTRUCTIVE: Require AllowDestructive=true.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if (opts.Force || opts.ForceWithLease) && !g.opts.AllowDestructive {
        checks = append(checks, "destructive_allowed")
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Push", RepoPath: g.path,
            Branch: branch, Remote: remote, SafetyChecks: checks,
            ChecksPassed: false, FailureReason: ErrDestructiveNotAllowed.Error(),
            Duration: time.Since(start),
        })
        return fmt.Errorf("%w: Push --force", ErrDestructiveNotAllowed)
    }

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Push", RepoPath: g.path,
            Branch: branch, Remote: remote, SafetyChecks: checks,
            ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Push", RepoPath: g.path,
            Branch: branch, Remote: remote, SafetyChecks: checks,
            ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"push"}
    if opts.SetUpstream {
        args = append(args, "-u")
    }
    if opts.ForceWithLease {
        args = append(args, "--force-with-lease")
    } else if opts.Force {
        args = append(args, "--force")
    }
    args = append(args, remote, branch)

    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "Push", RepoPath: g.path, Branch: branch,
        Remote: remote, Args: args, SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// Merge merges a branch into the current branch.
// Validates branch guard and acquires per-repo lock.
func (g *gitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error {
    start := time.Now()
    checks := []string{"path_valid", "branch_guard"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Merge", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    if err := g.validateBranchGuard(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Merge", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    args := []string{"merge"}
    if opts.FFOnly {
        args = append(args, "--ff-only")
    }
    if opts.NoFF {
        args = append(args, "--no-ff")
    }
    if opts.NoCommit {
        args = append(args, "--no-commit")
    }
    if opts.Message != "" {
        args = append(args, "-m", opts.Message)
    }
    args = append(args, branch)

    currentBranch, _ := g.currentBranchInternal(ctx)
    _, err := g.exec(ctx, args...)

    g.audit(AuditEntry{
        Timestamp: start, Operation: "Merge", RepoPath: g.path, Branch: currentBranch,
        Args: args, SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}

// MergeAbort aborts an in-progress merge.
// Acquires per-repo lock.
func (g *gitOps) MergeAbort(ctx context.Context) error {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "MergeAbort", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return err
    }

    lock := getRepoLock(g.path)
    lock.Lock()
    defer lock.Unlock()

    _, err := g.exec(ctx, "merge", "--abort")

    g.audit(AuditEntry{
        Timestamp: start, Operation: "MergeAbort", RepoPath: g.path,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    return err
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run 'TestGitOps_Destructive|TestGitOps_Remote|TestGitOps_Merge' -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGitOps_DestructiveCheckoutFiles_Blocked` | Returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestGitOps_DestructiveClean_Blocked` | Returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestGitOps_DestructiveResetHard_Blocked` | Returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestGitOps_DestructiveCheckoutFiles_Allowed` | Discards changes when AllowDestructive=true |
| `TestGitOps_DestructiveClean_Allowed` | Removes untracked files when AllowDestructive=true |
| `TestGitOps_DestructiveResetHard_Allowed` | Resets to ref when AllowDestructive=true |
| `TestGitOps_RemoteForcePush_Blocked` | Returns ErrDestructiveNotAllowed for force push |
| `TestGitOps_MergeFastForward` | Fast-forward merge succeeds |
| `TestGitOps_MergeAbort` | Aborts in-progress merge |

### Test Implementation

```go
func TestGitOps_DestructiveCheckoutFiles_Blocked(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.CheckoutFiles(context.Background(), ".")
    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestGitOps_DestructiveClean_Blocked(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.Clean(context.Background(), CleanOpts{Force: true})
    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestGitOps_DestructiveResetHard_Blocked(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.ResetHard(context.Background(), "HEAD")
    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}

func TestGitOps_DestructiveCheckoutFiles_Allowed(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    filePath := filepath.Join(dir, "file.txt")
    os.WriteFile(filePath, []byte("original"), 0644)
    exec.Command("git", "-C", dir, "add", ".").Run()
    exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
    os.WriteFile(filePath, []byte("modified"), 0644)

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true,
    })

    err := ops.CheckoutFiles(context.Background(), "file.txt")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    content, _ := os.ReadFile(filePath)
    if string(content) != "original" {
        t.Errorf("expected 'original', got '%s'", content)
    }
}

func TestGitOps_DestructiveClean_Allowed(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
    untrackedPath := filepath.Join(dir, "untracked.txt")
    os.WriteFile(untrackedPath, []byte("untracked"), 0644)

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: true,
    })

    err := ops.Clean(context.Background(), CleanOpts{Force: true})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
        t.Error("expected untracked file to be removed")
    }
}

func TestGitOps_RemoteForcePush_Blocked(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{
        AllowRepoRoot:    true,
        AllowDestructive: false,
    })

    err := ops.Push(context.Background(), "origin", "main", PushOpts{Force: true})
    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (Push tests don't actually push)
- [x] Runs in <60 seconds

## Implementation Notes

- Destructive operations check AllowDestructive BEFORE validateRuntime
- Force push and force-with-lease both require AllowDestructive
- All operations that modify state acquire per-repo lock
- Audit entries include all safety checks performed

## NOT In Scope

- Remote URL validation (future enhancement)
- Stash operations (future enhancement)
- Rebase operations (future enhancement)
