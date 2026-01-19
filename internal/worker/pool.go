package worker

import (
	"sync"

	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
)

// Pool manages a collection of workers executing units in parallel
type Pool struct {
	maxWorkers int
	config     WorkerConfig
	events     *events.Bus
	git        *git.WorktreeManager
	github     *github.PRClient
	claude     *ClaudeClient
	workers    map[string]*Worker
	mu         sync.Mutex
	wg         sync.WaitGroup
}

// PoolStats holds current pool statistics
type PoolStats struct {
	ActiveWorkers  int
	CompletedUnits int
	FailedUnits    int
	TotalTasks     int
	CompletedTasks int
}

// New creates a new worker pool (backward compatibility stub)
// This will be properly implemented in task #7
func New(size int, bus *events.Bus, git *git.WorktreeManager) *Pool {
	return &Pool{
		maxWorkers: size,
		events:     bus,
		git:        git,
		workers:    make(map[string]*Worker),
	}
}

// Stop shuts down the worker pool (backward compatibility stub)
// This will be properly implemented in task #7
func (p *Pool) Stop() error {
	return nil
}
