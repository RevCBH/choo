---
task: 1
status: complete
backpressure: "go test ./internal/provider/... -run TestReview"
depends_on: []
---

# Reviewer Interface and Types

**Parent spec**: `specs/REVIEWER-INTERFACE.md`
**Task**: #1 of 1 in implementation plan

## Objective

Define the Reviewer interface and its associated types (ReviewResult, ReviewIssue) with severity constants and helper methods for advisory code review.

## Dependencies

### External Specs (must be implemented)
- None (foundational unit)

### Task Dependencies (within this unit)
- None (first and only task)

### Package Dependencies
- `context` - for context-aware Review method

## Deliverables

### Files to Create/Modify

```
internal/provider/
└── reviewer.go       # CREATE: Reviewer interface and result types
└── reviewer_test.go  # CREATE: Unit tests for helper methods
```

### Types to Implement

```go
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
```

### Constants to Implement

```go
// Severity levels for review issues.
const (
    SeverityError      = "error"      // Must fix: bugs, security issues
    SeverityWarning    = "warning"    // Should fix: code smells, potential issues
    SeveritySuggestion = "suggestion" // Nice to have: style, improvements
    SeverityInfo       = "info"       // Informational: observations, notes
)

// ValidSeverities contains all recognized severity levels.
var ValidSeverities = []string{SeverityError, SeverityWarning, SeveritySuggestion, SeverityInfo}
```

### Functions to Implement

```go
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
```

## Backpressure

### Validation Command

```bash
go test ./internal/provider/... -run TestReview -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestReviewResult_HasErrors_NoIssues` | `HasErrors() == false` when Issues is nil/empty |
| `TestReviewResult_HasErrors_OnlyWarnings` | `HasErrors() == false` when only warnings present |
| `TestReviewResult_HasErrors_WithError` | `HasErrors() == true` when error issue exists |
| `TestReviewResult_ErrorCount` | Returns correct count of error-severity issues |
| `TestReviewResult_WarningCount` | Returns correct count of warning-severity issues |
| `TestReviewResult_IssuesByFile` | Groups issues correctly by file path |
| `TestReviewResult_IssuesByFile_Empty` | Returns empty map for no issues |
| `TestIsValidSeverity_Valid` | Returns true for "error", "warning", "suggestion", "info" |
| `TestIsValidSeverity_Invalid` | Returns false for "critical", "", "ERROR" (case sensitive) |

### Test Cases

```go
func TestReviewResult_HasErrors(t *testing.T) {
    tests := []struct {
        name     string
        issues   []ReviewIssue
        expected bool
    }{
        {
            name:     "no issues",
            issues:   nil,
            expected: false,
        },
        {
            name: "only warnings",
            issues: []ReviewIssue{
                {Severity: SeverityWarning, Message: "unused variable"},
            },
            expected: false,
        },
        {
            name: "has error",
            issues: []ReviewIssue{
                {Severity: SeverityWarning, Message: "unused variable"},
                {Severity: SeverityError, Message: "nil pointer dereference"},
            },
            expected: true,
        },
    }
    // ... test implementation
}

func TestReviewResult_IssuesByFile(t *testing.T) {
    result := &ReviewResult{
        Issues: []ReviewIssue{
            {File: "main.go", Line: 10, Message: "issue 1"},
            {File: "main.go", Line: 20, Message: "issue 2"},
            {File: "util.go", Line: 5, Message: "issue 3"},
        },
    }
    grouped := result.IssuesByFile()
    // Verify main.go has 2 issues, util.go has 1
}

func TestIsValidSeverity(t *testing.T) {
    tests := []struct {
        severity string
        valid    bool
    }{
        {"error", true},
        {"warning", true},
        {"suggestion", true},
        {"info", true},
        {"critical", false},
        {"", false},
        {"ERROR", false}, // Case sensitive
    }
    // ... test implementation
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] No test fixtures required
- [x] Runs in <60 seconds

## Implementation Notes

- The Reviewer interface reuses `ProviderType` from the existing provider package for consistency
- `Line = 0` indicates line not applicable (file-level or repository-level issues)
- Review errors are advisory and should never block merges or crash workers
- RawOutput is preserved for debugging when structured parsing fails
- Severity constants are lowercase strings to match typical linter output

## NOT In Scope

- Concrete Reviewer implementations (Codex reviewer in separate unit)
- Review invocation logic (handled by worker)
- Event emission for review results (handled by worker)
- Output parsing from specific tools (implementation-specific)
