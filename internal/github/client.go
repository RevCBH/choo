package github

import (
	"github.com/anthropics/choo/internal/config"
	"github.com/anthropics/choo/internal/events"
)

// PRClient handles GitHub pull request operations
type PRClient struct {
	config *config.Config
	events *events.Bus
}

// NewPRClient creates a new GitHub PR client
func NewPRClient(cfg *config.Config, bus *events.Bus) *PRClient {
	return &PRClient{
		config: cfg,
		events: bus,
	}
}
