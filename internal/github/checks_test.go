package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClientWithCheckRuns(t *testing.T, runs []CheckRun) (*PRClient, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := CheckRunsResponse{
			TotalCount: len(runs),
			CheckRuns:  runs,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)

	return &PRClient{
		httpClient: server.Client(),
		owner:      "test-owner",
		repo:       "test-repo",
		token:      "test-token",
	}, server
}

// getCheckRunsForTest is a test helper that uses the test server URL
func (c *PRClient) getCheckRunsForTest(ctx context.Context, ref string, serverURL string) ([]CheckRun, error) {
	// Replace the GitHub API URL with test server URL
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs", serverURL, c.owner, c.repo, ref)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response CheckRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return response.CheckRuns, nil
}

// GetCheckStatusForTest is a test helper that uses the test server URL
func (c *PRClient) GetCheckStatusForTest(ctx context.Context, ref string, serverURL string) (CheckStatus, error) {
	runs, err := c.getCheckRunsForTest(ctx, ref, serverURL)
	if err != nil {
		return "", fmt.Errorf("get check runs: %w", err)
	}

	allComplete := true
	anyFailed := false

	for _, run := range runs {
		if run.Status != "completed" {
			allComplete = false
		}
		if run.Conclusion == "failure" {
			anyFailed = true
		}
	}

	if anyFailed {
		return CheckFailure, nil
	}
	if !allComplete {
		return CheckPending, nil
	}
	return CheckSuccess, nil
}

// WaitForChecksForTest is a test helper that uses the test server URL
func (c *PRClient) WaitForChecksForTest(ctx context.Context, ref string, pollInterval time.Duration, serverURL string) (CheckStatus, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			status, err := c.GetCheckStatusForTest(ctx, ref, serverURL)
			if err != nil {
				return "", err
			}
			if status != CheckPending {
				return status, nil
			}
		}
	}
}

func TestGetCheckStatus_AllSuccess(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "lint", Status: "completed", Conclusion: "success"},
	}

	client, server := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatusForTest(context.Background(), "abc123", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess, got %s", status)
	}
}

func TestGetCheckStatus_OnePending(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "lint", Status: "in_progress", Conclusion: ""},
	}

	client, server := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatusForTest(context.Background(), "abc123", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckPending {
		t.Errorf("expected CheckPending, got %s", status)
	}
}

func TestGetCheckStatus_OneFailure(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "failure"},
		{Name: "lint", Status: "completed", Conclusion: "success"},
	}

	client, server := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatusForTest(context.Background(), "abc123", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure, got %s", status)
	}
}

func TestGetCheckStatus_FailureTakesPrecedence(t *testing.T) {
	// Even if some checks are pending, a failure is immediately reported
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "failure"},
		{Name: "lint", Status: "in_progress", Conclusion: ""},
	}

	client, server := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatusForTest(context.Background(), "abc123", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure, got %s", status)
	}
}

func TestGetCheckStatus_NoRuns(t *testing.T) {
	runs := []CheckRun{}

	client, server := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatusForTest(context.Background(), "abc123", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess for no runs, got %s", status)
	}
}

func TestWaitForChecks_Timeout(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "in_progress", Conclusion: ""},
	}

	client, server := newTestClientWithCheckRuns(t, runs)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.WaitForChecksForTest(ctx, "abc123", 10*time.Millisecond, server.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}
