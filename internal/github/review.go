package github

import "time"

// ReviewStatus represents the current state of PR review
type ReviewStatus string

const (
	ReviewPending          ReviewStatus = "pending"
	ReviewInProgress       ReviewStatus = "in_progress"
	ReviewApproved         ReviewStatus = "approved"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewTimeout          ReviewStatus = "timeout"
)

// ReviewState holds the parsed review state from PR reactions
type ReviewState struct {
	Status       ReviewStatus
	HasEyes      bool
	HasThumbsUp  bool
	CommentCount int
	LastActivity time.Time
}

// PollResult represents the result of a single poll iteration
type PollResult struct {
	State       ReviewState
	Changed     bool
	ShouldMerge bool
	HasFeedback bool
	TimedOut    bool
}
