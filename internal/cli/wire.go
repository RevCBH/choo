package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/anthropics/choo/internal/claude"
	"github.com/anthropics/choo/internal/config"
	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
	"github.com/anthropics/choo/internal/scheduler"
	"github.com/anthropics/choo/internal/worker"
	"gopkg.in/yaml.v3"
)

// Orchestrator holds all wired components
type Orchestrator struct {
	Config    *config.Config
	Events    *events.Bus
	Discovery *discovery.Discovery
	Scheduler *scheduler.Scheduler
	Workers   *worker.Pool
	Git       *git.WorktreeManager
	GitHub    *github.PRClient
}

// WireOrchestrator assembles all components for orchestration
func WireOrchestrator(cfg *config.Config) (*Orchestrator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Get repository root
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create event bus first (other components depend on it)
	eventBus := events.NewBus(1000)

	// Create discovery (no dependencies)
	disc := &discovery.Discovery{}

	// Create Git WorktreeManager
	gitManager := git.NewWorktreeManager(wd, nil)

	// Create GitHub PRClient
	pollInterval, _ := cfg.ReviewPollIntervalDuration()
	reviewTimeout, _ := cfg.ReviewTimeoutDuration()
	ghClient, err := github.NewPRClient(github.PRClientConfig{
		Owner:         cfg.GitHub.Owner,
		Repo:          cfg.GitHub.Repo,
		PollInterval:  pollInterval,
		ReviewTimeout: reviewTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Create Scheduler (depends on event bus)
	sched := scheduler.New(eventBus, cfg.Parallelism)

	// Create Claude client
	claudeClient := claude.NewCLIClient()

	// Create Worker Config
	workerCfg := worker.WorkerConfig{
		RepoRoot:            wd,
		TargetBranch:        cfg.TargetBranch,
		WorktreeBase:        cfg.Worktree.BasePath,
		MaxClaudeRetries:    3,
		MaxBaselineRetries:  3,
		BackpressureTimeout: 5 * time.Minute,
		BaselineTimeout:     10 * time.Minute,
		NoPR:                false,
	}

	// Create Worker Dependencies
	workerDeps := worker.WorkerDeps{
		Events: eventBus,
		Git:    gitManager,
		GitHub: ghClient,
		Claude: claudeClient,
	}

	// Create Worker Pool
	workers := worker.NewPool(cfg.Parallelism, workerCfg, workerDeps)

	return &Orchestrator{
		Config:    cfg,
		Events:    eventBus,
		Discovery: disc,
		Scheduler: sched,
		Workers:   workers,
		Git:       gitManager,
		GitHub:    ghClient,
	}, nil
}

// Close shuts down all orchestrator components
func (o *Orchestrator) Close() error {
	// Stop workers first
	if o.Workers != nil {
		if err := o.Workers.Stop(); err != nil {
			return fmt.Errorf("failed to stop workers: %w", err)
		}
	}

	// Close event bus
	if o.Events != nil {
		o.Events.Close()
	}

	return nil
}

// loadConfig loads configuration from .choo.yaml or defaults
func loadConfig(tasksDir string) (*config.Config, error) {
	// Look for .choo.yaml in current directory
	configPath := ".choo.yaml"
	if _, err := os.Stat(configPath); err == nil {
		// File exists, load it
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var cfg config.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		return &cfg, nil
	}

	// Fall back to defaults
	return &config.Config{
		Parallelism:  4,
		TargetBranch: "main",
		Worktree: config.WorktreeConfig{
			BasePath: ".worktrees",
		},
	}, nil
}
