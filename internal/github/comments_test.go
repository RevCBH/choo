package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newMockCommentsClient creates a mock PRClient for testing comment operations
func newMockCommentsClient(comments []ghReviewComment) (*PRClient, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Handle comments endpoint
		if strings.Contains(r.URL.Path, "/pulls/") && strings.HasSuffix(r.URL.Path, "/comments") {
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
	}

	// Use test transport to redirect API calls to test server
	client.httpClient = &http.Client{
		Transport: &testTransportComments{
			serverURL: server.URL,
		},
	}

	return client, server
}

// testTransportComments wraps http.RoundTripper to redirect to test server
type testTransportComments struct {
	serverURL string
}

func (t *testTransportComments) RoundTrip(req *http.Request) (*http.Response, error) {
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

	return http.DefaultTransport.RoundTrip(req)
}

func TestGetPRComments_Empty(t *testing.T) {
	client, server := newMockCommentsClient([]ghReviewComment{})
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetPRComments(ctx, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 0 {
		t.Errorf("expected empty slice, got %d comments", len(comments))
	}
}

func TestGetPRComments_Multiple(t *testing.T) {
	line1 := 42
	line2 := 15
	createdAt1 := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	createdAt2 := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	mockComments := []ghReviewComment{
		{
			ID:        1234,
			Path:      "internal/github/client.go",
			Line:      &line1,
			Body:      "Consider adding error handling here",
			User:      ghUser{Login: "reviewer-bot"},
			CreatedAt: createdAt1,
		},
		{
			ID:        1235,
			Path:      "internal/github/pr.go",
			Line:      &line2,
			Body:      "Nice work!",
			User:      ghUser{Login: "human-reviewer"},
			CreatedAt: createdAt2,
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetPRComments(ctx, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// Check first comment
	if comments[0].ID != 1234 {
		t.Errorf("expected ID 1234, got %d", comments[0].ID)
	}
	if comments[0].Path != "internal/github/client.go" {
		t.Errorf("expected path internal/github/client.go, got %s", comments[0].Path)
	}
	if comments[0].Line != 42 {
		t.Errorf("expected line 42, got %d", comments[0].Line)
	}
	if comments[0].Body != "Consider adding error handling here" {
		t.Errorf("expected body 'Consider adding error handling here', got %s", comments[0].Body)
	}
	if comments[0].Author != "reviewer-bot" {
		t.Errorf("expected author reviewer-bot, got %s", comments[0].Author)
	}
	if !comments[0].CreatedAt.Equal(createdAt1) {
		t.Errorf("expected CreatedAt %v, got %v", createdAt1, comments[0].CreatedAt)
	}

	// Check second comment
	if comments[1].ID != 1235 {
		t.Errorf("expected ID 1235, got %d", comments[1].ID)
	}
}

func TestGetPRComments_ParsesPath(t *testing.T) {
	line := 10
	mockComments := []ghReviewComment{
		{
			ID:        100,
			Path:      "cmd/main.go",
			Line:      &line,
			Body:      "test",
			User:      ghUser{Login: "tester"},
			CreatedAt: time.Now(),
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetPRComments(ctx, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	if comments[0].Path != "cmd/main.go" {
		t.Errorf("expected path cmd/main.go, got %s", comments[0].Path)
	}
}

func TestGetPRComments_ParsesLine(t *testing.T) {
	line := 99
	mockComments := []ghReviewComment{
		{
			ID:        200,
			Path:      "test.go",
			Line:      &line,
			Body:      "test",
			User:      ghUser{Login: "tester"},
			CreatedAt: time.Now(),
		},
		{
			ID:        201,
			Path:      "test2.go",
			Line:      nil, // null line for general comment
			Body:      "general comment",
			User:      ghUser{Login: "tester"},
			CreatedAt: time.Now(),
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetPRComments(ctx, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	if comments[0].Line != 99 {
		t.Errorf("expected line 99, got %d", comments[0].Line)
	}

	// Comment with null line should have line 0
	if comments[1].Line != 0 {
		t.Errorf("expected line 0 for null line, got %d", comments[1].Line)
	}
}

func TestGetPRComments_ParsesAuthor(t *testing.T) {
	line := 10
	mockComments := []ghReviewComment{
		{
			ID:        300,
			Path:      "test.go",
			Line:      &line,
			Body:      "test",
			User:      ghUser{Login: "octocat"},
			CreatedAt: time.Now(),
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetPRComments(ctx, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	if comments[0].Author != "octocat" {
		t.Errorf("expected author octocat, got %s", comments[0].Author)
	}
}

func TestGetUnaddressedComments_FiltersBySince(t *testing.T) {
	line := 10
	oldTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	sinceTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	mockComments := []ghReviewComment{
		{
			ID:        1,
			Path:      "old.go",
			Line:      &line,
			Body:      "old comment",
			User:      ghUser{Login: "tester"},
			CreatedAt: oldTime,
		},
		{
			ID:        2,
			Path:      "new.go",
			Line:      &line,
			Body:      "new comment",
			User:      ghUser{Login: "tester"},
			CreatedAt: newTime,
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetUnaddressedComments(ctx, 1, sinceTime)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 unaddressed comment, got %d", len(comments))
	}

	if comments[0].ID != 2 {
		t.Errorf("expected comment ID 2, got %d", comments[0].ID)
	}

	if comments[0].Body != "new comment" {
		t.Errorf("expected 'new comment', got %s", comments[0].Body)
	}
}

func TestGetUnaddressedComments_AllNew(t *testing.T) {
	line := 10
	newTime1 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	newTime2 := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)
	sinceTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	mockComments := []ghReviewComment{
		{
			ID:        1,
			Path:      "test1.go",
			Line:      &line,
			Body:      "comment 1",
			User:      ghUser{Login: "tester"},
			CreatedAt: newTime1,
		},
		{
			ID:        2,
			Path:      "test2.go",
			Line:      &line,
			Body:      "comment 2",
			User:      ghUser{Login: "tester"},
			CreatedAt: newTime2,
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetUnaddressedComments(ctx, 1, sinceTime)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 unaddressed comments, got %d", len(comments))
	}
}

func TestGetUnaddressedComments_NoneNew(t *testing.T) {
	line := 10
	oldTime1 := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	oldTime2 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sinceTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	mockComments := []ghReviewComment{
		{
			ID:        1,
			Path:      "test1.go",
			Line:      &line,
			Body:      "old comment 1",
			User:      ghUser{Login: "tester"},
			CreatedAt: oldTime1,
		},
		{
			ID:        2,
			Path:      "test2.go",
			Line:      &line,
			Body:      "old comment 2",
			User:      ghUser{Login: "tester"},
			CreatedAt: oldTime2,
		},
	}

	client, server := newMockCommentsClient(mockComments)
	defer server.Close()

	ctx := context.Background()
	comments, err := client.GetUnaddressedComments(ctx, 1, sinceTime)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(comments) != 0 {
		t.Fatalf("expected 0 unaddressed comments, got %d", len(comments))
	}
}
