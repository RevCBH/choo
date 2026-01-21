package feature

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/RevCBH/choo/internal/git"
)

// CommitResult holds the result of the commit operation
type CommitResult struct {
	CommitHash string
	FileCount  int
	Pushed     bool
}

// CommitOptions configures the commit operation
type CommitOptions struct {
	PushRetries int  // default 1 (one retry)
	DryRun      bool
}

// CommitSpecs stages and commits generated specs to the feature branch
func CommitSpecs(ctx context.Context, gitClient *git.Client, prdID string) (*CommitResult, error) {
	return CommitSpecsWithOptions(ctx, gitClient, prdID, CommitOptions{
		PushRetries: 1,
		DryRun:      false,
	})
}

// CommitSpecsWithOptions commits with custom options
func CommitSpecsWithOptions(ctx context.Context, gitClient *git.Client, prdID string, opts CommitOptions) (*CommitResult, error) {
	specsDir := filepath.Join("specs/tasks", prdID)

	if opts.DryRun {
		// In dry-run mode, just log what would happen
		return &CommitResult{
			CommitHash: "",
			FileCount:  0,
			Pushed:     false,
		}, nil
	}

	// Stage only the specs directory (not the entire worktree)
	if err := git.StagePath(ctx, gitClient.WorktreePath, specsDir); err != nil {
		return nil, fmt.Errorf("failed to stage specs: %w", err)
	}

	// Get count of staged files
	stagedFiles, err := git.GetStagedFiles(ctx, gitClient.WorktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}
	fileCount := len(stagedFiles)

	// Commit with standardized message (no Claude invocation)
	commitMsg := generateCommitMessage(prdID)
	if err := git.Commit(ctx, gitClient.WorktreePath, git.CommitOptions{
		Message:  commitMsg,
		NoVerify: true,
	}); err != nil {
		return nil, fmt.Errorf("failed to commit specs: %w", err)
	}

	// Get the commit hash
	commitHash, err := git.GetCommitHash(ctx, gitClient.WorktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}

	// Push to remote feature branch with retry
	pushed := false
	var pushErr error
	for i := 0; i <= opts.PushRetries; i++ {
		if pushErr = git.Push(ctx, gitClient.WorktreePath); pushErr == nil {
			pushed = true
			break
		}
	}
	if pushErr != nil {
		return nil, fmt.Errorf("failed to push specs after retry: %w", pushErr)
	}

	return &CommitResult{
		CommitHash: commitHash,
		FileCount:  fileCount,
		Pushed:     pushed,
	}, nil
}

// generateCommitMessage returns the standardized commit message
// Format: "chore(feature): add specs for <prd-id>"
func generateCommitMessage(prdID string) string {
	return fmt.Sprintf("chore(feature): add specs for %s", prdID)
}
