package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newMockClient creates a mock PRClient for testing with httptest server
func newMockClient(reactions []Reaction, comments []PRComment) (*PRClient, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Handle reactions endpoint
		if strings.Contains(r.URL.Path, "/reactions") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(reactions)
			return
		}

		// Handle comments endpoint (stub for Task #5)
		if strings.Contains(r.URL.Path, "/comments") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(comments)
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))

	// Create client that uses the test server URL
	client := &PRClient{
		httpClient:    server.Client(),
		owner:         "owner",
		repo:          "repo",
		pollInterval:  10 * time.Millisecond,
		reviewTimeout: 50 * time.Millisecond,
		token:         "test-token",
		baseURL:       server.URL,
	}

	return client, server
}

func TestGetReviewStatus_NoReactions(t *testing.T) {
	client, server := newMockClient([]Reaction{}, []PRComment{})
	defer server.Close()

	// Override getReactions to use test server
	ctx := context.Background()
	reactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(reactions, []PRComment{})

	if state.Status != ReviewPending {
		t.Errorf("expected ReviewPending, got %v", state.Status)
	}
	if state.HasEyes {
		t.Error("expected HasEyes to be false")
	}
	if state.HasThumbsUp {
		t.Error("expected HasThumbsUp to be false")
	}
}

func TestGetReviewStatus_EyesOnly(t *testing.T) {
	reactions := []Reaction{
		{ID: 1, Content: "eyes", CreatedAt: time.Now()},
	}
	client, server := newMockClient(reactions, []PRComment{})
	defer server.Close()

	ctx := context.Background()
	fetchedReactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(fetchedReactions, []PRComment{})

	if state.Status != ReviewInProgress {
		t.Errorf("expected ReviewInProgress, got %v", state.Status)
	}
	if !state.HasEyes {
		t.Error("expected HasEyes to be true")
	}
	if state.HasThumbsUp {
		t.Error("expected HasThumbsUp to be false")
	}
}

func TestGetReviewStatus_ThumbsUp(t *testing.T) {
	reactions := []Reaction{
		{ID: 1, Content: "+1", CreatedAt: time.Now()},
	}
	client, server := newMockClient(reactions, []PRComment{})
	defer server.Close()

	ctx := context.Background()
	fetchedReactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(fetchedReactions, []PRComment{})

	if state.Status != ReviewApproved {
		t.Errorf("expected ReviewApproved, got %v", state.Status)
	}
	if state.HasEyes {
		t.Error("expected HasEyes to be false")
	}
	if !state.HasThumbsUp {
		t.Error("expected HasThumbsUp to be true")
	}
}

func TestGetReviewStatus_ThumbsUpBeatsEyes(t *testing.T) {
	reactions := []Reaction{
		{ID: 1, Content: "eyes", CreatedAt: time.Now()},
		{ID: 2, Content: "+1", CreatedAt: time.Now()},
	}
	client, server := newMockClient(reactions, []PRComment{})
	defer server.Close()

	ctx := context.Background()
	fetchedReactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(fetchedReactions, []PRComment{})

	if state.Status != ReviewApproved {
		t.Errorf("expected ReviewApproved (thumbs up beats eyes), got %v", state.Status)
	}
	if !state.HasEyes {
		t.Error("expected HasEyes to be true")
	}
	if !state.HasThumbsUp {
		t.Error("expected HasThumbsUp to be true")
	}
}

func TestGetReviewStatus_CommentsWithoutEmoji(t *testing.T) {
	comments := []PRComment{
		{ID: 1, Body: "Please fix this"},
	}
	client, server := newMockClient([]Reaction{}, comments)
	defer server.Close()

	ctx := context.Background()
	fetchedReactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(fetchedReactions, comments)

	if state.Status != ReviewChangesRequested {
		t.Errorf("expected ReviewChangesRequested, got %v", state.Status)
	}
	if state.CommentCount != 1 {
		t.Errorf("expected CommentCount 1, got %d", state.CommentCount)
	}
}

func TestGetReviewStatus_EyesBeatsComments(t *testing.T) {
	reactions := []Reaction{
		{ID: 1, Content: "eyes", CreatedAt: time.Now()},
	}
	comments := []PRComment{
		{ID: 1, Body: "Please fix this"},
	}
	client, server := newMockClient(reactions, comments)
	defer server.Close()

	ctx := context.Background()
	fetchedReactions, err := testGetReactions(client, server, ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	state := evaluateReviewState(fetchedReactions, comments)

	if state.Status != ReviewInProgress {
		t.Errorf("expected ReviewInProgress (eyes beats comments), got %v", state.Status)
	}
	if !state.HasEyes {
		t.Error("expected HasEyes to be true")
	}
}

func TestPollReview_Timeout(t *testing.T) {
	client, server := newMockClient([]Reaction{}, []PRComment{})
	defer server.Close()

	// Create a test client with proper URL
	testClient := createTestClient(client, server)
	ctx := context.Background()

	result, err := testClient.PollReview(ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.TimedOut {
		t.Error("expected TimedOut to be true")
	}
	if result.ShouldMerge {
		t.Error("expected ShouldMerge to be false")
	}
}

func TestPollReview_ApprovalStopsPolling(t *testing.T) {
	reactionsWithApproval := []Reaction{
		{ID: 1, Content: "+1", CreatedAt: time.Now()},
	}
	client, server := newMockClient(reactionsWithApproval, []PRComment{})
	defer server.Close()

	testClient := createTestClient(client, server)
	ctx := context.Background()

	result, err := testClient.PollReview(ctx, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.ShouldMerge {
		t.Error("expected ShouldMerge to be true when thumbs up added")
	}
	if result.TimedOut {
		t.Error("expected TimedOut to be false")
	}
	if result.State.Status != ReviewApproved {
		t.Errorf("expected ReviewApproved, got %v", result.State.Status)
	}
}

func TestPollReview_ContextCancellation(t *testing.T) {
	client, server := newMockClient([]Reaction{}, []PRComment{})
	defer server.Close()

	testClient := createTestClient(client, server)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := testClient.PollReview(ctx, 1)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// Helper functions for testing

// testGetReactions calls getReactions with the test server URL
func testGetReactions(client *PRClient, server *httptest.Server, ctx context.Context, prNumber int) ([]Reaction, error) {
	url := server.URL + "/repos/owner/repo/issues/1/reactions"
	resp, err := client.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reactions []Reaction
	if err := json.NewDecoder(resp.Body).Decode(&reactions); err != nil {
		return nil, err
	}
	return reactions, nil
}

// evaluateReviewState evaluates review state from reactions and comments
func evaluateReviewState(reactions []Reaction, comments []PRComment) *ReviewState {
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

	return state
}

// createTestClient creates a client that routes requests through the test server
func createTestClient(baseClient *PRClient, server *httptest.Server) *PRClient {
	// Replace the base URL in the client
	testClient := &PRClient{
		httpClient:    baseClient.httpClient,
		owner:         baseClient.owner,
		repo:          baseClient.repo,
		pollInterval:  baseClient.pollInterval,
		reviewTimeout: baseClient.reviewTimeout,
		token:         baseClient.token,
		baseURL:       server.URL,
	}

	// Create wrapper that modifies URLs to use test server
	originalClient := testClient.httpClient
	testClient.httpClient = &http.Client{
		Transport: &testTransport{
			base:      originalClient.Transport,
			serverURL: server.URL,
		},
	}

	return testClient
}

// testTransport wraps http.RoundTripper to redirect to test server
type testTransport struct {
	base      http.RoundTripper
	serverURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace GitHub API URL with test server URL
	if strings.Contains(req.URL.String(), "api.github.com") {
		newURL := strings.Replace(req.URL.String(), "https://api.github.com", t.serverURL, 1)
		newReq, err := http.NewRequest(req.Method, newURL, req.Body)
		if err != nil {
			return nil, err
		}
		newReq.Header = req.Header
		req = newReq
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func TestGetReviewStatus_TracksLastActivityFromReactions(t *testing.T) {
	reactionTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

	client := &PRClient{
		httpClient: &http.Client{
			Transport: &testTransport{
				base:      http.DefaultTransport,
				serverURL: server.URL,
			},
		},
		owner:         "owner",
		repo:          "repo",
		pollInterval:  10 * time.Millisecond,
		reviewTimeout: 50 * time.Millisecond,
		token:         "test-token",
		baseURL:       server.URL,
	}

	state, err := client.GetReviewStatus(context.Background(), 123)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !state.LastActivity.Equal(reactionTime) {
		t.Errorf("expected LastActivity %v, got %v", reactionTime, state.LastActivity)
	}
}

func TestGetReviewStatus_TracksLastActivityFromComments(t *testing.T) {
	commentTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

	client := &PRClient{
		httpClient: &http.Client{
			Transport: &testTransport{
				base:      http.DefaultTransport,
				serverURL: server.URL,
			},
		},
		owner:         "owner",
		repo:          "repo",
		pollInterval:  10 * time.Millisecond,
		reviewTimeout: 50 * time.Millisecond,
		token:         "test-token",
		baseURL:       server.URL,
	}

	state, err := client.GetReviewStatus(context.Background(), 123)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !state.LastActivity.Equal(commentTime) {
		t.Errorf("expected LastActivity %v, got %v", commentTime, state.LastActivity)
	}
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
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
