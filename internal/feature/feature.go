package feature

import (
	"time"
)

// Status represents the current state of a feature
type Status string

const (
	StatusPending    Status = "pending"     // Feature created, no work started
	StatusInProgress Status = "in_progress" // Specs being implemented
	StatusComplete   Status = "complete"    // All specs merged to feature branch
	StatusMerged     Status = "merged"      // Feature branch merged to main
)

// Feature represents a feature being developed from a PRD
type Feature struct {
	PRD       *PRD      // Reference to the source PRD
	Branch    string    // Feature branch name (e.g., "feature/streaming-events")
	Status    Status    // Current lifecycle state
	StartedAt time.Time // When feature work began
}

// NewFeature creates a Feature from a PRD
// Sets Branch to "feature/<prd.ID>", Status to StatusPending, StartedAt to time.Now()
func NewFeature(p *PRD) *Feature {
	return &Feature{
		PRD:       p,
		Branch:    "feature/" + p.ID,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}
}

// GetBranch returns the feature branch name
func (f *Feature) GetBranch() string {
	return f.Branch
}

// SetStatus updates the feature status
func (f *Feature) SetStatus(status Status) {
	f.Status = status
}

// IsComplete returns true if all specs have been merged to the feature branch
func (f *Feature) IsComplete() bool {
	return f.Status == StatusComplete
}

// IsMerged returns true if the feature branch has been merged to main
func (f *Feature) IsMerged() bool {
	return f.Status == StatusMerged
}
