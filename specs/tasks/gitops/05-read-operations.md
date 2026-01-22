---
task: 5
status: pending
backpressure: "go test ./internal/git/... -run TestGitOps_Read -v"
depends_on: [4]
---

# Read Operations

**Parent spec**: `/specs/GITOPS.md`
**Task**: #5 of 7 in implementation plan

## Objective

Implement read-only git operations: Status, RevParse, Diff, Log, CurrentBranch, BranchExists.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #4 must be complete (provides: GitOps interface, gitOps struct, constructor)

### Package Dependencies
- Standard library (`context`, `fmt`, `strings`, `time`)

## Deliverables

### Files to Modify

```
internal/git/
├── gitops.go      # MODIFY: Add read operation methods
└── gitops_test.go # MODIFY: Add read operation tests
```

### Functions to Implement

```go
// validateRuntime re-validates path invariants before each operation.
func (g *gitOps) validateRuntime(ctx context.Context) error {
    info, err := os.Stat(g.path)
    if err != nil {
        return fmt.Errorf("runtime check failed: %w", err)
    }
    if !info.IsDir() {
        return fmt.Errorf("runtime check failed: path no longer a directory")
    }

    toplevel, err := g.runner.Exec(ctx, g.path, "rev-parse", "--show-toplevel")
    if err != nil {
        return fmt.Errorf("runtime check failed: not a git repo: %w", err)
    }
    if filepath.Clean(strings.TrimSpace(toplevel)) != g.path {
        return fmt.Errorf("runtime check failed: toplevel changed")
    }

    return nil
}

func (g *gitOps) exec(ctx context.Context, args ...string) (string, error) {
    return g.runner.Exec(ctx, g.path, args...)
}

func (g *gitOps) audit(entry AuditEntry) {
    if g.opts.AuditLogger != nil {
        g.opts.AuditLogger.Log(entry)
    }
}

// Status returns the current working tree status.
func (g *gitOps) Status(ctx context.Context) (StatusResult, error) {
    start := time.Now()
    checks := []string{"path_valid"}

    if err := g.validateRuntime(ctx); err != nil {
        g.audit(AuditEntry{
            Timestamp: start, Operation: "Status", RepoPath: g.path,
            SafetyChecks: checks, ChecksPassed: false, FailureReason: err.Error(),
            Duration: time.Since(start),
        })
        return StatusResult{}, err
    }

    out, err := g.exec(ctx, "status", "--porcelain")
    g.audit(AuditEntry{
        Timestamp: start, Operation: "Status", RepoPath: g.path,
        SafetyChecks: checks, ChecksPassed: err == nil,
        Duration: time.Since(start), Error: errorString(err),
    })

    if err != nil {
        return StatusResult{}, err
    }

    return parseStatusOutput(out), nil
}

func parseStatusOutput(out string) StatusResult {
    result := StatusResult{Clean: true}
    lines := strings.Split(out, "\n")

    for _, line := range lines {
        if len(line) < 3 {
            continue
        }
        result.Clean = false
        status := line[:2]
        file := strings.TrimSpace(line[3:])

        switch {
        case status[0] == 'U' || status[1] == 'U' || status == "AA" || status == "DD":
            result.Conflicted = append(result.Conflicted, file)
        case status[0] != ' ' && status[0] != '?':
            result.Staged = append(result.Staged, file)
        case status[1] == 'M':
            result.Modified = append(result.Modified, file)
        case status == "??":
            result.Untracked = append(result.Untracked, file)
        }
    }

    return result
}

// RevParse resolves a git ref to its SHA.
func (g *gitOps) RevParse(ctx context.Context, ref string) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    out, err := g.exec(ctx, "rev-parse", ref)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(out), nil
}

// Diff returns the diff between two refs.
func (g *gitOps) Diff(ctx context.Context, base, head string) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    return g.exec(ctx, "diff", base, head)
}

// Log returns commits matching the given options.
func (g *gitOps) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return nil, err
    }

    args := []string{"log", "--format=%H|%an|%aI|%s|%b%x00"}
    if opts.MaxCount > 0 {
        args = append(args, fmt.Sprintf("-n%d", opts.MaxCount))
    }
    if !opts.Since.IsZero() {
        args = append(args, "--since="+opts.Since.Format(time.RFC3339))
    }
    if !opts.Until.IsZero() {
        args = append(args, "--until="+opts.Until.Format(time.RFC3339))
    }
    if opts.Path != "" {
        args = append(args, "--", opts.Path)
    }

    out, err := g.exec(ctx, args...)
    if err != nil {
        return nil, err
    }

    return parseLogOutput(out), nil
}

func parseLogOutput(out string) []Commit {
    var commits []Commit
    entries := strings.Split(out, "\x00")
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        parts := strings.SplitN(entry, "|", 5)
        if len(parts) < 4 {
            continue
        }
        date, _ := time.Parse(time.RFC3339, parts[2])
        commit := Commit{
            Hash: parts[0], Author: parts[1], Date: date, Subject: parts[3],
        }
        if len(parts) == 5 {
            commit.Body = strings.TrimSpace(parts[4])
        }
        commits = append(commits, commit)
    }
    return commits
}

// CurrentBranch returns the name of the currently checked out branch.
func (g *gitOps) CurrentBranch(ctx context.Context) (string, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return "", err
    }
    return g.currentBranchInternal(ctx)
}

func (g *gitOps) currentBranchInternal(ctx context.Context) (string, error) {
    out, err := g.exec(ctx, "rev-parse", "--abbrev-ref", "HEAD")
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(out), nil
}

// BranchExists checks if a branch exists locally or on origin.
func (g *gitOps) BranchExists(ctx context.Context, branch string) (bool, error) {
    if err := g.validateRuntime(ctx); err != nil {
        return false, err
    }

    // Check local
    _, err := g.exec(ctx, "rev-parse", "--verify", branch)
    if err == nil {
        return true, nil
    }

    // Check remote
    _, err = g.exec(ctx, "rev-parse", "--verify", "origin/"+branch)
    return err == nil, nil
}

func errorString(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestGitOps_Read -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGitOps_Status_Clean` | `result.Clean == true` for clean repo |
| `TestGitOps_Status_Modified` | `result.Modified` contains modified files |
| `TestGitOps_Status_Staged` | `result.Staged` contains staged files |
| `TestGitOps_Status_Untracked` | `result.Untracked` contains untracked files |
| `TestGitOps_Status_Conflicted` | `result.Conflicted` contains conflicted files |
| `TestGitOps_RevParse` | Returns correct SHA for HEAD |
| `TestGitOps_CurrentBranch` | Returns current branch name |
| `TestGitOps_BranchExists_Local` | Returns true for existing local branch |
| `TestGitOps_BranchExists_NotFound` | Returns false for non-existent branch |

### Test Implementation

```go
func TestGitOps_ReadStatus_Clean(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    result, err := ops.Status(context.Background())

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !result.Clean {
        t.Error("expected Clean=true for fresh repo")
    }
}

func TestGitOps_ReadStatus_Modified(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", dir).Run()
    os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0644)
    exec.Command("git", "-C", dir, "add", ".").Run()
    exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
    os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    result, _ := ops.Status(context.Background())

    if result.Clean {
        t.Error("expected Clean=false")
    }
    if len(result.Modified) != 1 || result.Modified[0] != "file.txt" {
        t.Errorf("expected Modified=[file.txt], got %v", result.Modified)
    }
}

func TestGitOps_ReadCurrentBranch(t *testing.T) {
    dir := t.TempDir()
    exec.Command("git", "init", "-b", "main", dir).Run()
    exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

    ops, _ := NewGitOps(dir, GitOpsOpts{AllowRepoRoot: true})
    branch, err := ops.CurrentBranch(context.Background())

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if branch != "main" {
        t.Errorf("expected main, got %s", branch)
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All read operations call `validateRuntime` first
- Status output parsing handles all git status --porcelain codes
- Log uses null byte separator for reliable parsing of multi-line bodies
- Read operations do NOT acquire the per-repo lock (safe for concurrent reads)

## NOT In Scope

- Write operations (Task #6)
- Destructive operations (Task #7)
- Branch guard validation (only needed for write operations)
