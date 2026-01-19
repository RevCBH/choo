package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient creates a PRClient configured to use the test server
func newTestClient(t *testing.T, handler http.HandlerFunc) *PRClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &PRClient{
		httpClient: server.Client(),
		owner:      "test-owner",
		repo:       "test-repo",
		token:      "test-token",
		baseURL:    server.URL,
	}
}

// checkRunsHandler returns an http.HandlerFunc that serves the given check runs
func checkRunsHandler(runs []CheckRun) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := CheckRunsResponse{
			TotalCount: len(runs),
			CheckRuns:  runs,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func TestGetCheckStatus_AllSuccess(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "lint", Status: "completed", Conclusion: "success"},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
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

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
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

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure, got %s", status)
	}
}

func TestGetCheckStatus_FailureTakesPrecedence(t *testing.T) {
	// Failure takes precedence over pending - if any check has failed,
	// report failure immediately even if other checks are still running.
	// This enables fail-fast behavior in CI.
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "failure"},
		{Name: "lint", Status: "in_progress", Conclusion: ""},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure (failure takes precedence), got %s", status)
	}
}

func TestGetCheckStatus_NoRuns(t *testing.T) {
	runs := []CheckRun{}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Zero check runs should return pending, not success
	if status != CheckPending {
		t.Errorf("expected CheckPending for no runs (checks haven't started), got %s", status)
	}
}

func TestGetCheckStatus_Cancelled(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "cancelled"},
		{Name: "lint", Status: "completed", Conclusion: "success"},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure for cancelled check, got %s", status)
	}
}

func TestGetCheckStatus_TimedOut(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "timed_out"},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure for timed_out check, got %s", status)
	}
}

func TestGetCheckStatus_Skipped(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "optional", Status: "completed", Conclusion: "skipped"},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess (skipped is acceptable), got %s", status)
	}
}

func TestGetCheckStatus_Neutral(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "info", Status: "completed", Conclusion: "neutral"},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess (neutral is acceptable), got %s", status)
	}
}

func TestWaitForChecks_Timeout(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "in_progress", Conclusion: ""},
	}

	client := newTestClient(t, checkRunsHandler(runs))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.WaitForChecks(ctx, "abc123", 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}

func TestWaitForChecks_EventualSuccess(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var runs []CheckRun
		if callCount < 3 {
			runs = []CheckRun{
				{Name: "test", Status: "in_progress", Conclusion: ""},
			}
		} else {
			runs = []CheckRun{
				{Name: "test", Status: "completed", Conclusion: "success"},
			}
		}
		response := CheckRunsResponse{
			TotalCount: len(runs),
			CheckRuns:  runs,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	client := newTestClient(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	status, err := client.WaitForChecks(ctx, "abc123", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess, got %s", status)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}

func TestGetCheckRuns_Pagination(t *testing.T) {
	page1Runs := make([]CheckRun, 100)
	for i := range page1Runs {
		page1Runs[i] = CheckRun{ID: int64(i), Name: "test", Status: "completed", Conclusion: "success"}
	}
	page2Runs := []CheckRun{
		{ID: 100, Name: "test", Status: "completed", Conclusion: "success"},
		{ID: 101, Name: "test", Status: "completed", Conclusion: "success"},
	}

	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var response CheckRunsResponse
		if callCount == 1 {
			response = CheckRunsResponse{
				TotalCount: 102,
				CheckRuns:  page1Runs,
			}
		} else {
			response = CheckRunsResponse{
				TotalCount: 102,
				CheckRuns:  page2Runs,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	client := newTestClient(t, handler)

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckSuccess {
		t.Errorf("expected CheckSuccess, got %s", status)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}
