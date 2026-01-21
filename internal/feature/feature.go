package feature

import (
	"time"
)

// LifecycleStatus represents the high-level lifecycle state of a feature
type LifecycleStatus string

const (
	LifecyclePending    LifecycleStatus = "pending"     // Feature created, no work started
	LifecycleInProgress LifecycleStatus = "in_progress" // Specs being implemented
	LifecycleComplete   LifecycleStatus = "complete"    // All specs merged to feature branch
	LifecycleMerged     LifecycleStatus = "merged"      // Feature branch merged to main
)

// Feature represents a feature being developed from a PRD
type Feature struct {
	PRD       *PRD            // Reference to the source PRD
	Branch    string          // Feature branch name (e.g., "feature/streaming-events")
	Status    LifecycleStatus // Current lifecycle state
	StartedAt time.Time       // When feature work began
}

// NewFeature creates a Feature from a PRD
// Sets Branch to "feature/<prd.ID>", Status to LifecyclePending, StartedAt to time.Now()
func NewFeature(p *PRD) *Feature {
	return &Feature{
		PRD:       p,
		Branch:    "feature/" + p.ID,
		Status:    LifecyclePending,
		StartedAt: time.Now(),
	}
}

// GetBranch returns the feature branch name
func (f *Feature) GetBranch() string {
	return f.Branch
}

// SetStatus updates the feature status
func (f *Feature) SetStatus(status LifecycleStatus) {
	f.Status = status
}

// IsComplete returns true if all specs have been merged to the feature branch
func (f *Feature) IsComplete() bool {
	return f.Status == LifecycleComplete
}

// IsMerged returns true if the feature branch has been merged to main
func (f *Feature) IsMerged() bool {
	return f.Status == LifecycleMerged
}
