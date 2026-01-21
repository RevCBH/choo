package feature

import (
	"context"
	"fmt"

	"github.com/RevCBH/choo/internal/git"
)

// BranchManager handles feature branch operations
type BranchManager struct {
	git    *git.Client // Git client for operations
	prefix string      // Branch prefix (default: "feature/")
}

// NewBranchManager creates a branch manager with the given git client and prefix
// If prefix is empty, defaults to "feature/"
func NewBranchManager(gitClient *git.Client, prefix string) *BranchManager {
	if prefix == "" {
		prefix = "feature/"
	}
	return &BranchManager{
		git:    gitClient,
		prefix: prefix,
	}
}

// Create creates a new feature branch from the target branch
// Returns error if branch already exists
func (b *BranchManager) Create(ctx context.Context, prdID, fromBranch string) error {
	branchName := b.GetBranchName(prdID)

	// Check if branch already exists
	exists, err := b.Exists(ctx, prdID)
	if err != nil {
		return fmt.Errorf("checking if branch exists: %w", err)
	}
	if exists {
		return fmt.Errorf("feature branch %s already exists", branchName)
	}

	// Create the branch
	if err := b.git.CreateBranch(ctx, branchName, fromBranch); err != nil {
		return fmt.Errorf("creating branch %s from %s: %w", branchName, fromBranch, err)
	}

	return nil
}

// Exists checks if a feature branch exists locally or remotely
func (b *BranchManager) Exists(ctx context.Context, prdID string) (bool, error) {
	branchName := b.GetBranchName(prdID)
	return b.git.BranchExists(ctx, branchName)
}

// Checkout switches to the feature branch
// Returns error if branch does not exist
func (b *BranchManager) Checkout(ctx context.Context, prdID string) error {
	branchName := b.GetBranchName(prdID)

	// Check if branch exists
	exists, err := b.Exists(ctx, prdID)
	if err != nil {
		return fmt.Errorf("checking if branch exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("feature branch %s does not exist", branchName)
	}

	// Checkout the branch
	if err := b.git.Checkout(ctx, branchName); err != nil {
		return fmt.Errorf("checking out branch %s: %w", branchName, err)
	}

	return nil
}

// Delete removes a feature branch (typically after merge)
func (b *BranchManager) Delete(ctx context.Context, prdID string) error {
	branchName := b.GetBranchName(prdID)

	if err := b.git.DeleteBranch(ctx, branchName); err != nil {
		return fmt.Errorf("deleting branch %s: %w", branchName, err)
	}

	return nil
}

// GetBranchName returns the full branch name for a PRD ID
// Concatenates prefix + prdID (e.g., "feature/" + "streaming-events")
func (b *BranchManager) GetBranchName(prdID string) string {
	return b.prefix + prdID
}

// GetPrefix returns the configured branch prefix
func (b *BranchManager) GetPrefix() string {
	return b.prefix
}
