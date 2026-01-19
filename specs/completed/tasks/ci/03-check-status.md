---
task: 3
status: complete
backpressure: "go test ./internal/github/... -run Check"
depends_on: [2]
---

# Check Status API

**Parent spec**: `/specs/CI.md`
**Task**: #3 of 3 in implementation plan

## Objective

Implement the check status API methods: `GetCheckStatus`, `getCheckRuns`, and `WaitForChecks` on the `PRClient`.

## Dependencies

### Task Dependencies (within this unit)
- Task #2: Check types must be defined

### External Dependencies
- `PRClient` with `doRequest` method (already exists in `client.go`)

## Deliverables

### Files to Create/Modify
```
internal/github/
├── checks.go       # MODIFY: Add API methods
└── checks_test.go  # CREATE: Unit tests
```

### Implementation (append to checks.go)

```go
// Add these imports to the existing import block if not present
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// GetCheckStatus returns the aggregated status of all CI checks for a git ref.
// Returns CheckPending if any check is still running, CheckFailure if any check
// failed, or CheckSuccess if all checks completed successfully.
func (c *PRClient) GetCheckStatus(ctx context.Context, ref string) (CheckStatus, error) {
	runs, err := c.getCheckRuns(ctx, ref)
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

// getCheckRuns fetches all check runs for a git ref from the GitHub API.
func (c *PRClient) getCheckRuns(ctx context.Context, ref string) ([]CheckRun, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/check-runs", c.owner, c.repo, ref)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var response CheckRunsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return response.CheckRuns, nil
}

// WaitForChecks polls check status until all checks complete or context is cancelled.
func (c *PRClient) WaitForChecks(ctx context.Context, ref string, pollInterval time.Duration) (CheckStatus, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			status, err := c.GetCheckStatus(ctx, ref)
			if err != nil {
				return "", err
			}
			if status != CheckPending {
				return status, nil
			}
		}
	}
}
```

### Test File (checks_test.go)

```go
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

func newTestClientWithCheckRuns(t *testing.T, runs []CheckRun) *PRClient {
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
	}
}

func TestGetCheckStatus_AllSuccess(t *testing.T) {
	runs := []CheckRun{
		{Name: "test", Status: "completed", Conclusion: "success"},
		{Name: "lint", Status: "completed", Conclusion: "success"},
	}

	client := newTestClientWithCheckRuns(t, runs)
	// Override URL construction by using a custom implementation
	// For this test, we need to intercept the actual HTTP call

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

	client := newTestClientWithCheckRuns(t, runs)

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

	client := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatus(context.Background(), "abc123")
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

	client := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatus(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != CheckFailure {
		t.Errorf("expected CheckFailure, got %s", status)
	}
}

func TestGetCheckStatus_NoRuns(t *testing.T) {
	runs := []CheckRun{}

	client := newTestClientWithCheckRuns(t, runs)

	status, err := client.GetCheckStatus(context.Background(), "abc123")
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

	client := newTestClientWithCheckRuns(t, runs)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.WaitForChecks(ctx, "abc123", 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/github/... -run Check
```

### Success Criteria
- All `*Check*` tests pass
- `GetCheckStatus` returns correct status for all scenarios
- `WaitForChecks` respects context cancellation

## NOT In Scope
- Integration tests against real GitHub API
- Branch protection configuration
- Webhook-based status updates
- Configuration loading (`require_ci` setting)
