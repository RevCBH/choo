---
task: 2
status: pending
backpressure: "go test ./internal/github/... -run TestGetReviewStatus"
depends_on: [1]
---

# Enhance Status Detection with LastActivity

**Parent spec**: `/specs/REVIEW-POLLING.md`
**Task**: #2 of 5 in implementation plan

## Objective

Enhance GetReviewStatus to properly track LastActivity from reactions and comments, rather than using time.Now().

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1: ReviewPollerConfig type must exist

## Deliverables

### Files to Modify
```
internal/github/
├── review.go      # MODIFY: Update GetReviewStatus
└── review_test.go # MODIFY: Add tests for LastActivity tracking
```

### Code Changes

Update `GetReviewStatus` in `internal/github/review.go` to track LastActivity properly:

```go
// GetReviewStatus fetches the current review status from reactions and comments
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
	}

	// Track last activity from reactions
	for _, reaction := range reactions {
		if reaction.Content == "+1" {
			state.HasThumbsUp = true
		}
		if reaction.Content == "eyes" {
			state.HasEyes = true
		}
		if reaction.CreatedAt.After(state.LastActivity) {
			state.LastActivity = reaction.CreatedAt
		}
	}

	// Track last activity from comments
	for _, comment := range comments {
		if comment.CreatedAt.After(state.LastActivity) {
			state.LastActivity = comment.CreatedAt
		}
	}

	// If no activity found, use current time
	if state.LastActivity.IsZero() {
		state.LastActivity = time.Now()
	}

	// Determine status with precedence: approved > in_progress > changes_requested > pending
	state.Status = determineStatus(state.HasThumbsUp, state.HasEyes, state.CommentCount)

	return state, nil
}

// determineStatus applies status precedence rules
func determineStatus(hasThumbsUp, hasEyes bool, commentCount int) ReviewStatus {
	if hasThumbsUp {
		return ReviewApproved
	}
	if hasEyes {
		return ReviewInProgress
	}
	if commentCount > 0 {
		return ReviewChangesRequested
	}
	return ReviewPending
}
```

### Tests to Add

Add to `internal/github/review_test.go`:

```go
func TestGetReviewStatus_TracksLastActivityFromReactions(t *testing.T) {
	reactionTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "reactions") {
			json.NewEncoder(w).Encode([]Reaction{
				{ID: 1, Content: "eyes", CreatedAt: reactionTime},
			})
			return
		}
		if strings.Contains(r.URL.Path, "comments") {
			json.NewEncoder(w).Encode([]ghReviewComment{})
			return
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	state, err := client.GetReviewStatus(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, reactionTime, state.LastActivity)
}

func TestGetReviewStatus_TracksLastActivityFromComments(t *testing.T) {
	commentTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "reactions") {
			json.NewEncoder(w).Encode([]Reaction{})
			return
		}
		if strings.Contains(r.URL.Path, "comments") {
			json.NewEncoder(w).Encode([]ghReviewComment{
				{ID: 1, Body: "Fix this", User: ghUser{Login: "reviewer"}, CreatedAt: commentTime},
			})
			return
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	state, err := client.GetReviewStatus(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, commentTime, state.LastActivity)
}

func TestDetermineStatus_Precedence(t *testing.T) {
	tests := []struct {
		name         string
		hasThumbsUp  bool
		hasEyes      bool
		commentCount int
		expected     ReviewStatus
	}{
		{"approved takes precedence", true, true, 5, ReviewApproved},
		{"eyes without approval", false, true, 3, ReviewInProgress},
		{"comments without reactions", false, false, 2, ReviewChangesRequested},
		{"no activity is pending", false, false, 0, ReviewPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatus(tt.hasThumbsUp, tt.hasEyes, tt.commentCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/github/... -run TestGetReviewStatus
```

## NOT In Scope
- Changes to PollReview or WaitForApproval
- Event emission
- Error resilience (handled in task #3)
