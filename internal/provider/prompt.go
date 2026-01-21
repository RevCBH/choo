package provider

import "fmt"

// BuildClaudeReviewPrompt creates a prompt for Claude to review a diff.
// The prompt requests JSON output with a specific schema for parsing.
func BuildClaudeReviewPrompt(diff string) string {
	return fmt.Sprintf(`Review the following code changes and identify any issues.

Focus on:
1. Bugs or logical errors
2. Security vulnerabilities
3. Performance problems
4. Code style and best practices

Output your review as JSON in this exact format:
{
  "passed": true/false,
  "summary": "Brief summary of findings",
  "issues": [
    {
      "file": "path/to/file.go",
      "line": 42,
      "severity": "error|warning|suggestion",
      "message": "Description of the issue",
      "suggestion": "How to fix it"
    }
  ]
}

If there are no issues, set "passed": true and "issues": [].

DIFF:
%s`, diff)
}
