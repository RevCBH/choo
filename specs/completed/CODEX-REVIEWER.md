# CODEX-REVIEWER — Codex CLI Code Review Provider Implementation

## Overview

The CODEX-REVIEWER component implements the Reviewer interface using the OpenAI Codex CLI. It provides automated code review capabilities by invoking `codex review --base <branch>` to compare changes against a base branch and parse the output into structured ReviewResult objects.

This component differs from the task execution provider (CodexProvider) which uses `codex exec --yolo`. The reviewer specifically targets the review subcommand to leverage Codex's code analysis capabilities. Exit code handling is critical: non-zero exits may indicate issues were found (not necessarily an error), requiring careful distinction between review findings and execution failures.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Code Review System                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐               │
│  │   Review     │    │   Codex      │    │   Output     │               │
│  │   Request    │───▶│   Reviewer   │───▶│   Parser     │               │
│  └──────────────┘    └──────┬───────┘    └──────────────┘               │
│                             │                                            │
│                             ▼                                            │
│                      ┌──────────────┐                                    │
│                      │  codex CLI   │                                    │
│                      │   review     │                                    │
│                      │ --base <br>  │                                    │
│                      └──────┬───────┘                                    │
│                             │                                            │
│         ┌───────────────────┼───────────────────┐                       │
│         ▼                   ▼                   ▼                        │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                │
│  │ Exit Code   │     │   Stdout    │     │   Stderr    │                │
│  │  Handling   │     │   Output    │     │   Errors    │                │
│  └─────────────┘     └─────────────┘     └─────────────┘                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. **FR-1**: Implement Reviewer interface for code review operations
2. **FR-2**: Invoke `codex review --base <baseBranch>` CLI command
3. **FR-3**: Execute review command in the specified working directory
4. **FR-4**: Parse Codex review output into structured ReviewResult
5. **FR-5**: Handle non-zero exit codes as potential issue indicators (not errors)
6. **FR-6**: Extract individual issues from Codex output format
7. **FR-7**: Support custom path to codex binary via configuration
8. **FR-8**: Capture combined stdout/stderr for parsing
9. **FR-9**: Propagate context cancellation to subprocess

### Performance Requirements

| Metric | Target |
|--------|--------|
| Review invocation startup | < 500ms |
| Output parsing | < 50ms |
| Context cancellation response | < 1s |

### Constraints

- Depends on REVIEWER-INTERFACE for the Reviewer interface contract
- Requires codex CLI installed and accessible (PATH or configured path)
- Output format depends on codex review specification (may evolve)
- Non-zero exit codes require special handling (issues vs errors)

## Design

### Module Structure

```
internal/provider/
├── provider.go         # Provider interface and types (existing)
├── codex.go           # CodexProvider for task execution (existing)
├── codex_reviewer.go  # CodexReviewer for code review (NEW)
└── codex_reviewer_test.go  # Unit tests (NEW)
```

### Core Types

```go
// internal/provider/codex_reviewer.go

package provider

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
)

// ReviewResult represents the outcome of a code review
type ReviewResult struct {
    // RawOutput contains the complete output from the review command
    RawOutput string

    // Passed indicates whether the review found no issues
    Passed bool

    // Summary provides a human-readable summary of the review
    Summary string

    // Issues contains structured details of each issue found
    Issues []ReviewIssue
}

// ReviewIssue represents a single issue found during review
type ReviewIssue struct {
    // File is the path to the file containing the issue
    File string

    // Line is the line number where the issue occurs (0 if unknown)
    Line int

    // Severity indicates the issue severity (e.g., "error", "warning", "info")
    Severity string

    // Message describes the issue
    Message string

    // Suggestion provides a recommended fix (optional)
    Suggestion string
}

// Reviewer defines the interface for code review providers
type Reviewer interface {
    // Review performs a code review comparing workdir against baseBranch
    Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)

    // Name returns the provider type identifier
    Name() ProviderType
}

// CodexReviewer implements Reviewer using Codex CLI.
type CodexReviewer struct {
    // command is the path to codex CLI, empty for system PATH
    command string
}
```

### API Surface

```go
// internal/provider/codex_reviewer.go

// NewCodexReviewer creates a CodexReviewer with optional command override.
// If command is empty, defaults to "codex" resolved via PATH.
func NewCodexReviewer(command string) *CodexReviewer

// Name returns ProviderCodex to identify this reviewer
func (r *CodexReviewer) Name() ProviderType

// Review executes codex review and returns structured results.
// The review compares changes in workdir against baseBranch.
// Non-zero exit codes indicate issues were found (not necessarily an error).
func (r *CodexReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
```

### Complete Implementation

```go
// internal/provider/codex_reviewer.go

package provider

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
)

// CodexReviewer implements Reviewer using Codex CLI.
type CodexReviewer struct {
    command string // Path to codex CLI, empty for system PATH
}

// NewCodexReviewer creates a CodexReviewer with optional command override.
func NewCodexReviewer(command string) *CodexReviewer {
    return &CodexReviewer{command: command}
}

// Name returns ProviderCodex
func (r *CodexReviewer) Name() ProviderType {
    return ProviderCodex
}

// Review performs a code review using codex CLI.
func (r *CodexReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
    cmdPath := r.command
    if cmdPath == "" {
        cmdPath = "codex"
    }

    // Invoke: codex review --base <baseBranch>
    cmd := exec.CommandContext(ctx, cmdPath, "review", "--base", baseBranch)
    cmd.Dir = workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode := exitErr.ExitCode()
            // Exit code 1 = issues found (not an error)
            if exitCode == 1 {
                return r.parseOutput(string(output), exitCode)
            }
            // Exit code 2+ = actual execution error
            return nil, fmt.Errorf("codex review error (exit %d): %s",
                exitCode, string(output))
        }
        // Command not found or other exec error
        return nil, fmt.Errorf("codex review failed: %w", err)
    }

    return r.parseOutput(string(output), 0)
}

// parseOutput converts codex output into structured ReviewResult
func (r *CodexReviewer) parseOutput(output string, exitCode int) (*ReviewResult, error) {
    result := &ReviewResult{
        RawOutput: output,
        Passed:    exitCode == 0,
        Issues:    []ReviewIssue{},
    }

    // Parse codex review output format
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        if issue := r.parseLine(line); issue != nil {
            result.Issues = append(result.Issues, *issue)
        }
    }

    if len(result.Issues) > 0 {
        result.Passed = false
        result.Summary = fmt.Sprintf("Found %d issues", len(result.Issues))
    } else if exitCode == 0 {
        result.Summary = "No issues found"
    } else {
        result.Summary = fmt.Sprintf("Review completed with exit code %d", exitCode)
    }

    return result, nil
}

// issuePattern matches common issue output formats:
// file.go:10: error: message
// file.go:10:5: warning: message
var issuePattern = regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?:\s*(\w+):\s*(.+)$`)

// parseLine attempts to parse a single line as an issue.
// Returns nil if the line is not an issue.
func (r *CodexReviewer) parseLine(line string) *ReviewIssue {
    line = strings.TrimSpace(line)
    if line == "" {
        return nil
    }

    matches := issuePattern.FindStringSubmatch(line)
    if matches == nil {
        return nil
    }

    lineNum, _ := strconv.Atoi(matches[2])

    return &ReviewIssue{
        File:     matches[1],
        Line:     lineNum,
        Severity: strings.ToLower(matches[3]),
        Message:  matches[4],
    }
}
```

## Implementation Notes

### Exit Code Handling

The codex review command uses exit codes to indicate review status:

| Exit Code | Meaning | Handling |
|-----------|---------|----------|
| 0 | No issues found | Passed = true |
| 1 | Issues found | Passed = false, parse issues |
| 2+ | Execution error | Return error |

The implementation distinguishes between issues found (exit code 1) and actual errors (exit code 2+, command not found, etc.).

```go
output, err := cmd.CombinedOutput()
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        // Exit code 1 = issues found, not an error
        if exitErr.ExitCode() == 1 {
            return r.parseOutput(string(output), 1)
        }
        // Exit code 2+ = actual error
        return nil, fmt.Errorf("codex review error (exit %d): %s",
            exitErr.ExitCode(), string(output))
    }
    // Command not found or other exec error
    return nil, fmt.Errorf("codex review failed: %w", err)
}
```

### Output Parsing

The parser handles common issue formats seen in linter and review tools:

```
path/to/file.go:42: error: undefined variable x
path/to/file.go:42:15: warning: unused import
path/to/file.go:10: info: consider renaming function
path/to/file.go:5: suggestion: use constants for magic numbers
```

The regex pattern `^([^:]+):(\d+)(?::\d+)?:\s*(\w+):\s*(.+)$` captures:
- Group 1: File path
- Group 2: Line number
- Optional column number (ignored)
- Group 3: Severity level (error, warning, suggestion, info)
- Group 4: Message text

Severity levels are normalized to lowercase for consistency with the `ValidSeverities` constants defined in REVIEWER-INTERFACE.

### Context Cancellation

The implementation properly propagates context cancellation to the subprocess:

```go
cmd := exec.CommandContext(ctx, cmdPath, "review", "--base", baseBranch)
```

When the context is cancelled, the subprocess receives SIGKILL after a grace period. The caller should handle context.Canceled and context.DeadlineExceeded errors appropriately.

### Command Configuration

The codex command path can be configured in two ways:

1. **System PATH** (default): Leave command empty, uses "codex"
2. **Explicit path**: Provide full path to codex binary

```yaml
# .choo.yaml
code_review:
  provider: codex
  command: "/usr/local/bin/codex"  # Optional: override CLI path
```

## Testing Strategy

### Unit Tests

```go
// internal/provider/codex_reviewer_test.go

package provider

import (
    "context"
    "testing"
)

func TestCodexReviewer_Name(t *testing.T) {
    reviewer := NewCodexReviewer("")
    if got := reviewer.Name(); got != ProviderCodex {
        t.Errorf("Name() = %v, want %v", got, ProviderCodex)
    }
}

func TestCodexReviewer_ParseOutput_NoIssues(t *testing.T) {
    reviewer := NewCodexReviewer("")
    output := "Review complete.\nNo issues found."

    result, err := reviewer.parseOutput(output, 0)
    if err != nil {
        t.Fatalf("parseOutput() error = %v", err)
    }

    if !result.Passed {
        t.Error("Passed = false, want true")
    }
    if len(result.Issues) != 0 {
        t.Errorf("Issues count = %d, want 0", len(result.Issues))
    }
    if result.Summary != "No issues found" {
        t.Errorf("Summary = %q, want %q", result.Summary, "No issues found")
    }
}

func TestCodexReviewer_ParseOutput_WithIssues(t *testing.T) {
    reviewer := NewCodexReviewer("")
    output := `main.go:10: error: undefined variable x
main.go:15: warning: unused import "fmt"
Review complete.`

    result, err := reviewer.parseOutput(output, 1)
    if err != nil {
        t.Fatalf("parseOutput() error = %v", err)
    }

    if result.Passed {
        t.Error("Passed = true, want false")
    }
    if len(result.Issues) != 2 {
        t.Fatalf("Issues count = %d, want 2", len(result.Issues))
    }

    // Verify first issue
    issue := result.Issues[0]
    if issue.File != "main.go" {
        t.Errorf("Issue[0].File = %q, want %q", issue.File, "main.go")
    }
    if issue.Line != 10 {
        t.Errorf("Issue[0].Line = %d, want %d", issue.Line, 10)
    }
    if issue.Severity != "error" {
        t.Errorf("Issue[0].Severity = %q, want %q", issue.Severity, "error")
    }
    if issue.Message != "undefined variable x" {
        t.Errorf("Issue[0].Message = %q, want %q", issue.Message, "undefined variable x")
    }
}

func TestCodexReviewer_ParseLine_Valid(t *testing.T) {
    tests := []struct {
        name     string
        line     string
        wantFile string
        wantLine int
        wantSev  string
        wantMsg  string
    }{
        {
            name:     "simple error",
            line:     "file.go:42: error: message here",
            wantFile: "file.go",
            wantLine: 42,
            wantSev:  "error",
            wantMsg:  "message here",
        },
        {
            name:     "with column",
            line:     "path/to/file.go:10:5: warning: unused var",
            wantFile: "path/to/file.go",
            wantLine: 10,
            wantSev:  "warning",
            wantMsg:  "unused var",
        },
        {
            name:     "info severity",
            line:     "test.go:1: info: consider renaming",
            wantFile: "test.go",
            wantLine: 1,
            wantSev:  "info",
            wantMsg:  "consider renaming",
        },
        {
            name:     "suggestion severity",
            line:     "util.go:25: suggestion: use constant for magic number",
            wantFile: "util.go",
            wantLine: 25,
            wantSev:  "suggestion",
            wantMsg:  "use constant for magic number",
        },
    }

    reviewer := NewCodexReviewer("")
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            issue := reviewer.parseLine(tt.line)
            if issue == nil {
                t.Fatal("parseLine() returned nil, want issue")
            }
            if issue.File != tt.wantFile {
                t.Errorf("File = %q, want %q", issue.File, tt.wantFile)
            }
            if issue.Line != tt.wantLine {
                t.Errorf("Line = %d, want %d", issue.Line, tt.wantLine)
            }
            if issue.Severity != tt.wantSev {
                t.Errorf("Severity = %q, want %q", issue.Severity, tt.wantSev)
            }
            if issue.Message != tt.wantMsg {
                t.Errorf("Message = %q, want %q", issue.Message, tt.wantMsg)
            }
        })
    }
}

func TestCodexReviewer_ParseLine_Invalid(t *testing.T) {
    lines := []string{
        "",
        "   ",
        "not an issue line",
        "file.go: missing line number",
        "Review complete.",
    }

    reviewer := NewCodexReviewer("")
    for _, line := range lines {
        t.Run(line, func(t *testing.T) {
            issue := reviewer.parseLine(line)
            if issue != nil {
                t.Errorf("parseLine(%q) = %+v, want nil", line, issue)
            }
        })
    }
}

func TestCodexReviewer_DefaultCommand(t *testing.T) {
    reviewer := NewCodexReviewer("")
    // Verify the default is used internally
    // This is tested indirectly via integration tests
    if reviewer.command != "" {
        t.Errorf("command = %q, want empty (for default)", reviewer.command)
    }
}

func TestCodexReviewer_CustomCommand(t *testing.T) {
    customPath := "/opt/codex/bin/codex"
    reviewer := NewCodexReviewer(customPath)
    if reviewer.command != customPath {
        t.Errorf("command = %q, want %q", reviewer.command, customPath)
    }
}
```

### Integration Tests

```go
// internal/provider/codex_reviewer_integration_test.go

//go:build integration

package provider

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

func TestCodexReviewer_Integration(t *testing.T) {
    // Skip if codex is not installed
    if _, err := exec.LookPath("codex"); err != nil {
        t.Skip("codex not found in PATH")
    }

    // Create temp git repo with changes
    tmpDir := t.TempDir()
    setupTestRepo(t, tmpDir)

    reviewer := NewCodexReviewer("")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := reviewer.Review(ctx, tmpDir, "main")
    if err != nil {
        t.Fatalf("Review() error = %v", err)
    }

    // Verify result structure
    if result.RawOutput == "" {
        t.Error("RawOutput is empty")
    }
    if result.Summary == "" {
        t.Error("Summary is empty")
    }
}

func TestCodexReviewer_ContextCancellation(t *testing.T) {
    if _, err := exec.LookPath("codex"); err != nil {
        t.Skip("codex not found in PATH")
    }

    tmpDir := t.TempDir()
    setupTestRepo(t, tmpDir)

    reviewer := NewCodexReviewer("")

    // Cancel immediately
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    _, err := reviewer.Review(ctx, tmpDir, "main")
    if err == nil {
        t.Error("Review() expected error for cancelled context")
    }
}

func setupTestRepo(t *testing.T, dir string) {
    t.Helper()

    commands := [][]string{
        {"git", "init"},
        {"git", "config", "user.email", "test@test.com"},
        {"git", "config", "user.name", "Test"},
    }

    for _, args := range commands {
        cmd := exec.Command(args[0], args[1:]...)
        cmd.Dir = dir
        if err := cmd.Run(); err != nil {
            t.Fatalf("setup command %v failed: %v", args, err)
        }
    }

    // Create initial file and commit
    testFile := filepath.Join(dir, "main.go")
    content := []byte("package main\n\nfunc main() {}\n")
    if err := os.WriteFile(testFile, content, 0644); err != nil {
        t.Fatalf("failed to write test file: %v", err)
    }

    cmd := exec.Command("git", "add", ".")
    cmd.Dir = dir
    cmd.Run()

    cmd = exec.Command("git", "commit", "-m", "initial")
    cmd.Dir = dir
    cmd.Run()
}
```

### Manual Testing

- [ ] `codex review --base main` executes correctly in test repository
- [ ] Exit code 0 = passed, exit code 1 = issues found, exit code 2+ = error
- [ ] Issues are correctly parsed from output (all four severity levels)
- [ ] Context cancellation stops the review process
- [ ] Custom command path works correctly
- [ ] Error messages are clear for missing codex binary
- [ ] Error messages include exit code for exit code 2+ failures

## Design Decisions

### Why Separate from CodexProvider?

The CodexProvider uses `codex exec --yolo` for task execution, while CodexReviewer uses `codex review --base`. These are fundamentally different operations:

| Aspect | CodexProvider | CodexReviewer |
|--------|--------------|---------------|
| Command | `exec --yolo` | `review --base` |
| Purpose | Execute tasks | Analyze code |
| Output | Streaming | Structured |
| Exit codes | Error only | Issue indicator |

Keeping them separate maintains single responsibility and allows independent evolution.

### Why Handle Exit Code 1 as Issues Found?

Many review and linting tools use exit code 1 to indicate issues were found, while exit code 0 means clean. This is a convention (similar to grep returning 1 for no matches). Treating exit code 1 as an error would incorrectly fail builds when the reviewer is working correctly.

### Why Combined Output?

Using `CombinedOutput()` instead of separate stdout/stderr:
- Simpler parsing (single stream)
- Issues may appear in either stream
- Order of messages preserved
- Sufficient for structured parsing

### Why Regex for Parsing?

The regex approach handles common issue formats flexibly:
- Works with various file path formats
- Optional column numbers
- Multiple severity levels
- Extensible for new formats

Alternative: JSON output from codex (if supported in future versions).

## Future Enhancements

1. **JSON Output Mode**: Support `--format json` flag when available in codex
2. **Suggestion Extraction**: Parse fix suggestions from expanded output
3. **Severity Filtering**: Allow filtering issues by severity level
4. **Caching**: Cache review results for unchanged file sets
5. **Diff Context**: Include code context around issues
6. **Parallel Reviews**: Support reviewing multiple directories concurrently

## References

- REVIEWER-INTERFACE spec (implements Reviewer interface)
- PROVIDER-INTERFACE spec (for ProviderType constants)
- [Codex CLI Documentation](https://github.com/openai/codex)
- `internal/provider/codex.go` (existing CodexProvider for comparison)
