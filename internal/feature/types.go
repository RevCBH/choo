package feature

import (
	"fmt"
	"time"
)

// PRD represents a Product Requirements Document
type PRD struct {
	// Required fields (from frontmatter)
	ID     string `yaml:"prd_id"`
	Title  string `yaml:"title"`
	Status string `yaml:"status"` // draft | approved | in_progress | complete | archived

	// Optional dependency hints
	DependsOn []string `yaml:"depends_on,omitempty"`

	// Complexity estimates
	EstimatedUnits int `yaml:"estimated_units,omitempty"`
	EstimatedTasks int `yaml:"estimated_tasks,omitempty"`

	// Orchestrator-managed fields (updated at runtime)
	FeatureBranch        string     `yaml:"feature_branch,omitempty"`
	FeatureStatus        string     `yaml:"feature_status,omitempty"`
	FeatureStartedAt     *time.Time `yaml:"feature_started_at,omitempty"`
	FeatureCompletedAt   *time.Time `yaml:"feature_completed_at,omitempty"`
	SpecReviewIterations int        `yaml:"spec_review_iterations,omitempty"`
	LastSpecReview       *time.Time `yaml:"last_spec_review,omitempty"`

	// File metadata (not in frontmatter)
	FilePath string `yaml:"-"`
	Body     string `yaml:"-"` // Markdown content after frontmatter
	BodyHash string `yaml:"-"` // SHA-256 for drift detection

	// Runtime tracking (not persisted)
	Units []Unit `yaml:"-"` // Units associated with this PRD
}

// Unit represents a work unit associated with a PRD
type Unit struct {
	Name   string
	Status string
}

// PRDStatus values for the status field
const (
	PRDStatusDraft      = "draft"
	PRDStatusApproved   = "approved"
	PRDStatusInProgress = "in_progress"
	PRDStatusComplete   = "complete"
	PRDStatusArchived   = "archived"
)

// FeatureStatus values for orchestrator-managed feature_status field
const (
	FeatureStatusPending         = "pending"
	FeatureStatusGeneratingSpecs = "generating_specs"
	FeatureStatusReviewingSpecs  = "reviewing_specs"
	FeatureStatusReviewBlocked   = "review_blocked"
	FeatureStatusValidatingSpecs = "validating_specs"
	FeatureStatusGeneratingTasks = "generating_tasks"
	FeatureStatusSpecsCommitted  = "specs_committed"
	FeatureStatusInProgress      = "in_progress"
	FeatureStatusUnitsComplete   = "units_complete"
	FeatureStatusPROpen          = "pr_open"
	FeatureStatusComplete        = "complete"
	FeatureStatusFailed          = "failed"
)

// ValidationError represents a frontmatter validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid PRD: %s: %s", e.Field, e.Message)
}

// validPRDStatuses returns the set of valid PRD status values
func validPRDStatuses() []string {
	return []string{
		PRDStatusDraft,
		PRDStatusApproved,
		PRDStatusInProgress,
		PRDStatusComplete,
		PRDStatusArchived,
	}
}

// validFeatureStatuses returns the set of valid feature status values
func validFeatureStatuses() []string {
	return []string{
		FeatureStatusPending,
		FeatureStatusGeneratingSpecs,
		FeatureStatusReviewingSpecs,
		FeatureStatusReviewBlocked,
		FeatureStatusValidatingSpecs,
		FeatureStatusGeneratingTasks,
		FeatureStatusSpecsCommitted,
		FeatureStatusInProgress,
		FeatureStatusUnitsComplete,
		FeatureStatusPROpen,
		FeatureStatusComplete,
		FeatureStatusFailed,
	}
}

// IsValidPRDStatus checks if a status string is a valid PRD status
func IsValidPRDStatus(s string) bool {
	for _, status := range validPRDStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidFeatureStatus checks if a status string is a valid feature status
func IsValidFeatureStatus(s string) bool {
	for _, status := range validFeatureStatuses() {
		if s == status {
			return true
		}
	}
	return false
}
