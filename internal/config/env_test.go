package config

import (
	"testing"
)

func TestEnvOverrides_ClaudeCmd(t *testing.T) {
	cfg := &Config{Claude: ClaudeConfig{Command: "original"}}
	t.Setenv("RALPH_CLAUDE_CMD", "/custom/claude")

	applyEnvOverrides(cfg)

	if cfg.Claude.Command != "/custom/claude" {
		t.Errorf("expected Claude.Command to be '/custom/claude', got '%s'", cfg.Claude.Command)
	}
}

func TestEnvOverrides_WorktreeBase(t *testing.T) {
	cfg := &Config{Worktree: WorktreeConfig{BasePath: "original"}}
	t.Setenv("RALPH_WORKTREE_BASE", "/tmp/worktrees")

	applyEnvOverrides(cfg)

	if cfg.Worktree.BasePath != "/tmp/worktrees" {
		t.Errorf("expected Worktree.BasePath to be '/tmp/worktrees', got '%s'", cfg.Worktree.BasePath)
	}
}

func TestEnvOverrides_LogLevel(t *testing.T) {
	cfg := &Config{LogLevel: "info"}
	t.Setenv("RALPH_LOG_LEVEL", "debug")

	applyEnvOverrides(cfg)

	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel to be 'debug', got '%s'", cfg.LogLevel)
	}
}

func TestEnvOverrides_EmptyNoChange(t *testing.T) {
	cfg := &Config{
		Claude:   ClaudeConfig{Command: "original-claude"},
		Worktree: WorktreeConfig{BasePath: "original-worktree"},
		LogLevel: "original-level",
	}
	t.Setenv("RALPH_CLAUDE_CMD", "")
	t.Setenv("RALPH_WORKTREE_BASE", "")
	t.Setenv("RALPH_LOG_LEVEL", "")

	applyEnvOverrides(cfg)

	if cfg.Claude.Command != "original-claude" {
		t.Errorf("expected Claude.Command to remain 'original-claude', got '%s'", cfg.Claude.Command)
	}
	if cfg.Worktree.BasePath != "original-worktree" {
		t.Errorf("expected Worktree.BasePath to remain 'original-worktree', got '%s'", cfg.Worktree.BasePath)
	}
	if cfg.LogLevel != "original-level" {
		t.Errorf("expected LogLevel to remain 'original-level', got '%s'", cfg.LogLevel)
	}
}

func TestEnvOverrides_MultipleVars(t *testing.T) {
	cfg := &Config{
		Claude:   ClaudeConfig{Command: "original-claude"},
		Worktree: WorktreeConfig{BasePath: "original-worktree"},
		LogLevel: "info",
	}
	t.Setenv("RALPH_CLAUDE_CMD", "/new/claude")
	t.Setenv("RALPH_WORKTREE_BASE", "/new/worktree")
	t.Setenv("RALPH_LOG_LEVEL", "debug")

	applyEnvOverrides(cfg)

	if cfg.Claude.Command != "/new/claude" {
		t.Errorf("expected Claude.Command to be '/new/claude', got '%s'", cfg.Claude.Command)
	}
	if cfg.Worktree.BasePath != "/new/worktree" {
		t.Errorf("expected Worktree.BasePath to be '/new/worktree', got '%s'", cfg.Worktree.BasePath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel to be 'debug', got '%s'", cfg.LogLevel)
	}
}
