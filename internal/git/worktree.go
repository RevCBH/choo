package git

import "github.com/anthropics/choo/internal/config"

// WorktreeManager manages git worktrees for parallel execution
type WorktreeManager struct {
	config *config.Config
}

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(cfg *config.Config) *WorktreeManager {
	return &WorktreeManager{
		config: cfg,
	}
}
