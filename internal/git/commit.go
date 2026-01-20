package git

import (
	"context"
	"strings"
)

// CommitOptions configures a commit operation
type CommitOptions struct {
	// Message is the commit message
	Message string

	// NoVerify skips pre-commit hooks (default: true during tasks)
	NoVerify bool

	// AllowEmpty permits commits with no changes
	AllowEmpty bool
}

// Commit stages and commits changes in a worktree
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

// StageAll stages all changes in a worktree (git add -A)
func StageAll(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "add", "-A")
	return err
}

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges(ctx context.Context, worktreePath string) (bool, error) {
	output, err := gitExec(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	// If there's any output, there are uncommitted changes
	return strings.TrimSpace(output) != "", nil
}

// GetStagedFiles returns list of files staged for commit
func GetStagedFiles(ctx context.Context, worktreePath string) ([]string, error) {
	output, err := gitExec(ctx, worktreePath, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}

	// Split output by lines and filter empty lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// Push pushes the current branch to remote
func Push(ctx context.Context, worktreePath string) error {
	_, err := gitExec(ctx, worktreePath, "push")
	return err
}

// GetCommitHash retrieves the current HEAD commit hash
func GetCommitHash(ctx context.Context, worktreePath string) (string, error) {
	output, err := gitExec(ctx, worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
