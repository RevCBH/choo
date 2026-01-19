package worker

import (
	"context"
	"os/exec"
	"strings"
)

// getHeadRef returns the current HEAD commit SHA
func (w *Worker) getHeadRef(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hasNewCommit checks if HEAD has moved since the given ref
func (w *Worker) hasNewCommit(ctx context.Context, sinceRef string) (bool, error) {
	currentHead, err := w.getHeadRef(ctx)
	if err != nil {
		return false, err
	}
	return currentHead != sinceRef, nil
}

// branchExistsOnRemote checks if a branch exists on the remote
func (w *Worker) branchExistsOnRemote(ctx context.Context, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// getChangedFiles returns list of modified/added/deleted files
func (w *Worker) getChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = w.worktreePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) >= 3 {
			// Format: "XY filename" where XY is status
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}
