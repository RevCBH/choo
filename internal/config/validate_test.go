package config

import (
	"strings"
	"testing"
)

func TestValidation_Parallelism_Zero(t *testing.T) {
	cfg := &Config{
		Parallelism: 0,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for zero parallelism")
	}
	if !strings.Contains(err.Error(), "parallelism") {
		t.Errorf("error should contain 'parallelism', got: %v", err)
	}
}

func TestValidation_Parallelism_Negative(t *testing.T) {
	cfg := &Config{
		Parallelism: -1,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for negative parallelism")
	}
	if !strings.Contains(err.Error(), "parallelism") {
		t.Errorf("error should contain 'parallelism', got: %v", err)
	}
}

func TestValidation_Parallelism_Valid(t *testing.T) {
	cfg := &Config{
		Parallelism: 1,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err != nil {
		t.Errorf("expected no error for valid parallelism, got: %v", err)
	}
}

func TestValidation_GitHubOwner_Empty(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty github.owner")
	}
	if !strings.Contains(err.Error(), "github.owner") {
		t.Errorf("error should contain 'github.owner', got: %v", err)
	}
}

func TestValidation_GitHubOwner_Auto(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "auto",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for github.owner still set to 'auto'")
	}
	if !strings.Contains(err.Error(), "github.owner") {
		t.Errorf("error should contain 'github.owner', got: %v", err)
	}
}

func TestValidation_GitHubRepo_Empty(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty github.repo")
	}
	if !strings.Contains(err.Error(), "github.repo") {
		t.Errorf("error should contain 'github.repo', got: %v", err)
	}
}

func TestValidation_ClaudeCommand_Empty(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty claude.command")
	}
	if !strings.Contains(err.Error(), "claude.command") {
		t.Errorf("error should contain 'claude.command', got: %v", err)
	}
}

func TestValidation_MaxTurns_Negative(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command:  "claude",
			MaxTurns: -1,
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for negative claude.max_turns")
	}
	if !strings.Contains(err.Error(), "claude.max_turns") {
		t.Errorf("error should contain 'claude.max_turns', got: %v", err)
	}
}

func TestValidation_MaxConflictRetries_Zero(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 0,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for zero max_conflict_retries")
	}
	if !strings.Contains(err.Error(), "max_conflict_retries") {
		t.Errorf("error should contain 'max_conflict_retries', got: %v", err)
	}
}

func TestValidation_ReviewTimeout_Invalid(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "invalid",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid review.timeout")
	}
	if !strings.Contains(err.Error(), "review.timeout") {
		t.Errorf("error should contain 'review.timeout', got: %v", err)
	}
}

func TestValidation_ReviewPollInterval_Invalid(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "invalid",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid review.poll_interval")
	}
	if !strings.Contains(err.Error(), "review.poll_interval") {
		t.Errorf("error should contain 'review.poll_interval', got: %v", err)
	}
}

func TestValidation_LogLevel_Invalid(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "invalid",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid log_level")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("error should contain 'log_level', got: %v", err)
	}
}

func TestValidation_LogLevel_CaseSensitive(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "DEBUG",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for uppercase log_level (case-sensitive)")
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("error should contain 'log_level', got: %v", err)
	}
}

func TestValidation_BaselineCheck_EmptyName(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
		BaselineChecks: []BaselineCheck{
			{
				Name:    "",
				Command: "go fmt",
			},
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty baseline_checks[0].name")
	}
	if !strings.Contains(err.Error(), "baseline_checks[0].name") {
		t.Errorf("error should contain 'baseline_checks[0].name', got: %v", err)
	}
}

func TestValidation_BaselineCheck_EmptyCommand(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
		BaselineChecks: []BaselineCheck{
			{
				Name:    "test",
				Command: "",
			},
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty baseline_checks[0].command")
	}
	if !strings.Contains(err.Error(), "baseline_checks[0].command") {
		t.Errorf("error should contain 'baseline_checks[0].command', got: %v", err)
	}
}

func TestValidation_SetupCommand_Empty(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Worktree: WorktreeConfig{
			SetupCommands: []ConditionalCommand{
				{
					Command: "",
				},
			},
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty worktree.setup[0].command")
	}
	if !strings.Contains(err.Error(), "worktree.setup[0].command") {
		t.Errorf("error should contain 'worktree.setup[0].command', got: %v", err)
	}
}

func TestValidation_TeardownCommand_Empty(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Worktree: WorktreeConfig{
			TeardownCommands: []ConditionalCommand{
				{
					Command: "",
				},
			},
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty worktree.teardown[0].command")
	}
	if !strings.Contains(err.Error(), "worktree.teardown[0].command") {
		t.Errorf("error should contain 'worktree.teardown[0].command', got: %v", err)
	}
}

func TestValidation_MultipleErrors(t *testing.T) {
	cfg := &Config{
		Parallelism: 0,
		GitHub: GitHubConfig{
			Owner: "",
			Repo:  "",
		},
		Claude: ClaudeConfig{
			Command: "",
		},
		Merge: MergeConfig{
			MaxConflictRetries: 0,
		},
		Review: ReviewConfig{
			Timeout:      "invalid",
			PollInterval: "invalid",
		},
		LogLevel: "invalid",
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for multiple validation failures")
	}

	// Check that multiple errors are joined
	errStr := err.Error()
	if !strings.Contains(errStr, "parallelism") {
		t.Errorf("error should contain 'parallelism', got: %v", err)
	}
	if !strings.Contains(errStr, "github.owner") {
		t.Errorf("error should contain 'github.owner', got: %v", err)
	}
	if !strings.Contains(errStr, "github.repo") {
		t.Errorf("error should contain 'github.repo', got: %v", err)
	}
	if !strings.Contains(errStr, "claude.command") {
		t.Errorf("error should contain 'claude.command', got: %v", err)
	}
}

func TestValidation_ValidConfig(t *testing.T) {
	cfg := &Config{
		Parallelism: 4,
		GitHub: GitHubConfig{
			Owner: "test",
			Repo:  "repo",
		},
		Claude: ClaudeConfig{
			Command:  "claude",
			MaxTurns: 0,
		},
		Merge: MergeConfig{
			MaxConflictRetries: 3,
		},
		Review: ReviewConfig{
			Timeout:      "2h",
			PollInterval: "30s",
		},
		LogLevel: "info",
	}

	err := validateConfig(cfg)
	if err != nil {
		t.Errorf("expected no error for fully valid config, got: %v", err)
	}
}
