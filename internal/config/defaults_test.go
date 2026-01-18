package config

import "testing"

func TestDefaultConfig_TargetBranch(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TargetBranch != "main" {
		t.Errorf("expected TargetBranch to be 'main', got %q", cfg.TargetBranch)
	}
}

func TestDefaultConfig_Parallelism(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Parallelism != 4 {
		t.Errorf("expected Parallelism to be 4, got %d", cfg.Parallelism)
	}
}

func TestDefaultConfig_GitHubAuto(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.GitHub.Owner != "auto" {
		t.Errorf("expected GitHub.Owner to be 'auto', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "auto" {
		t.Errorf("expected GitHub.Repo to be 'auto', got %q", cfg.GitHub.Repo)
	}
}

func TestDefaultConfig_WorktreePath(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Worktree.BasePath != ".ralph/worktrees/" {
		t.Errorf("expected Worktree.BasePath to be '.ralph/worktrees/', got %q", cfg.Worktree.BasePath)
	}
}

func TestDefaultConfig_Claude(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Claude.Command != "claude" {
		t.Errorf("expected Claude.Command to be 'claude', got %q", cfg.Claude.Command)
	}
	if cfg.Claude.MaxTurns != 0 {
		t.Errorf("expected Claude.MaxTurns to be 0, got %d", cfg.Claude.MaxTurns)
	}
}

func TestDefaultConfig_Merge(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Merge.MaxConflictRetries != 3 {
		t.Errorf("expected Merge.MaxConflictRetries to be 3, got %d", cfg.Merge.MaxConflictRetries)
	}
}

func TestDefaultConfig_Review(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Review.Timeout != "2h" {
		t.Errorf("expected Review.Timeout to be '2h', got %q", cfg.Review.Timeout)
	}
	if cfg.Review.PollInterval != "30s" {
		t.Errorf("expected Review.PollInterval to be '30s', got %q", cfg.Review.PollInterval)
	}
}

func TestDefaultConfig_LogLevel(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel to be 'info', got %q", cfg.LogLevel)
	}
}
