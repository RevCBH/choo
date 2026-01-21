package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCodeReview_NilReviewer(t *testing.T) {
	w := &Worker{
		reviewer: nil, // Disabled
		events:   events.NewBus(100),
	}

	// Should return immediately without error or panic
	ctx := context.Background()
	w.runCodeReview(ctx)
	// No assertions needed - just verify no panic
}

func TestRunCodeReview_ReviewerError(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{err: errors.New("reviewer unavailable")},
		events:   eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled: true,
			Verbose: true,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	// Should emit started, then failed
	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewFailed, (*collected)[1].Type)
}

func TestRunCodeReview_Passed(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed:  true,
				Summary: "All checks passed",
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled: true,
			Verbose: true,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewPassed, (*collected)[1].Type)
}

func TestRunCodeReview_IssuesFound(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed: false,
				Issues: []provider.ReviewIssue{
					{File: "test.go", Line: 1, Message: "Test issue"},
				},
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, (*collected)[1].Type)
	// Note: we can't verify fix loop was called without refactoring,
	// but the config check ensures it would be called if implemented
}

func TestRunCodeReview_IssuesFound_ZeroIterations(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed: false,
				Issues: []provider.ReviewIssue{
					{File: "test.go", Line: 1, Message: "Test issue"},
				},
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 0, // Review-only mode
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, (*collected)[1].Type)
	// The logic ensures fix loop is not called when MaxFixIterations=0
}

// collectEvents subscribes to the event bus and collects events for testing.
// Returns a pointer to a slice that will be populated with events as they occur.
func collectEvents(bus *events.Bus) *[]events.Event {
	collected := &[]events.Event{}
	var mu sync.Mutex
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		*collected = append(*collected, e)
		mu.Unlock()
	})
	return collected
}

// waitForEvents waits for the event bus to process all pending events
func waitForEvents(bus *events.Bus) {
	// Wait a small amount of time for events to be processed by the goroutine
	time.Sleep(10 * time.Millisecond)
}

// mockReviewer for testing
type mockReviewer struct {
	result *provider.ReviewResult
	err    error
}

func (m *mockReviewer) Review(ctx context.Context, workdir, baseBranch string) (*provider.ReviewResult, error) {
	return m.result, m.err
}

func (m *mockReviewer) Name() provider.ProviderType {
	return "mock"
}
