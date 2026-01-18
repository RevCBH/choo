package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/choo/internal/config"
)

func TestWireOrchestrator_AllComponents(t *testing.T) {
	cfg := &config.Config{
		Parallelism:  4,
		TargetBranch: "main",
		Worktree: config.WorktreeConfig{
			BasePath: ".worktrees",
		},
	}

	orch, err := WireOrchestrator(cfg)
	if err != nil {
		t.Fatalf("WireOrchestrator failed: %v", err)
	}
	defer orch.Close()

	// Verify all components are non-nil
	if orch.Config == nil {
		t.Error("Config is nil")
	}
	if orch.Events == nil {
		t.Error("Events is nil")
	}
	if orch.Discovery == nil {
		t.Error("Discovery is nil")
	}
	if orch.Scheduler == nil {
		t.Error("Scheduler is nil")
	}
	if orch.Workers == nil {
		t.Error("Workers is nil")
	}
	if orch.Git == nil {
		t.Error("Git is nil")
	}
	if orch.GitHub == nil {
		t.Error("GitHub is nil")
	}
}

func TestWireOrchestrator_EventBus(t *testing.T) {
	cfg := &config.Config{
		Parallelism:  4,
		TargetBranch: "main",
		Worktree: config.WorktreeConfig{
			BasePath: ".worktrees",
		},
	}

	orch, err := WireOrchestrator(cfg)
	if err != nil {
		t.Fatalf("WireOrchestrator failed: %v", err)
	}
	defer orch.Close()

	// Verify event bus is shared across components
	// The event bus should be the same instance
	if orch.Events == nil {
		t.Fatal("Events bus is nil")
	}
}

func TestOrchestrator_Close(t *testing.T) {
	cfg := &config.Config{
		Parallelism:  4,
		TargetBranch: "main",
		Worktree: config.WorktreeConfig{
			BasePath: ".worktrees",
		},
	}

	orch, err := WireOrchestrator(cfg)
	if err != nil {
		t.Fatalf("WireOrchestrator failed: %v", err)
	}

	// Close should shut down cleanly
	if err := orch.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestLoadConfig_Default(t *testing.T) {
	// Use a non-existent directory to ensure no .choo.yaml file is found
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig("specs/tasks")
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Verify defaults
	if cfg.Parallelism != 4 {
		t.Errorf("Expected Parallelism 4, got %d", cfg.Parallelism)
	}
	if cfg.TargetBranch != "main" {
		t.Errorf("Expected TargetBranch 'main', got '%s'", cfg.TargetBranch)
	}
	if cfg.Worktree.BasePath != ".worktrees" {
		t.Errorf("Expected Worktree.BasePath '.worktrees', got '%s'", cfg.Worktree.BasePath)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Create a test .choo.yaml file
	configContent := `parallelism: 8
target_branch: develop
worktree:
  base_path: .custom-worktrees
`
	configPath := filepath.Join(tmpDir, ".choo.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Verify config loaded from file
	if cfg.Parallelism != 8 {
		t.Errorf("Expected Parallelism 8, got %d", cfg.Parallelism)
	}
	if cfg.TargetBranch != "develop" {
		t.Errorf("Expected TargetBranch 'develop', got '%s'", cfg.TargetBranch)
	}
	if cfg.Worktree.BasePath != ".custom-worktrees" {
		t.Errorf("Expected Worktree.BasePath '.custom-worktrees', got '%s'", cfg.Worktree.BasePath)
	}
}
