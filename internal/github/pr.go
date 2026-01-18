package github

import "time"

// PRInfo holds information about a created PR
type PRInfo struct {
	Number       int
	URL          string
	Branch       string
	TargetBranch string
	Title        string
	CreatedAt    time.Time
}

// MergeResult holds the result of a merge operation
type MergeResult struct {
	Merged  bool
	SHA     string
	Message string
}
