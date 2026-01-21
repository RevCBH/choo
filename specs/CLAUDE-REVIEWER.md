# CLAUDE-REVIEWER — Claude Reviewer Implementation Using Diff-Based Prompts for Structured JSON Output

## Overview

The ClaudeReviewer implements the Reviewer interface for automated code review using Claude CLI. It retrieves the git diff between the current HEAD and a base branch, constructs a review prompt requesting structured JSON output, invokes Claude, and parses the response into actionable review issues.

This component enables automated code review as part of the CI/CD pipeline. By requesting JSON output from Claude, the reviewer can extract structured issues with file locations, line numbers, severities, and fix suggestions. The design prioritizes graceful degradation: if JSON parsing fails, the review passes to avoid blocking development workflows.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ClaudeReviewer Flow                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                 │
│  │  git diff    │────▶│  Build       │────▶│  Invoke      │                 │
│  │  base...HEAD │     │  Prompt      │     │  Claude CLI  │                 │
│  └──────────────┘     └──────────────┘     └──────────────┘                 │
│                                                   │                          │
│                                                   ▼                          │
│                       ┌──────────────┐     ┌──────────────┐                 │
│                       │  ReviewResult│◀────│  Parse JSON  │                 │
│                       │  (Issues)    │     │  Output      │                 │
│                       └──────────────┘     └──────────────┘                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. ClaudeReviewer MUST implement the Reviewer interface
2. System MUST retrieve git diff using `git diff <baseBranch>...HEAD`
3. System MUST build a review prompt requesting JSON output with specific schema
4. System MUST invoke Claude CLI with non-interactive flags (`--dangerously-skip-permissions`, `--print`, `-p`)
5. System MUST parse Claude's JSON output into structured ReviewIssue types
6. System MUST return passed=true with no issues when diff is empty
7. System MUST gracefully degrade to passed=true if JSON parsing fails
8. System MUST support configurable Claude CLI command path
9. System MUST return the ProviderClaude provider type from Name()

### Performance Requirements

| Metric | Target |
|--------|--------|
| Git diff execution | <500ms |
| Claude CLI startup | <2s |
| JSON parsing | <10ms |
| Total review (excluding Claude inference) | <3s |

### Constraints

- Depends on REVIEWER-INTERFACE for the Reviewer interface definition
- Requires `claude` CLI installed and accessible
- Cannot control Claude model parameters through this interface
- JSON extraction from Claude output may fail for complex responses
- Review quality depends on prompt engineering and model capabilities

## Design

### Module Structure

```
internal/provider/
├── reviewer.go         # Reviewer interface and types (REVIEWER-INTERFACE)
├── claude_reviewer.go  # ClaudeReviewer implementation
└── prompt.go           # BuildClaudeReviewPrompt function
```

### Core Types

```go
// internal/provider/reviewer.go (from REVIEWER-INTERFACE)

// ProviderType identifies which provider is being used.
type ProviderType string

const (
    ProviderClaude ProviderType = "claude"
    ProviderCodex  ProviderType = "codex"
)

// ReviewResult contains the outcome of a code review.
type ReviewResult struct {
    Passed    bool          // Whether the review passed
    Summary   string        // Brief summary of findings
    Issues    []ReviewIssue // List of identified issues
    RawOutput string        // Original output for debugging
}

// ReviewIssue represents a single issue found during review.
type ReviewIssue struct {
    File       string `json:"file"`       // Path to the file with issue
    Line       int    `json:"line"`       // Line number of the issue
    Severity   string `json:"severity"`   // "error", "warning", or "suggestion"
    Message    string `json:"message"`    // Description of the issue
    Suggestion string `json:"suggestion"` // How to fix the issue
}

// Reviewer defines the interface for code review providers.
type Reviewer interface {
    Name() ProviderType
    Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
}
```

```go
// internal/provider/claude_reviewer.go

// ClaudeReviewer implements Reviewer using Claude with diff-based prompts.
// It retrieves the git diff, builds a prompt requesting JSON output,
// and parses the structured response into ReviewIssue types.
type ClaudeReviewer struct {
    command string // Path to claude CLI executable
}
```

### API Surface

```go
// internal/provider/claude_reviewer.go

// NewClaudeReviewer creates a ClaudeReviewer with optional command override.
// If command is empty, defaults to "claude" (resolved via PATH).
func NewClaudeReviewer(command string) *ClaudeReviewer

// Name returns ProviderClaude to identify this reviewer.
func (r *ClaudeReviewer) Name() ProviderType

// Review performs code review by getting the diff, invoking Claude,
// and parsing the structured JSON response.
func (r *ClaudeReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
```

```go
// internal/provider/prompt.go

// BuildClaudeReviewPrompt creates a prompt for Claude to review a diff.
// The prompt requests JSON output with a specific schema for parsing.
func BuildClaudeReviewPrompt(diff string) string
```

### Full Implementation

```go
// internal/provider/claude_reviewer.go

package provider

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
)

// ClaudeReviewer implements Reviewer using Claude with diff-based prompts.
type ClaudeReviewer struct {
    command string // Path to claude CLI
}

// NewClaudeReviewer creates a ClaudeReviewer with optional command override.
func NewClaudeReviewer(command string) *ClaudeReviewer {
    if command == "" {
        command = "claude"
    }
    return &ClaudeReviewer{command: command}
}

func (r *ClaudeReviewer) Name() ProviderType {
    return ProviderClaude
}

func (r *ClaudeReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
    // Get diff for review
    diff, err := r.getDiff(ctx, workdir, baseBranch)
    if err != nil {
        return nil, fmt.Errorf("failed to get diff: %w", err)
    }

    if diff == "" {
        return &ReviewResult{
            Passed:  true,
            Summary: "No changes to review",
        }, nil
    }

    // Build review prompt requesting JSON output
    prompt := BuildClaudeReviewPrompt(diff)

    // Invoke Claude with non-interactive flags to prevent hangs:
    // --dangerously-skip-permissions: bypass interactive permission prompts
    // --print: output to stdout instead of interactive mode
    // -p: provide the prompt
    cmd := exec.CommandContext(ctx, r.command,
        "--dangerously-skip-permissions",
        "--print",
        "-p", prompt,
    )
    cmd.Dir = workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("claude review failed: %w", err)
    }

    return r.parseOutput(string(output))
}

func (r *ClaudeReviewer) getDiff(ctx context.Context, workdir, baseBranch string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "diff", baseBranch+"...HEAD")
    cmd.Dir = workdir
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return string(output), nil
}

func (r *ClaudeReviewer) parseOutput(output string) (*ReviewResult, error) {
    // Extract JSON from Claude's response
    jsonStr := extractJSON(output)
    if jsonStr == "" {
        return &ReviewResult{
            Passed:    true,
            Summary:   "No structured review output",
            RawOutput: output,
        }, nil
    }

    var parsed struct {
        Passed  bool          `json:"passed"`
        Summary string        `json:"summary"`
        Issues  []ReviewIssue `json:"issues"`
    }

    if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
        return &ReviewResult{
            Passed:    true,
            Summary:   "Failed to parse review output",
            RawOutput: output,
        }, nil
    }

    return &ReviewResult{
        Passed:    parsed.Passed,
        Summary:   parsed.Summary,
        Issues:    parsed.Issues,
        RawOutput: output,
    }, nil
}

// extractJSON finds and returns the first JSON object in the output.
// Handles JSON in markdown code fences and bare JSON.
// Returns empty string if no valid JSON found.
func extractJSON(output string) string {
    // First, try to extract JSON from markdown code fence
    // Pattern: ```json\n{...}\n``` or ```\n{...}\n```
    if jsonStr := extractJSONFromCodeFence(output); jsonStr != "" {
        return jsonStr
    }

    // Fall back to finding bare JSON by brace matching
    return extractJSONByBraces(output)
}

// extractJSONFromCodeFence extracts JSON from markdown code fences.
func extractJSONFromCodeFence(output string) string {
    // Look for ```json or ``` followed by {
    markers := []string{"```json\n", "```\n"}
    for _, marker := range markers {
        start := strings.Index(output, marker)
        if start == -1 {
            continue
        }
        contentStart := start + len(marker)
        // Find the closing ```
        end := strings.Index(output[contentStart:], "```")
        if end == -1 {
            continue
        }
        content := strings.TrimSpace(output[contentStart : contentStart+end])
        if strings.HasPrefix(content, "{") {
            return content
        }
    }
    return ""
}

// extractJSONByBraces finds JSON by matching braces.
func extractJSONByBraces(output string) string {
    start := -1
    depth := 0

    for i, ch := range output {
        if ch == '{' {
            if start == -1 {
                start = i
            }
            depth++
        } else if ch == '}' {
            depth--
            if depth == 0 && start != -1 {
                return output[start : i+1]
            }
        }
    }

    return ""
}
```

```go
// internal/provider/prompt.go

package provider

import "fmt"

// BuildClaudeReviewPrompt creates a prompt for Claude to review a diff.
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

### Diff Format

The reviewer uses `git diff <baseBranch>...HEAD` which shows changes between the merge base of baseBranch and HEAD, and HEAD itself. This captures all commits on the current branch since it diverged from the base branch.

Example diff output format:
```diff
diff --git a/internal/provider/claude.go b/internal/provider/claude.go
index abc123..def456 100644
--- a/internal/provider/claude.go
+++ b/internal/provider/claude.go
@@ -10,6 +10,7 @@ func NewClaudeProvider() *ClaudeProvider {
     return &ClaudeProvider{
         command: "claude",
+        timeout: 30 * time.Second,
     }
 }
```

### Non-Interactive CLI Flags

The Claude CLI requires specific flags for non-interactive (automated) operation:

| Flag | Purpose |
|------|---------|
| `--dangerously-skip-permissions` | Bypass interactive permission prompts |
| `--print` | Output to stdout instead of interactive TUI mode |
| `-p` | Provide the prompt as an argument |

Without these flags, the Claude CLI may hang waiting for user input, which would cause review timeouts.

### JSON Extraction Strategy

Claude's output may include explanatory text before and after the JSON. The `extractJSON` function handles multiple formats:

1. **Markdown code fences**: First checks for ` ```json ` or ` ``` ` blocks
2. **Bare JSON**: Falls back to brace matching to find the outermost JSON object

The brace matching approach:
1. Scan for first `{` character
2. Track brace depth (increment on `{`, decrement on `}`)
3. Return substring when depth returns to 0
4. Return empty string if no valid JSON found

This handles Claude wrapping JSON in markdown (common behavior) and nested JSON objects.

### Graceful Degradation

When JSON parsing fails, the reviewer returns `passed=true` to avoid blocking development:

| Scenario | Result |
|----------|--------|
| Empty diff | `passed=true`, summary="No changes to review" |
| No JSON in output | `passed=true`, summary="No structured review output" |
| Malformed JSON | `passed=true`, summary="Failed to parse review output" |
| Valid JSON, no issues | `passed=true`, issues=[] |
| Valid JSON, issues found | `passed=false`, issues populated |

This design choice prioritizes developer velocity over strict review enforcement.

### Context Cancellation

The reviewer respects context cancellation at two points:
1. During `git diff` execution
2. During Claude CLI invocation

When cancelled, `exec.CommandContext` sends SIGKILL to the subprocess.

### Security Considerations

The diff is passed directly in the prompt. For large diffs:
- Claude CLI may have input length limits
- Token usage scales with diff size
- Consider truncating very large diffs or reviewing file-by-file

The prompt does not include any secrets, but the diff may contain sensitive code changes.

## Testing Strategy

### Unit Tests

```go
// internal/provider/claude_reviewer_test.go

package provider

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestNewClaudeReviewer_DefaultCommand(t *testing.T) {
    r := NewClaudeReviewer("")
    if r.command != "claude" {
        t.Errorf("expected default command 'claude', got %q", r.command)
    }
}

func TestNewClaudeReviewer_CustomCommand(t *testing.T) {
    r := NewClaudeReviewer("/usr/local/bin/claude")
    if r.command != "/usr/local/bin/claude" {
        t.Errorf("expected custom command, got %q", r.command)
    }
}

func TestClaudeReviewer_Name(t *testing.T) {
    r := NewClaudeReviewer("")
    if r.Name() != ProviderClaude {
        t.Errorf("expected ProviderClaude, got %v", r.Name())
    }
}

func TestExtractJSON_ValidJSON(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "plain JSON",
            input:    `{"passed": true, "summary": "LGTM", "issues": []}`,
            expected: `{"passed": true, "summary": "LGTM", "issues": []}`,
        },
        {
            name:     "JSON with prefix text",
            input:    "Here is my review:\n{\"passed\": false, \"summary\": \"Issues found\", \"issues\": []}",
            expected: `{"passed": false, "summary": "Issues found", "issues": []}`,
        },
        {
            name:     "JSON with suffix text",
            input:    "{\"passed\": true, \"summary\": \"OK\", \"issues\": []}\nLet me know if you have questions.",
            expected: `{"passed": true, "summary": "OK", "issues": []}`,
        },
        {
            name:     "nested JSON",
            input:    `{"passed": true, "summary": "OK", "issues": [{"file": "test.go", "line": 1}]}`,
            expected: `{"passed": true, "summary": "OK", "issues": [{"file": "test.go", "line": 1}]}`,
        },
        {
            name: "JSON in markdown code fence",
            input: "Here is the review:\n```json\n{\"passed\": true, \"summary\": \"OK\", \"issues\": []}\n```\nDone.",
            expected: `{"passed": true, "summary": "OK", "issues": []}`,
        },
        {
            name: "JSON in plain code fence",
            input: "```\n{\"passed\": false, \"summary\": \"Issues\", \"issues\": []}\n```",
            expected: `{"passed": false, "summary": "Issues", "issues": []}`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := extractJSON(tt.input)
            if got != tt.expected {
                t.Errorf("extractJSON() = %q, want %q", got, tt.expected)
            }
        })
    }
}

func TestExtractJSON_NoJSON(t *testing.T) {
    inputs := []string{
        "No JSON here",
        "Just some text without braces",
        "Incomplete {",
        "} backwards",
    }

    for _, input := range inputs {
        got := extractJSON(input)
        if got != "" {
            t.Errorf("extractJSON(%q) = %q, want empty string", input, got)
        }
    }
}

func TestClaudeReviewer_ParseOutput_ValidJSON(t *testing.T) {
    r := NewClaudeReviewer("")
    output := `{"passed": false, "summary": "Found issues", "issues": [{"file": "main.go", "line": 10, "severity": "error", "message": "Nil pointer", "suggestion": "Check for nil"}]}`

    result, err := r.parseOutput(output)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if result.Passed {
        t.Error("expected passed=false")
    }
    if result.Summary != "Found issues" {
        t.Errorf("summary = %q, want %q", result.Summary, "Found issues")
    }
    if len(result.Issues) != 1 {
        t.Fatalf("issues count = %d, want 1", len(result.Issues))
    }
    if result.Issues[0].File != "main.go" {
        t.Errorf("issue file = %q, want %q", result.Issues[0].File, "main.go")
    }
    if result.Issues[0].Line != 10 {
        t.Errorf("issue line = %d, want %d", result.Issues[0].Line, 10)
    }
}

func TestClaudeReviewer_ParseOutput_NoJSON(t *testing.T) {
    r := NewClaudeReviewer("")
    output := "The code looks good to me!"

    result, err := r.parseOutput(output)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if !result.Passed {
        t.Error("expected passed=true for graceful degradation")
    }
    if result.Summary != "No structured review output" {
        t.Errorf("summary = %q, want %q", result.Summary, "No structured review output")
    }
    if result.RawOutput != output {
        t.Error("expected raw output to be preserved")
    }
}

func TestClaudeReviewer_ParseOutput_MalformedJSON(t *testing.T) {
    r := NewClaudeReviewer("")
    output := `{"passed": true, "summary": "OK", "issues": [invalid]}`

    result, err := r.parseOutput(output)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if !result.Passed {
        t.Error("expected passed=true for graceful degradation")
    }
    if result.Summary != "Failed to parse review output" {
        t.Errorf("summary = %q, want %q", result.Summary, "Failed to parse review output")
    }
}

func TestBuildClaudeReviewPrompt(t *testing.T) {
    diff := "diff --git a/test.go b/test.go\n+new line"
    prompt := BuildClaudeReviewPrompt(diff)

    // Verify prompt contains key elements
    if !containsString(prompt, "Review the following code changes") {
        t.Error("prompt missing review instruction")
    }
    if !containsString(prompt, "Bugs or logical errors") {
        t.Error("prompt missing bugs criteria")
    }
    if !containsString(prompt, "Security vulnerabilities") {
        t.Error("prompt missing security criteria")
    }
    if !containsString(prompt, `"passed": true/false`) {
        t.Error("prompt missing JSON schema")
    }
    if !containsString(prompt, diff) {
        t.Error("prompt missing diff content")
    }
}

func containsString(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}
```

```go
// internal/provider/claude_reviewer_integration_test.go

package provider

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func TestClaudeReviewer_GetDiff_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create a temporary git repo
    tmpDir := t.TempDir()
    runGit := func(args ...string) {
        cmd := exec.Command("git", args...)
        cmd.Dir = tmpDir
        if err := cmd.Run(); err != nil {
            t.Fatalf("git %v failed: %v", args, err)
        }
    }

    // Initialize repo with a commit
    runGit("init")
    runGit("config", "user.email", "test@test.com")
    runGit("config", "user.name", "Test")

    testFile := filepath.Join(tmpDir, "test.go")
    os.WriteFile(testFile, []byte("package main\n"), 0644)
    runGit("add", ".")
    runGit("commit", "-m", "initial")

    // Create feature branch with changes
    runGit("checkout", "-b", "feature")
    os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644)
    runGit("add", ".")
    runGit("commit", "-m", "add main")

    // Test getDiff
    r := NewClaudeReviewer("")
    diff, err := r.getDiff(context.Background(), tmpDir, "master")
    if err != nil {
        t.Fatalf("getDiff failed: %v", err)
    }

    if diff == "" {
        t.Error("expected non-empty diff")
    }
    if !containsString(diff, "+func main()") {
        t.Errorf("diff missing expected content: %s", diff)
    }
}

func TestClaudeReviewer_Review_EmptyDiff(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create a temporary git repo with no changes
    tmpDir := t.TempDir()
    runGit := func(args ...string) {
        cmd := exec.Command("git", args...)
        cmd.Dir = tmpDir
        if err := cmd.Run(); err != nil {
            t.Fatalf("git %v failed: %v", args, err)
        }
    }

    runGit("init")
    runGit("config", "user.email", "test@test.com")
    runGit("config", "user.name", "Test")

    testFile := filepath.Join(tmpDir, "test.go")
    os.WriteFile(testFile, []byte("package main\n"), 0644)
    runGit("add", ".")
    runGit("commit", "-m", "initial")

    // Review on same branch (empty diff)
    r := NewClaudeReviewer("")
    result, err := r.Review(context.Background(), tmpDir, "HEAD")
    if err != nil {
        t.Fatalf("Review failed: %v", err)
    }

    if !result.Passed {
        t.Error("expected passed=true for empty diff")
    }
    if result.Summary != "No changes to review" {
        t.Errorf("summary = %q, want %q", result.Summary, "No changes to review")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Review with real Claude CLI | Requires claude CLI; create repo with known issues |
| Review passes clean code | Create repo with well-written code, verify passed=true |
| Review finds issues | Create repo with obvious bug, verify issues detected |
| Context cancellation | Start review, cancel context, verify termination |
| Large diff handling | Create repo with many file changes, verify completion |

### Manual Testing

- [ ] ClaudeReviewer invokes claude with all required flags (`--dangerously-skip-permissions --print -p`)
- [ ] Review does not hang (non-interactive mode works)
- [ ] Review of empty diff returns passed=true immediately
- [ ] Review identifies obvious bugs in diff
- [ ] Review returns structured issues with file and line numbers
- [ ] Graceful degradation when Claude output lacks JSON
- [ ] JSON extraction works when Claude wraps in markdown code fence
- [ ] Context cancellation terminates review promptly
- [ ] Custom command path override works correctly

## Design Decisions

### Why Diff-Based Review?

Reviewing the full codebase would be expensive and noisy. Diff-based review:
- Focuses on new/changed code
- Reduces token usage and latency
- Provides relevant context for changes
- Matches how human reviewers work

Trade-off: May miss issues related to unchanged code interactions.

### Why JSON Output Format?

Structured JSON output enables:
- Programmatic parsing of issues
- Integration with CI systems
- Consistent issue format across reviews
- Machine-readable severities and suggestions

Trade-off: Claude may not always produce valid JSON. The graceful degradation handles this.

### Why Graceful Degradation to passed=true?

Failing reviews when parsing fails would:
- Block development for parsing issues
- Create frustration with false negatives
- Require manual intervention

Passing by default prioritizes developer velocity. Teams wanting strict enforcement can wrap the reviewer with additional validation.

### Why Not Use Claude's Structured Output API?

The Claude CLI does not currently support structured output mode. The prompt-based approach:
- Works with any Claude CLI version
- Requires no API changes
- Allows prompt iteration without code changes

Future enhancement: If Claude CLI adds structured output support, migrate to that for more reliable parsing.

## Future Enhancements

1. **Structured output mode**: Use Claude's structured output API when available in CLI
2. **Review caching**: Cache reviews for unchanged diffs to reduce API calls
3. **Incremental review**: Review only new commits since last review
4. **Custom review criteria**: Allow configuration of focus areas in the prompt
5. **Multi-file batching**: Split large diffs into file-based chunks
6. **Review history**: Track review results over time for trend analysis
7. **GitHub PR comments**: Post issues as inline PR comments

## References

- [REVIEWER-INTERFACE spec](REVIEWER-INTERFACE.md) - Interface definition this module implements
- [CODE-REVIEW PRD](../../../docs/prd/CODE-REVIEW.md) - Original requirements
- [PROVIDER-IMPLEMENTATIONS spec](../../completed/PROVIDER-IMPLEMENTATIONS.md) - Related provider pattern
