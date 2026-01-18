package scheduler

import (
	"github.com/anthropics/choo/internal/config"
	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
)

// Scheduler coordinates task execution across units
type Scheduler struct {
	config    *config.Config
	events    *events.Bus
	discovery *discovery.Discovery
}

// New creates a new scheduler
func New(cfg *config.Config, bus *events.Bus, disc *discovery.Discovery) *Scheduler {
	return &Scheduler{
		config:    cfg,
		events:    bus,
		discovery: disc,
	}
}
