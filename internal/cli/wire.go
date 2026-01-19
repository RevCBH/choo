package cli

import (
	"fmt"
	"os"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/scheduler"
	"github.com/RevCBH/choo/internal/worker"
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

	// Create Worker Pool (depends on event bus and git manager)
	workers := worker.New(cfg.Parallelism, eventBus, gitManager)

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
