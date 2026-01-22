package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitOps provides safe git operations bound to a specific repository path.
// All operations are validated to prevent accidental execution in wrong directories.
type GitOps interface {
	Path() string

	// Read operations
	Status(ctx context.Context) (StatusResult, error)
	RevParse(ctx context.Context, ref string) (string, error)
	Diff(ctx context.Context, base, head string) (string, error)
	Log(ctx context.Context, opts LogOpts) ([]CommitRecord, error)

	// Branch operations
	CurrentBranch(ctx context.Context) (string, error)
	CheckoutBranch(ctx context.Context, branch string, create bool) error
	BranchExists(ctx context.Context, branch string) (bool, error)

	// Staging operations
	Add(ctx context.Context, paths ...string) error
	AddAll(ctx context.Context) error
	Reset(ctx context.Context, paths ...string) error

	// Commit operations
	Commit(ctx context.Context, msg string, opts CommitOpts) error

	// Working tree operations (destructive)
	CheckoutFiles(ctx context.Context, paths ...string) error
	Clean(ctx context.Context, opts CleanOpts) error
	ResetHard(ctx context.Context, ref string) error

	// Remote operations
	Fetch(ctx context.Context, remote, ref string) error
	Push(ctx context.Context, remote, branch string, opts PushOpts) error

	// Merge operations
	Merge(ctx context.Context, branch string, opts MergeOpts) error
	MergeAbort(ctx context.Context) error
}

// gitOps implements GitOps with path validation and safety checks.
type gitOps struct {
	path   string // Canonical, absolute path
	opts   GitOpsOpts
	runner Runner
}

// NewGitOps creates a GitOps bound to a specific path with safety options.
// Performs 9 validation checks at construction time.
func NewGitOps(path string, opts GitOpsOpts) (GitOps, error) {
	return newGitOpsWithRunner(path, opts, DefaultRunner())
}

func newGitOpsWithRunner(path string, opts GitOpsOpts, runner Runner) (GitOps, error) {
	// 1. Non-empty path
	if path == "" {
		return nil, ErrEmptyPath
	}

	// 2. Absolute path
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("%w: %s", ErrRelativePath, path)
	}

	// 3. Path exists (check before resolving symlinks, as EvalSymlinks fails on non-existent paths)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPathNotFound, err)
	}

	// 4. Path is directory (check before resolving symlinks)
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, path)
	}

	// 5. Canonical path (resolve symlinks, clean)
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNonCanonicalPath, err)
	}
	canonical = filepath.Clean(canonical)

	// 6. Valid git worktree
	toplevel, err := runner.Exec(context.Background(), canonical, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotGitRepo, err)
	}
	toplevel = strings.TrimSpace(toplevel)

	// 7. Path matches toplevel
	if filepath.Clean(toplevel) != canonical {
		return nil, fmt.Errorf("%w: toplevel=%s, path=%s", ErrPathMismatch, toplevel, canonical)
	}

	// 8. Worktree not repo root (unless AllowRepoRoot)
	if !opts.AllowRepoRoot {
		gitDir, err := runner.Exec(context.Background(), canonical, "rev-parse", "--absolute-git-dir")
		if err == nil {
			gitDir = strings.TrimSpace(gitDir)
			if !strings.Contains(gitDir, "worktrees") {
				return nil, ErrRepoRootNotAllowed
			}
		}
	}

	// 9. Under worktree base (unless AllowRepoRoot)
	if !opts.AllowRepoRoot && opts.WorktreeBase != "" {
		base, err := filepath.EvalSymlinks(opts.WorktreeBase)
		if err == nil {
			base = filepath.Clean(base)
			if !strings.HasPrefix(canonical, base+string(filepath.Separator)) && canonical != base {
				return nil, fmt.Errorf("%w: path=%s, base=%s", ErrOutsideWorktreeBase, canonical, base)
			}
		}
	}

	return &gitOps{
		path:   canonical,
		opts:   opts,
		runner: runner,
	}, nil
}

// NewWorktreeGitOps creates a GitOps for worktree operations.
func NewWorktreeGitOps(path string, worktreeBase string) (GitOps, error) {
	return NewGitOps(path, GitOpsOpts{
		WorktreeBase:     worktreeBase,
		AllowRepoRoot:    false,
		AllowDestructive: true,
		SafetyLevel:      SafetyDefault,
	})
}

// NewRepoRootGitOps creates a GitOps for repo root operations.
func NewRepoRootGitOps(path string, guard *BranchGuard) (GitOps, error) {
	return NewGitOps(path, GitOpsOpts{
		AllowRepoRoot:    true,
		AllowDestructive: false,
		BranchGuard:      guard,
		SafetyLevel:      SafetyStrict,
	})
}

func (g *gitOps) Path() string {
	return g.path
}

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

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
func (g *gitOps) Log(ctx context.Context, opts LogOpts) ([]CommitRecord, error) {
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

func parseLogOutput(out string) []CommitRecord {
	var commits []CommitRecord
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
		commit := CommitRecord{
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
