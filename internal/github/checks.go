// internal/github/checks.go
package github

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
