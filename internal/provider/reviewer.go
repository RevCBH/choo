package provider

import "context"

// Reviewer performs code review on changes in a worktree.
// Unlike Provider (task execution), Reviewer produces structured feedback.
type Reviewer interface {
	// Review examines changes between baseBranch and HEAD in workdir.
	// Returns structured review results or error.
	// Errors should be treated as non-fatal (advisory review).
	Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)

	// Name returns the provider type for logging/events.
	Name() ProviderType
}

// ReviewResult contains the structured output of a code review.
type ReviewResult struct {
	// Passed is true if no issues were found.
	Passed bool

	// Issues contains individual review findings.
	Issues []ReviewIssue

	// Summary is a human-readable overview of the review.
	Summary string

	// RawOutput preserves the original reviewer output for debugging.
	RawOutput string
}

// ReviewIssue represents a single finding from the code review.
type ReviewIssue struct {
	// File is the path to the file containing the issue.
	File string

	// Line is the line number (0 if not applicable).
	Line int

	// Severity indicates issue importance: "error", "warning", "suggestion", "info".
	Severity string

	// Message describes what the issue is.
	Message string

	// Suggestion provides recommended fix (may be empty).
	Suggestion string
}

// Severity levels for review issues.
const (
	SeverityError      = "error"      // Must fix: bugs, security issues
	SeverityWarning    = "warning"    // Should fix: code smells, potential issues
	SeveritySuggestion = "suggestion" // Nice to have: style, improvements
	SeverityInfo       = "info"       // Informational: observations, notes
)

// ValidSeverities contains all recognized severity levels.
var ValidSeverities = []string{SeverityError, SeverityWarning, SeveritySuggestion, SeverityInfo}

// HasErrors returns true if any issue has severity "error".
func (r *ReviewResult) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of issues with severity "error".
func (r *ReviewResult) ErrorCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of issues with severity "warning".
func (r *ReviewResult) WarningCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// IssuesByFile groups issues by their file path.
func (r *ReviewResult) IssuesByFile() map[string][]ReviewIssue {
	result := make(map[string][]ReviewIssue)
	for _, issue := range r.Issues {
		result[issue.File] = append(result[issue.File], issue)
	}
	return result
}

// IsValidSeverity checks if a severity string is recognized.
func IsValidSeverity(s string) bool {
	for _, v := range ValidSeverities {
		if v == s {
			return true
		}
	}
	return false
}
