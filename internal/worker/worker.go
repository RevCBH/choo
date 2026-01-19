package worker

import (
	"time"

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
)

// Worker executes a single unit in an isolated worktree
type Worker struct {
	unit         *discovery.Unit
	config       WorkerConfig
	events       *events.Bus
	git          *git.WorktreeManager
	github       *github.PRClient
	claude       *ClaudeClient
	worktreePath string
	branch       string
	currentTask  *discovery.Task
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	RepoRoot            string
	TargetBranch        string
	WorktreeBase        string
	BaselineChecks      []BaselineCheck
	MaxClaudeRetries    int
	MaxBaselineRetries  int
	BackpressureTimeout time.Duration
	BaselineTimeout     time.Duration
	NoPR                bool
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
	Name    string
	Command string
	Pattern string
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
	Events *events.Bus
	Git    *git.WorktreeManager
	GitHub *github.PRClient
	Claude *ClaudeClient
}

// ClaudeClient is a placeholder interface for the Claude client
// This will be replaced when the claude package is implemented
type ClaudeClient interface{}

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
	return &Worker{
		unit:   unit,
		config: cfg,
		events: deps.Events,
		git:    deps.Git,
		github: deps.GitHub,
		claude: deps.Claude,
	}
}
