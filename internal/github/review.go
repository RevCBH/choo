package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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

// Reaction represents a GitHub reaction on a PR/issue
type Reaction struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// getReactions fetches reactions on a PR (issue endpoint)
func (c *PRClient) getReactions(ctx context.Context, prNumber int) ([]Reaction, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/reactions", c.owner, c.repo, prNumber)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reactions []Reaction
	if err := json.NewDecoder(resp.Body).Decode(&reactions); err != nil {
		return nil, fmt.Errorf("failed to decode reactions: %w", err)
	}

	return reactions, nil
}

// GetPRComments is a stub for Task #5
func (c *PRClient) GetPRComments(ctx context.Context, prNumber int) ([]PRComment, error) {
	return []PRComment{}, nil
}

// GetReviewStatus fetches the current review status from reactions
func (c *PRClient) GetReviewStatus(ctx context.Context, prNumber int) (*ReviewState, error) {
	reactions, err := c.getReactions(ctx, prNumber)
	if err != nil {
		return nil, err
	}

	comments, err := c.GetPRComments(ctx, prNumber)
	if err != nil {
		return nil, err
	}

	state := &ReviewState{
		CommentCount: len(comments),
		LastActivity: time.Now(),
	}

	for _, reaction := range reactions {
		if reaction.Content == "+1" {
			state.HasThumbsUp = true
		}
		if reaction.Content == "eyes" {
			state.HasEyes = true
		}
	}

	// Determine status with precedence: approved > in_progress > changes_requested > pending
	if state.HasThumbsUp {
		state.Status = ReviewApproved
	} else if state.HasEyes {
		state.Status = ReviewInProgress
	} else if state.CommentCount > 0 {
		state.Status = ReviewChangesRequested
	} else {
		state.Status = ReviewPending
	}

	return state, nil
}

// PollReview polls the PR for review status changes
// Returns when approval received, feedback found, or timeout
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	startTime := time.Now()
	var previousState *ReviewState

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			state, err := c.GetReviewStatus(ctx, prNumber)
			if err != nil {
				return nil, err
			}

			result := &PollResult{
				State:   *state,
				Changed: previousState != nil && previousState.Status != state.Status,
			}

			// Check timeout condition
			if time.Since(startTime) >= c.reviewTimeout {
				result.TimedOut = true
				return result, nil
			}

			// Check terminal conditions
			if state.Status == ReviewApproved {
				result.ShouldMerge = true
				return result, nil
			}

			if state.Status == ReviewChangesRequested {
				result.HasFeedback = true
				return result, nil
			}

			previousState = state
		}
	}
}

// WaitForApproval blocks until PR is approved or timeout
// Emits events on state changes
func (c *PRClient) WaitForApproval(ctx context.Context, prNumber int, prStartTime time.Time) error {
	result, err := c.PollReview(ctx, prNumber)
	if err != nil {
		return err
	}

	if result.TimedOut {
		return fmt.Errorf("review timed out after %v", c.reviewTimeout)
	}

	if result.HasFeedback {
		return fmt.Errorf("changes requested")
	}

	if !result.ShouldMerge {
		return fmt.Errorf("unexpected poll result: not approved")
	}

	return nil
}
