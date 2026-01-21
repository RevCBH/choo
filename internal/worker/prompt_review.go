// internal/worker/prompt_review.go

package worker

import (
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/provider"
)

// BuildReviewFixPrompt creates a prompt for the task provider to fix review issues.
func BuildReviewFixPrompt(issues []provider.ReviewIssue) string {
	var sb strings.Builder

	sb.WriteString("Code review found the following issues that need to be addressed:\n\n")

	for i, issue := range issues {
		sb.WriteString(fmt.Sprintf("## Issue %d: %s\n", i+1, issue.Severity))
		if issue.File != "" {
			sb.WriteString(fmt.Sprintf("**File**: %s", issue.File))
			if issue.Line > 0 {
				sb.WriteString(fmt.Sprintf(":%d", issue.Line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("**Problem**: %s\n", issue.Message))
		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("**Suggestion**: %s\n", issue.Suggestion))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Please address these issues. Focus on the most critical ones first.\n")
	sb.WriteString("Make minimal changes needed to resolve the issues.\n")

	return sb.String()
}
