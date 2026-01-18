package worker

import (
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
)

// Pool manages a pool of worker goroutines
type Pool struct {
	events *events.Bus
	git    *git.WorktreeManager
	size   int
}

// New creates a new worker pool
func New(size int, bus *events.Bus, git *git.WorktreeManager) *Pool {
	return &Pool{
		events: bus,
		git:    git,
		size:   size,
	}
}

// Stop shuts down the worker pool
func (p *Pool) Stop() error {
	return nil
}
