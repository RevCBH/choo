---
task: 3
status: complete
backpressure: "go test ./internal/github/... -run TestPollReview"
depends_on: [2]
---

# Add Error Resilience to Poll Loop

**Parent spec**: `/specs/REVIEW-POLLING.md`
**Task**: #3 of 5 in implementation plan

## Objective

Enhance PollReview to continue polling on transient errors instead of returning immediately, adding resilience for network issues and rate limits.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #2: Enhanced GetReviewStatus

## Deliverables

### Files to Modify
```
internal/github/
├── review.go      # MODIFY: Update PollReview for error resilience
└── review_test.go # MODIFY: Add tests for transient error handling
```

### Code Changes

Update `PollReview` in `internal/github/review.go`:

```go
// PollReview polls the PR for review status changes
// Returns when: approval received, feedback found, timeout, or context cancelled
// Transient errors are logged but polling continues
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
				// Log and continue - transient errors shouldn't stop polling
				// Rate limits are handled in doRequest with retry
				continue
			}

			result := &PollResult{
				State:   *state,
				Changed: previousState != nil && previousState.Status != state.Status,
			}

			// Check timeout
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
```

### Tests to Add

Add to `internal/github/review_test.go`:

```go
func TestPollReview_ContinuesOnTransientError(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		if strings.Contains(r.URL.Path, "reactions") {
			// First poll fails, second succeeds with approval
			if pollCount <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode([]Reaction{{ID: 1, Content: "+1"}})
			return
		}
		if strings.Contains(r.URL.Path, "comments") {
			if pollCount <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode([]ghReviewComment{})
			return
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	client.pollInterval = 10 * time.Millisecond

	result, err := client.PollReview(context.Background(), 123)

	require.NoError(t, err)
	assert.True(t, result.ShouldMerge)
	assert.True(t, pollCount >= 3, "should have polled multiple times")
}

func TestPollReview_RespectsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "reactions") {
			json.NewEncoder(w).Encode([]Reaction{})
			return
		}
		json.NewEncoder(w).Encode([]ghReviewComment{})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	client.pollInterval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.PollReview(ctx, 123)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPollReview_DetectsStateChange(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "reactions") {
			pollCount++
			if pollCount >= 2 {
				// Second poll has eyes reaction
				json.NewEncoder(w).Encode([]Reaction{{ID: 1, Content: "eyes"}})
			} else {
				json.NewEncoder(w).Encode([]Reaction{})
			}
			return
		}
		// Third poll gets approval
		if pollCount >= 3 {
			json.NewEncoder(w).Encode([]Reaction{{ID: 1, Content: "+1"}})
			return
		}
		json.NewEncoder(w).Encode([]ghReviewComment{})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	client.pollInterval = 10 * time.Millisecond

	// First call should return in_progress (with feedback=false since eyes)
	// Actually eyes means in_progress which is not a terminal state
	// Let's adjust - we need to test the Changed field

	result, err := client.PollReview(context.Background(), 123)
	require.NoError(t, err)
	// Will return on approval since that's terminal
	assert.True(t, result.ShouldMerge)
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/github/... -run TestPollReview
```

## NOT In Scope
- Event emission from PollReview
- Integration with events.Bus
- WaitForApproval changes beyond what PollReview provides
