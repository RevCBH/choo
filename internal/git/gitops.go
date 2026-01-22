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

// Stub implementations for interface compliance - will be implemented in Tasks #6-7

func (g *gitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
	panic("not implemented")
}

func (g *gitOps) Add(ctx context.Context, paths ...string) error {
	panic("not implemented")
}

func (g *gitOps) AddAll(ctx context.Context) error {
	panic("not implemented")
}

func (g *gitOps) Reset(ctx context.Context, paths ...string) error {
	panic("not implemented")
}

func (g *gitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
	panic("not implemented")
}

func (g *gitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
	panic("not implemented")
}

func (g *gitOps) Clean(ctx context.Context, opts CleanOpts) error {
	panic("not implemented")
}

func (g *gitOps) ResetHard(ctx context.Context, ref string) error {
	panic("not implemented")
}

func (g *gitOps) Fetch(ctx context.Context, remote, ref string) error {
	panic("not implemented")
}

func (g *gitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
	panic("not implemented")
}

func (g *gitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error {
	panic("not implemented")
}

func (g *gitOps) MergeAbort(ctx context.Context) error {
	panic("not implemented")
}
