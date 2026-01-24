package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file with the given content for testing
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Create temp directory with git repo (for GitHub detection)
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Load config with no file
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify defaults
	if cfg.TargetBranch != DefaultTargetBranch {
		t.Errorf("expected TargetBranch to be %q, got %q", DefaultTargetBranch, cfg.TargetBranch)
	}
	if cfg.Parallelism != DefaultParallelism {
		t.Errorf("expected Parallelism to be %d, got %d", DefaultParallelism, cfg.Parallelism)
	}
	if cfg.GitHub.Owner != "testowner" {
		t.Errorf("expected GitHub.Owner to be 'testowner', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "testrepo" {
		t.Errorf("expected GitHub.Repo to be 'testrepo', got %q", cfg.GitHub.Repo)
	}
	expectedPath := filepath.Join(dir, DefaultWorktreeBasePath)
	if cfg.Worktree.BasePath != expectedPath {
		t.Errorf("expected Worktree.BasePath to be %q, got %q", expectedPath, cfg.Worktree.BasePath)
	}
	if cfg.Claude.Command != DefaultClaudeCommand {
		t.Errorf("expected Claude.Command to be %q, got %q", DefaultClaudeCommand, cfg.Claude.Command)
	}
	if cfg.Claude.MaxTurns != DefaultClaudeMaxTurns {
		t.Errorf("expected Claude.MaxTurns to be %d, got %d", DefaultClaudeMaxTurns, cfg.Claude.MaxTurns)
	}
	if cfg.Merge.MaxConflictRetries != DefaultMaxConflictRetries {
		t.Errorf("expected Merge.MaxConflictRetries to be %d, got %d", DefaultMaxConflictRetries, cfg.Merge.MaxConflictRetries)
	}
	if cfg.Review.Timeout != DefaultReviewTimeout {
		t.Errorf("expected Review.Timeout to be %q, got %q", DefaultReviewTimeout, cfg.Review.Timeout)
	}
	if cfg.Review.PollInterval != DefaultReviewPollInterval {
		t.Errorf("expected Review.PollInterval to be %q, got %q", DefaultReviewPollInterval, cfg.Review.PollInterval)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("expected LogLevel to be %q, got %q", DefaultLogLevel, cfg.LogLevel)
	}
}

func TestLoadConfig_FileOverrides(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config file
	configContent := `
target_branch: develop
parallelism: 8
github:
  owner: myorg
  repo: myrepo
worktree:
  base_path: /tmp/worktrees
claude:
  command: custom-claude
  max_turns: 100
merge:
  max_conflict_retries: 5
review:
  timeout: 4h
  poll_interval: 1m
log_level: debug
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify overrides
	if cfg.TargetBranch != "develop" {
		t.Errorf("expected TargetBranch to be 'develop', got %q", cfg.TargetBranch)
	}
	if cfg.Parallelism != 8 {
		t.Errorf("expected Parallelism to be 8, got %d", cfg.Parallelism)
	}
	if cfg.GitHub.Owner != "myorg" {
		t.Errorf("expected GitHub.Owner to be 'myorg', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "myrepo" {
		t.Errorf("expected GitHub.Repo to be 'myrepo', got %q", cfg.GitHub.Repo)
	}
	if cfg.Worktree.BasePath != "/tmp/worktrees" {
		t.Errorf("expected Worktree.BasePath to be '/tmp/worktrees', got %q", cfg.Worktree.BasePath)
	}
	if cfg.Claude.Command != "custom-claude" {
		t.Errorf("expected Claude.Command to be 'custom-claude', got %q", cfg.Claude.Command)
	}
	if cfg.Claude.MaxTurns != 100 {
		t.Errorf("expected Claude.MaxTurns to be 100, got %d", cfg.Claude.MaxTurns)
	}
	if cfg.Merge.MaxConflictRetries != 5 {
		t.Errorf("expected Merge.MaxConflictRetries to be 5, got %d", cfg.Merge.MaxConflictRetries)
	}
	if cfg.Review.Timeout != "4h" {
		t.Errorf("expected Review.Timeout to be '4h', got %q", cfg.Review.Timeout)
	}
	if cfg.Review.PollInterval != "1m" {
		t.Errorf("expected Review.PollInterval to be '1m', got %q", cfg.Review.PollInterval)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel to be 'debug', got %q", cfg.LogLevel)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config file
	configContent := `
claude:
  command: file-claude
worktree:
  base_path: file-path
log_level: info
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	// Set environment variables
	t.Setenv("RALPH_CLAUDE_CMD", "env-claude")
	t.Setenv("RALPH_WORKTREE_BASE", "env-path")
	t.Setenv("RALPH_LOG_LEVEL", "error")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify env vars override file
	if cfg.Claude.Command != "env-claude" {
		t.Errorf("expected Claude.Command to be 'env-claude', got %q", cfg.Claude.Command)
	}
	expectedPath := filepath.Join(dir, "env-path")
	if cfg.Worktree.BasePath != expectedPath {
		t.Errorf("expected Worktree.BasePath to be %q, got %q", expectedPath, cfg.Worktree.BasePath)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("expected LogLevel to be 'error', got %q", cfg.LogLevel)
	}
}

func TestLoadConfig_PathResolution(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with relative path
	configContent := `
worktree:
  base_path: relative/worktrees
github:
  owner: myorg
  repo: myrepo
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify path is now absolute
	expectedPath := filepath.Join(dir, "relative/worktrees")
	if cfg.Worktree.BasePath != expectedPath {
		t.Errorf("expected Worktree.BasePath to be %q, got %q", expectedPath, cfg.Worktree.BasePath)
	}
	if !filepath.IsAbs(cfg.Worktree.BasePath) {
		t.Errorf("expected Worktree.BasePath to be absolute, got %q", cfg.Worktree.BasePath)
	}
}

func TestLoadConfig_GitHubAutoDetect(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "git@github.com:detected/fromgit.git", nil)

	// Config with auto values (default)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auto-detection happened
	if cfg.GitHub.Owner != "detected" {
		t.Errorf("expected GitHub.Owner to be 'detected', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "fromgit" {
		t.Errorf("expected GitHub.Repo to be 'fromgit', got %q", cfg.GitHub.Repo)
	}
}

func TestLoadConfig_GitHubExplicit(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/detected/fromgit.git", nil)

	// Write config with explicit values
	configContent := `
github:
  owner: explicit-owner
  repo: explicit-repo
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify explicit values were NOT overwritten
	if cfg.GitHub.Owner != "explicit-owner" {
		t.Errorf("expected GitHub.Owner to be 'explicit-owner', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "explicit-repo" {
		t.Errorf("expected GitHub.Repo to be 'explicit-repo', got %q", cfg.GitHub.Repo)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write invalid YAML
	writeFile(t, filepath.Join(dir, ".choo.yaml"), "invalid: yaml: content: [")

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("expected error to contain 'parse config', got %q", err.Error())
	}
}

func TestLoadConfig_ValidationError(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with invalid values
	configContent := `
parallelism: 0
github:
  owner: valid
  repo: valid
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "validate config") {
		t.Errorf("expected error to contain 'validate config', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "parallelism") {
		t.Errorf("expected error to contain 'parallelism', got %q", err.Error())
	}
}

func TestLoadConfig_NoGitRemote(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "", errors.New("no git remote"))

	// Config tries to auto-detect (default behavior)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when auto-detect fails, got nil")
	}
	if !strings.Contains(err.Error(), "auto-detect github") {
		t.Errorf("expected error to contain 'auto-detect github', got %q", err.Error())
	}
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write partial config (only some fields)
	configContent := `
parallelism: 2
log_level: warn
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify partial overrides
	if cfg.Parallelism != 2 {
		t.Errorf("expected Parallelism to be 2, got %d", cfg.Parallelism)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected LogLevel to be 'warn', got %q", cfg.LogLevel)
	}

	// Verify defaults for unspecified fields
	if cfg.TargetBranch != DefaultTargetBranch {
		t.Errorf("expected TargetBranch to be %q, got %q", DefaultTargetBranch, cfg.TargetBranch)
	}
	if cfg.Claude.Command != DefaultClaudeCommand {
		t.Errorf("expected Claude.Command to be %q, got %q", DefaultClaudeCommand, cfg.Claude.Command)
	}
	// GitHub should be auto-detected
	if cfg.GitHub.Owner != "testowner" {
		t.Errorf("expected GitHub.Owner to be 'testowner', got %q", cfg.GitHub.Owner)
	}
	if cfg.GitHub.Repo != "testrepo" {
		t.Errorf("expected GitHub.Repo to be 'testrepo', got %q", cfg.GitHub.Repo)
	}
}

func TestLoadConfig_BaselineChecks(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with baseline checks
	configContent := `
github:
  owner: test
  repo: test
baseline_checks:
  - name: go-fmt
    command: go fmt ./...
    pattern: "*.go"
  - name: go-vet
    command: go vet ./...
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify baseline checks parsed
	if len(cfg.BaselineChecks) != 2 {
		t.Fatalf("expected 2 baseline checks, got %d", len(cfg.BaselineChecks))
	}
	if cfg.BaselineChecks[0].Name != "go-fmt" {
		t.Errorf("expected BaselineChecks[0].Name to be 'go-fmt', got %q", cfg.BaselineChecks[0].Name)
	}
	if cfg.BaselineChecks[0].Command != "go fmt ./..." {
		t.Errorf("expected BaselineChecks[0].Command to be 'go fmt ./...', got %q", cfg.BaselineChecks[0].Command)
	}
	if cfg.BaselineChecks[0].Pattern != "*.go" {
		t.Errorf("expected BaselineChecks[0].Pattern to be '*.go', got %q", cfg.BaselineChecks[0].Pattern)
	}
	if cfg.BaselineChecks[1].Name != "go-vet" {
		t.Errorf("expected BaselineChecks[1].Name to be 'go-vet', got %q", cfg.BaselineChecks[1].Name)
	}
	if cfg.BaselineChecks[1].Command != "go vet ./..." {
		t.Errorf("expected BaselineChecks[1].Command to be 'go vet ./...', got %q", cfg.BaselineChecks[1].Command)
	}
	if cfg.BaselineChecks[1].Pattern != "" {
		t.Errorf("expected BaselineChecks[1].Pattern to be '', got %q", cfg.BaselineChecks[1].Pattern)
	}
}

func TestLoadConfig_ConditionalCommands(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with conditional commands
	configContent := `
github:
  owner: test
  repo: test
worktree:
  setup:
    - command: npm install
      if: package.json
    - command: go mod download
  teardown:
    - command: make clean
      if: Makefile
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify setup commands parsed
	if len(cfg.Worktree.SetupCommands) != 2 {
		t.Fatalf("expected 2 setup commands, got %d", len(cfg.Worktree.SetupCommands))
	}
	if cfg.Worktree.SetupCommands[0].Command != "npm install" {
		t.Errorf("expected SetupCommands[0].Command to be 'npm install', got %q", cfg.Worktree.SetupCommands[0].Command)
	}
	if cfg.Worktree.SetupCommands[0].If != "package.json" {
		t.Errorf("expected SetupCommands[0].If to be 'package.json', got %q", cfg.Worktree.SetupCommands[0].If)
	}
	if cfg.Worktree.SetupCommands[1].Command != "go mod download" {
		t.Errorf("expected SetupCommands[1].Command to be 'go mod download', got %q", cfg.Worktree.SetupCommands[1].Command)
	}
	if cfg.Worktree.SetupCommands[1].If != "" {
		t.Errorf("expected SetupCommands[1].If to be '', got %q", cfg.Worktree.SetupCommands[1].If)
	}

	// Verify teardown commands parsed
	if len(cfg.Worktree.TeardownCommands) != 1 {
		t.Fatalf("expected 1 teardown command, got %d", len(cfg.Worktree.TeardownCommands))
	}
	if cfg.Worktree.TeardownCommands[0].Command != "make clean" {
		t.Errorf("expected TeardownCommands[0].Command to be 'make clean', got %q", cfg.Worktree.TeardownCommands[0].Command)
	}
	if cfg.Worktree.TeardownCommands[0].If != "Makefile" {
		t.Errorf("expected TeardownCommands[0].If to be 'Makefile', got %q", cfg.Worktree.TeardownCommands[0].If)
	}
}

func TestCodeReviewConfig_Validate_ValidCodex(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:          true,
		Provider:         ReviewProviderCodex,
		MaxFixIterations: 1,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestCodeReviewConfig_Validate_ValidClaude(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:          true,
		Provider:         ReviewProviderClaude,
		MaxFixIterations: 3,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestCodeReviewConfig_Validate_DisabledInvalidProvider(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:  false,
		Provider: "invalid",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil (invalid provider allowed when disabled)", err)
	}
}

func TestCodeReviewConfig_Validate_InvalidProvider(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:  true,
		Provider: "gpt4",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid provider")
	}
	if !strings.Contains(err.Error(), "invalid review provider") {
		t.Errorf("error should mention 'invalid review provider', got: %v", err)
	}
}

func TestCodeReviewConfig_Validate_NegativeIterations(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:          true,
		Provider:         ReviewProviderCodex,
		MaxFixIterations: -1,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() error = nil, want error for negative iterations")
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Errorf("error should mention 'negative', got: %v", err)
	}
}

func TestCodeReviewConfig_Validate_ZeroIterations(t *testing.T) {
	cfg := CodeReviewConfig{
		Enabled:          true,
		Provider:         ReviewProviderCodex,
		MaxFixIterations: 0,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil (0 iterations is valid review-only mode)", err)
	}
}

func TestCodeReviewConfig_IsReviewOnlyMode(t *testing.T) {
	tests := []struct {
		iterations int
		want       bool
	}{
		{0, true},
		{1, false},
		{5, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("iterations=%d", tt.iterations), func(t *testing.T) {
			cfg := CodeReviewConfig{MaxFixIterations: tt.iterations}
			if got := cfg.IsReviewOnlyMode(); got != tt.want {
				t.Errorf("IsReviewOnlyMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultCodeReviewConfig(t *testing.T) {
	cfg := DefaultCodeReviewConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if cfg.Provider != ReviewProviderCodex {
		t.Errorf("expected Provider to be %q, got %q", ReviewProviderCodex, cfg.Provider)
	}
	if cfg.MaxFixIterations != 1 {
		t.Errorf("expected MaxFixIterations to be 1, got %d", cfg.MaxFixIterations)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose to be true by default (noisy)")
	}
	if cfg.Command != "" {
		t.Errorf("expected Command to be empty, got %q", cfg.Command)
	}
}

func TestLoadConfig_MissingCodeReview(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with no code_review section
	configContent := `
github:
  owner: test
  repo: test
parallelism: 4
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should have defaults applied
	if !cfg.CodeReview.Enabled {
		t.Error("expected CodeReview.Enabled to default to true")
	}
	if cfg.CodeReview.Provider != ReviewProviderCodex {
		t.Errorf("expected CodeReview.Provider to default to %q, got %q", ReviewProviderCodex, cfg.CodeReview.Provider)
	}
	if cfg.CodeReview.MaxFixIterations != 1 {
		t.Errorf("expected CodeReview.MaxFixIterations to default to 1, got %d", cfg.CodeReview.MaxFixIterations)
	}
}

func TestLoadConfig_PartialCodeReview(t *testing.T) {
	dir := t.TempDir()
	stubGitRemote(t, "https://github.com/testowner/testrepo.git", nil)

	// Write config with partial code_review section
	configContent := `
github:
  owner: test
  repo: test
code_review:
  provider: claude
  max_fix_iterations: 3
`
	writeFile(t, filepath.Join(dir, ".choo.yaml"), configContent)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Explicit values should be used
	if cfg.CodeReview.Provider != ReviewProviderClaude {
		t.Errorf("expected Provider to be %q, got %q", ReviewProviderClaude, cfg.CodeReview.Provider)
	}
	if cfg.CodeReview.MaxFixIterations != 3 {
		t.Errorf("expected MaxFixIterations to be 3, got %d", cfg.CodeReview.MaxFixIterations)
	}

	// Other fields should have defaults
	if !cfg.CodeReview.Enabled {
		t.Error("expected Enabled to default to true")
	}
	if !cfg.CodeReview.Verbose {
		t.Error("expected Verbose to default to true")
	}
}

func TestValidateConfig_SpecRepairModelMismatch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GitHub.Owner = "owner"
	cfg.GitHub.Repo = "repo"
	cfg.SpecRepair.Provider = ProviderClaude
	cfg.SpecRepair.Model = "gpt-4o"

	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for model/provider mismatch")
	}
	if !strings.Contains(err.Error(), "spec_repair.model") {
		t.Fatalf("expected spec_repair.model in error, got: %v", err)
	}
}
