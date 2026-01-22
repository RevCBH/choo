package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Stub implementations for interface compliance - will be implemented in Tasks #5-7

func (g *gitOps) Status(ctx context.Context) (StatusResult, error) {
	panic("not implemented")
}

func (g *gitOps) RevParse(ctx context.Context, ref string) (string, error) {
	panic("not implemented")
}

func (g *gitOps) Diff(ctx context.Context, base, head string) (string, error) {
	panic("not implemented")
}

func (g *gitOps) Log(ctx context.Context, opts LogOpts) ([]CommitRecord, error) {
	panic("not implemented")
}

func (g *gitOps) CurrentBranch(ctx context.Context) (string, error) {
	panic("not implemented")
}

func (g *gitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
	panic("not implemented")
}

func (g *gitOps) BranchExists(ctx context.Context, branch string) (bool, error) {
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
