# REVIEWER-INTERFACE — Reviewer interface and result types for advisory code review system

## Overview

The Reviewer interface defines the contract for advisory code review providers in Columbus. After all tasks in a unit complete successfully, the system runs a code review against the accumulated changes. This review examines the diff between the local feature branch (containing any prior unit merges) and HEAD, producing structured feedback about potential issues. A second review runs after all units are merged to the feature branch, before the final rebase/merge to the target branch.

Unlike the Provider interface (which executes tasks and returns success/failure), Reviewer produces structured output that must be parsed into discrete issues. The review is purely advisory—it never blocks merges or causes unit failure. This separation allows different implementations (Codex, Claude, etc.) to provide review capabilities independent of their task execution roles.

The design prioritizes resilience: review failures are logged but do not crash workers or affect unit completion. Raw output is preserved alongside structured results for debugging when parsing fails or produces unexpected results.

## Requirements

### Functional Requirements

1. **FR-1**: Reviewer MUST accept a working directory path and base branch name as inputs
2. **FR-2**: Reviewer MUST examine changes between base branch and HEAD
3. **FR-3**: Reviewer MUST return structured results containing pass/fail status, issues, and summary
4. **FR-4**: Reviewer MUST preserve raw output from the underlying review tool
5. **FR-5**: System MUST parse review output into structured issues with file, line, severity, and message
6. **FR-6**: Reviewer MUST identify itself via Name() for logging and event emission
7. **FR-7**: Each ReviewIssue MUST include severity level: "error", "warning", "suggestion", or "info"

### Performance Requirements

| Metric | Target |
|--------|--------|
| Review timeout | Configurable, default 5 minutes |
| Memory overhead | < 50MB for result parsing |
| Issue limit | Handle up to 1000 issues per review |

### Constraints

- Review providers MUST be independent of task providers (NFR-4)
- Review failures MUST NOT crash the worker (NFR-2)
- Review errors are non-fatal; the merge always proceeds
- Context cancellation MUST be respected for graceful shutdown

## Design

### Module Structure

```
internal/provider/
├── reviewer.go      # Reviewer interface and result types
├── provider.go      # Existing Provider interface (task execution)
└── codex/
    └── reviewer.go  # Codex implementation of Reviewer
```

### Core Types

```go
// internal/provider/reviewer.go

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

    // Severity indicates issue importance: "error", "warning", "suggestion".
    Severity string

    // Message describes what the issue is.
    Message string

    // Suggestion provides recommended fix (may be empty).
    Suggestion string
}
```

### API Surface

```go
// Reviewer interface methods (implementations must satisfy these)
//
// Review examines changes and returns structured feedback.
// workdir: absolute path to the git worktree
// baseBranch: branch to diff against (e.g., "main", "feature-branch")
// Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
//
// Name returns the provider type for logging and event attribution.
// Name() ProviderType

// Helper methods on ReviewResult

// HasErrors returns true if any issue has severity "error".
func (r *ReviewResult) HasErrors() bool {
    for _, issue := range r.Issues {
        if issue.Severity == "error" {
            return true
        }
    }
    return false
}

// IssuesByFile groups issues by their file path.
func (r *ReviewResult) IssuesByFile() map[string][]ReviewIssue {
    result := make(map[string][]ReviewIssue)
    for _, issue := range r.Issues {
        result[issue.File] = append(result[issue.File], issue)
    }
    return result
}

// ErrorCount returns the number of issues with severity "error".
func (r *ReviewResult) ErrorCount() int {
    count := 0
    for _, issue := range r.Issues {
        if issue.Severity == "error" {
            count++
        }
    }
    return count
}

// WarningCount returns the number of issues with severity "warning".
func (r *ReviewResult) WarningCount() int {
    count := 0
    for _, issue := range r.Issues {
        if issue.Severity == "warning" {
            count++
        }
    }
    return count
}
```

### Severity Constants

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

## Implementation Notes

### Error Handling

Review errors are advisory and should never crash the worker or block merges:

```go
func (w *Worker) runReview(ctx context.Context) {
    result, err := w.reviewer.Review(ctx, w.worktree, w.baseBranch)
    if err != nil {
        // Log error but continue - review is advisory
        w.logger.Warn("review failed", "error", err)
        w.emitEvent(EventReviewFailed, map[string]any{"error": err.Error()})
        return
    }

    // Process results even if issues found
    w.emitEvent(EventReviewComplete, map[string]any{
        "passed":       result.Passed,
        "issue_count":  len(result.Issues),
        "error_count":  result.ErrorCount(),
    })
}
```

### Context Cancellation

Reviewers must respect context cancellation for graceful shutdown:

```go
func (r *CodexReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
    cmd := exec.CommandContext(ctx, "codex", "review", "--base", baseBranch)
    cmd.Dir = workdir

    output, err := cmd.Output()
    if ctx.Err() != nil {
        return nil, fmt.Errorf("review cancelled: %w", ctx.Err())
    }
    if err != nil {
        return nil, fmt.Errorf("codex review failed: %w", err)
    }

    return r.parseOutput(output)
}
```

### Output Parsing

The RawOutput field preserves the original reviewer output for debugging when structured parsing fails:

```go
func (r *CodexReviewer) parseOutput(output []byte) (*ReviewResult, error) {
    result := &ReviewResult{
        RawOutput: string(output),
    }

    // Attempt structured parsing
    issues, err := r.extractIssues(output)
    if err != nil {
        // Parsing failed but we still have raw output
        result.Summary = "Review completed but output parsing failed"
        return result, nil // Non-fatal
    }

    result.Issues = issues
    result.Passed = len(issues) == 0
    result.Summary = r.generateSummary(issues)
    return result, nil
}
```

### Codex Review Invocation

The Codex reviewer uses `codex review --base <branch>` which differs from task execution (`codex exec --yolo`):

```go
func (r *CodexReviewer) buildCommand(baseBranch string) []string {
    return []string{
        "codex",
        "review",
        "--base", baseBranch,
        "--format", "json", // Request structured output if supported
    }
}
```

## Testing Strategy

### Unit Tests

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

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := &ReviewResult{Issues: tt.issues}
            if got := result.HasErrors(); got != tt.expected {
                t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
            }
        })
    }
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

    if len(grouped["main.go"]) != 2 {
        t.Errorf("expected 2 issues for main.go, got %d", len(grouped["main.go"]))
    }
    if len(grouped["util.go"]) != 1 {
        t.Errorf("expected 1 issue for util.go, got %d", len(grouped["util.go"]))
    }
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

    for _, tt := range tests {
        t.Run(tt.severity, func(t *testing.T) {
            if got := IsValidSeverity(tt.severity); got != tt.valid {
                t.Errorf("IsValidSeverity(%q) = %v, want %v", tt.severity, got, tt.valid)
            }
        })
    }
}

func TestReviewIssue_Fields(t *testing.T) {
    issue := ReviewIssue{
        File:       "internal/worker/task.go",
        Line:       42,
        Severity:   SeverityError,
        Message:    "potential nil pointer dereference",
        Suggestion: "add nil check before accessing field",
    }

    if issue.File != "internal/worker/task.go" {
        t.Errorf("unexpected file: %s", issue.File)
    }
    if issue.Line != 42 {
        t.Errorf("unexpected line: %d", issue.Line)
    }
    if issue.Severity != SeverityError {
        t.Errorf("unexpected severity: %s", issue.Severity)
    }
}
```

### Integration Tests

1. **Review with clean diff**: Verify Passed=true when no issues found
2. **Review with issues**: Verify issues are correctly parsed and categorized
3. **Review timeout**: Verify context cancellation terminates long-running reviews
4. **Review error handling**: Verify worker continues when review returns error
5. **Raw output preservation**: Verify RawOutput contains original tool output

### Manual Testing

- [ ] Run review against repository with known issues
- [ ] Verify issues map to correct files and line numbers
- [ ] Confirm review errors do not block merge
- [ ] Test with different base branches (main, feature branches)
- [ ] Verify timeout behavior with slow reviewer

## Design Decisions

### Why separate Reviewer from Provider?

The Provider interface handles task execution with simple success/failure outcomes. Reviewer has fundamentally different semantics:

1. **Structured output**: Reviews produce detailed findings that need parsing, not just exit codes
2. **Different CLI commands**: Codex uses `codex review --base` vs `codex exec --yolo`
3. **Non-blocking nature**: Task failures stop the unit; review failures are informational
4. **Independent selection**: Users may want different providers for tasks vs review

Combining these into a single interface would conflate two distinct responsibilities and complicate implementations.

### Why advisory (non-blocking) reviews?

1. **Developer autonomy**: Engineers should decide whether to address review feedback
2. **Merge velocity**: Blocking on review parsing failures would create frustrating false negatives
3. **Iteration speed**: Initial review implementations may have parsing bugs; non-blocking allows safe rollout
4. **Flexibility**: Some teams want informational reviews; others may build blocking workflows on top

The system emits events for review completion, allowing downstream systems to implement blocking behavior if desired.

### Why preserve RawOutput?

1. **Debugging**: When structured parsing fails or produces unexpected results, raw output aids investigation
2. **Flexibility**: Different reviewers may output in different formats; raw output provides fallback
3. **Auditability**: Complete record of what the reviewer actually said
4. **Evolution**: As parsing improves, historical raw output can be re-processed

### Why Line=0 for non-applicable issues?

Some review findings are file-level or repository-level (e.g., "missing license header" or "inconsistent naming across files"). Using 0 as a sentinel value is idiomatic in Go and clearly indicates "line not applicable" without requiring a pointer type.

## Future Enhancements

1. **Column support**: Add Column field to ReviewIssue for precise positioning in IDEs
2. **Code snippets**: Include surrounding code context in ReviewIssue for better understanding
3. **Auto-fix support**: Extend Suggestion to include structured patch data for automated fixes
4. **Category tags**: Add Tags field for categorizing issues (security, performance, style)
5. **Confidence scores**: Add confidence level to help prioritize review feedback
6. **Review caching**: Cache review results for unchanged file ranges
7. **Incremental review**: Review only files changed since last review

## References

- [Provider Interface](../PROVIDER-INTERFACE.md) - Task execution provider contract
- [Unit Lifecycle](../../UNIT-LIFECYCLE.md) - When reviews run in unit workflow
- [Event System](../../events/EVENTS.md) - EventReviewComplete, EventReviewFailed
