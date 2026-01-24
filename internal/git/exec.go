package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// Runner executes git commands.
type Runner interface {
	Exec(ctx context.Context, dir string, args ...string) (string, error)
	ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error)
}

// osRunner executes real git commands via exec.CommandContext.
type osRunner struct{}

func (osRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
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

func (osRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(stdin)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s",
			strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

var (
	defaultRunner Runner = osRunner{}
	runnerMu      sync.RWMutex
)

// DefaultRunner returns the current default runner.
func DefaultRunner() Runner {
	runnerMu.RLock()
	defer runnerMu.RUnlock()
	return defaultRunner
}

// SetDefaultRunner replaces the default runner. Intended for tests.
func SetDefaultRunner(runner Runner) {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	if runner == nil {
		defaultRunner = osRunner{}
		return
	}
	defaultRunner = runner
}

// gitExec executes a git command in the specified directory and returns stdout.
// Returns an error with stderr content if the command fails.
func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
	runnerMu.RLock()
	runner := defaultRunner
	runnerMu.RUnlock()
	return runner.Exec(ctx, dir, args...)
}

// gitExecWithStdin executes a git command with stdin input.
// Used for commands that require piped input.
//
//nolint:unused // WIP: will be used for commands requiring stdin input
func gitExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	runnerMu.RLock()
	runner := defaultRunner
	runnerMu.RUnlock()
	return runner.ExecWithStdin(ctx, dir, stdin, args...)
}

// Client provides git operations for a specific repository or worktree
type Client struct {
	// RepoPath is the root directory of the git repository
	RepoPath string
	// WorktreePath is an alias for RepoPath (for worktree operations)
	WorktreePath string
}

// NewClient creates a new git client for the given repository path
func NewClient(repoPath string) *Client {
	return &Client{
		RepoPath:     repoPath,
		WorktreePath: repoPath, // Alias for worktree operations
	}
}

// CreateBranch creates a new branch from the target branch
func (c *Client) CreateBranch(ctx context.Context, branchName, fromBranch string) error {
	_, err := gitExec(ctx, c.RepoPath, "branch", branchName, fromBranch)
	return err
}

// BranchExists checks if a branch exists locally or remotely
func (c *Client) BranchExists(ctx context.Context, branchName string) (bool, error) {
	// Check local branches
	_, err := gitExec(ctx, c.RepoPath, "rev-parse", "--verify", branchName)
	if err == nil {
		return true, nil
	}

	// Check remote branches
	_, err = gitExec(ctx, c.RepoPath, "rev-parse", "--verify", "origin/"+branchName)
	if err == nil {
		return true, nil
	}

	return false, nil
}

// Checkout switches to the specified branch
func (c *Client) Checkout(ctx context.Context, branchName string) error {
	_, err := gitExec(ctx, c.RepoPath, "checkout", branchName)
	return err
}

// DeleteBranch removes a branch locally
func (c *Client) DeleteBranch(ctx context.Context, branchName string) error {
	_, err := gitExec(ctx, c.RepoPath, "branch", "-D", branchName)
	return err
}

// ListLocalBranchesWithPrefix lists local branches matching the prefix.
func (c *Client) ListLocalBranchesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	ref := "refs/heads"
	trimmed := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if trimmed != "" {
		ref = ref + "/" + trimmed
	}
	output, err := gitExec(ctx, c.RepoPath, "for-each-ref", "--format=%(refname:short)", ref)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(line, prefix) {
			continue
		}
		branches = append(branches, line)
	}
	sort.Strings(branches)
	return branches, nil
}
