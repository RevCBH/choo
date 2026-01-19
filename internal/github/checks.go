// internal/github/checks.go
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// CheckStatus represents the aggregated status of CI checks for a commit
type CheckStatus string

const (
	// CheckPending indicates one or more checks are still running
	CheckPending CheckStatus = "pending"
	// CheckSuccess indicates all checks completed successfully
	CheckSuccess CheckStatus = "success"
	// CheckFailure indicates one or more checks failed
	CheckFailure CheckStatus = "failure"
)

// CheckRun represents a single GitHub Actions check run
type CheckRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`     // queued, in_progress, completed
	Conclusion string `json:"conclusion"` // success, failure, cancelled, skipped
}

// CheckRunsResponse represents the GitHub API response for check runs
type CheckRunsResponse struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []CheckRun `json:"check_runs"`
}

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
