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
// Returns CheckPending if any check is still running or if there are no checks,
// CheckFailure if any check failed, or CheckSuccess if all checks completed successfully.
// Priority: failure > pending > success (failure is reported immediately even if checks are pending).
func (c *PRClient) GetCheckStatus(ctx context.Context, ref string) (CheckStatus, error) {
	runs, err := c.getCheckRuns(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("get check runs: %w", err)
	}

	// No check runs means checks haven't started yet
	if len(runs) == 0 {
		return CheckPending, nil
	}

	allComplete := true
	anyFailed := false

	for _, run := range runs {
		if run.Status != "completed" {
			allComplete = false
			continue
		}
		// Treat any conclusion other than success, skipped, or neutral as failure
		// This includes: failure, cancelled, timed_out, action_required, stale
		switch run.Conclusion {
		case "success", "skipped", "neutral":
			// These are acceptable outcomes
		default:
			anyFailed = true
		}
	}

	// Failure takes precedence over pending
	if anyFailed {
		return CheckFailure, nil
	}
	if !allComplete {
		return CheckPending, nil
	}
	return CheckSuccess, nil
}

// getCheckRuns fetches all check runs for a git ref from the GitHub API.
// It handles pagination to ensure all check runs are returned.
func (c *PRClient) getCheckRuns(ctx context.Context, ref string) ([]CheckRun, error) {
	var allRuns []CheckRun
	page := 1
	perPage := 100 // Max allowed by GitHub API

	for {
		url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs?per_page=%d&page=%d",
			c.baseURL, c.owner, c.repo, ref, perPage, page)

		resp, err := c.doRequest(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		var response CheckRunsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		allRuns = append(allRuns, response.CheckRuns...)

		// Check if we've fetched all runs
		if len(allRuns) >= response.TotalCount || len(response.CheckRuns) < perPage {
			break
		}
		page++
	}

	return allRuns, nil
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
