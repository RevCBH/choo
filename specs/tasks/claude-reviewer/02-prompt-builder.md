---
task: 2
status: pending
backpressure: "go test ./internal/provider/... -run TestBuildClaudeReviewPrompt -v"
depends_on: []
---

# Review Prompt Builder

**Parent spec**: `/specs/CLAUDE-REVIEWER.md`
**Task**: #2 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the BuildClaudeReviewPrompt function that constructs a review prompt requesting structured JSON output from Claude.

## Dependencies

### External Specs (must be implemented)
- None (this is a standalone utility function)

### Task Dependencies (within this unit)
- None (can be implemented in parallel with Task #1)

### Package Dependencies
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/provider/
└── prompt.go    # CREATE: BuildClaudeReviewPrompt function
```

### Functions to Implement

```go
// internal/provider/prompt.go

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
```

## Implementation Notes

- The prompt must include the exact JSON schema to guide Claude's output format
- The schema matches the ReviewIssue struct fields (file, line, severity, message, suggestion)
- The diff is appended at the end after a clear "DIFF:" marker
- Focus areas (bugs, security, performance, style) guide review quality
- The instruction for empty reviews ("passed": true, "issues": []) prevents ambiguous output

## Backpressure

### Validation Command

```bash
go test ./internal/provider/... -run TestBuildClaudeReviewPrompt -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestBuildClaudeReviewPrompt_ContainsReviewInstruction` | Prompt contains "Review the following code changes" |
| `TestBuildClaudeReviewPrompt_ContainsFocusAreas` | Prompt contains all focus areas: bugs, security, performance, style |
| `TestBuildClaudeReviewPrompt_ContainsJSONSchema` | Prompt contains `"passed": true/false` schema example |
| `TestBuildClaudeReviewPrompt_ContainsDiff` | Prompt contains the provided diff content after "DIFF:" marker |
| `TestBuildClaudeReviewPrompt_ContainsSeverityOptions` | Prompt contains severity options: error, warning, suggestion |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## NOT In Scope

- ClaudeReviewer struct and Review method (Task #1)
- JSON extraction and parsing (Task #3)
- Prompt optimization or A/B testing
- Custom focus areas or configurable prompts
