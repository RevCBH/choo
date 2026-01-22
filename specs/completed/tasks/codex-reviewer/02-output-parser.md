---
task: 2
status: complete
backpressure: "go test ./internal/provider/... -run TestCodexReviewer"
depends_on: [1]
---

# Codex Output Parser

**Parent spec**: `/specs/CODEX-REVIEWER.md`
**Task**: #2 of 2 in implementation plan

## Objective

Implement regex-based parsing of codex review output into structured ReviewIssue objects, supporting all four severity levels (error, warning, suggestion, info).

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `ReviewIssue` type with severity constants

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `CodexReviewer` struct, `parseOutput` stub)

### Package Dependencies
- `regexp` for pattern matching
- `strconv` for line number parsing
- `strings` for text processing

## Deliverables

### Files to Create/Modify

```
internal/provider/
├── codex_reviewer.go       # MODIFY: Add full parseOutput and parseLine
└── codex_reviewer_test.go  # CREATE: Unit tests for parsing
```

### Functions to Implement

```go
// internal/provider/codex_reviewer.go

// issuePattern matches common issue output formats:
// file.go:10: error: message
// file.go:10:5: warning: message
// path/to/file.go:42: suggestion: use constants
// test.go:1: info: consider renaming
var issuePattern = regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?:\s*(\w+):\s*(.+)$`)

// parseOutput converts codex output into structured ReviewResult.
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

### Output Format

The parser handles the standard linter output format:
```
path/to/file.go:42: error: undefined variable x
path/to/file.go:42:15: warning: unused import
path/to/file.go:10: info: consider renaming function
path/to/file.go:5: suggestion: use constants for magic numbers
```

### Regex Pattern Breakdown

Pattern: `^([^:]+):(\d+)(?::\d+)?:\s*(\w+):\s*(.+)$`

| Group | Captures | Example |
|-------|----------|---------|
| 1 | File path (anything before first colon) | `path/to/file.go` |
| 2 | Line number (required digits) | `42` |
| n/a | Optional column number (ignored) | `:15` |
| 3 | Severity word | `error`, `warning`, `suggestion`, `info` |
| 4 | Message text (rest of line) | `undefined variable x` |

### Severity Normalization

Severity is converted to lowercase to match the `ValidSeverities` constants from REVIEWER-INTERFACE:
- `SeverityError = "error"`
- `SeverityWarning = "warning"`
- `SeveritySuggestion = "suggestion"`
- `SeverityInfo = "info"`

### Non-Issue Lines

Lines that don't match the pattern are silently skipped:
- Empty lines
- Status messages ("Review complete.")
- Headers or decorative output

## Backpressure

### Validation Command

```bash
go test ./internal/provider/... -run TestCodexReviewer
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCodexReviewer_Name` | `reviewer.Name() == ProviderCodex` |
| `TestCodexReviewer_ParseOutput_NoIssues` | Empty issues, Passed=true, Summary="No issues found" |
| `TestCodexReviewer_ParseOutput_WithIssues` | Issues parsed correctly, Passed=false |
| `TestCodexReviewer_ParseLine_Error` | Parses error severity correctly |
| `TestCodexReviewer_ParseLine_Warning` | Parses warning severity correctly |
| `TestCodexReviewer_ParseLine_Suggestion` | Parses suggestion severity correctly |
| `TestCodexReviewer_ParseLine_Info` | Parses info severity correctly |
| `TestCodexReviewer_ParseLine_WithColumn` | Handles optional column number |
| `TestCodexReviewer_ParseLine_Invalid` | Returns nil for non-issue lines |
| `TestCodexReviewer_DefaultCommand` | Empty command field for PATH resolution |
| `TestCodexReviewer_CustomCommand` | Custom command path stored correctly |

### Test Implementation

```go
// internal/provider/codex_reviewer_test.go

package provider

import (
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
}

func TestCodexReviewer_ParseLine_AllSeverities(t *testing.T) {
    tests := []struct {
        name     string
        line     string
        wantFile string
        wantLine int
        wantSev  string
        wantMsg  string
    }{
        {
            name:     "error severity",
            line:     "file.go:42: error: message here",
            wantFile: "file.go",
            wantLine: 42,
            wantSev:  "error",
            wantMsg:  "message here",
        },
        {
            name:     "warning with column",
            line:     "path/to/file.go:10:5: warning: unused var",
            wantFile: "path/to/file.go",
            wantLine: 10,
            wantSev:  "warning",
            wantMsg:  "unused var",
        },
        {
            name:     "suggestion severity",
            line:     "util.go:25: suggestion: use constant for magic number",
            wantFile: "util.go",
            wantLine: 25,
            wantSev:  "suggestion",
            wantMsg:  "use constant for magic number",
        },
        {
            name:     "info severity",
            line:     "test.go:1: info: consider renaming",
            wantFile: "test.go",
            wantLine: 1,
            wantSev:  "info",
            wantMsg:  "consider renaming",
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

func TestCodexReviewer_CommandConfig(t *testing.T) {
    t.Run("default command", func(t *testing.T) {
        reviewer := NewCodexReviewer("")
        if reviewer.command != "" {
            t.Errorf("command = %q, want empty", reviewer.command)
        }
    })

    t.Run("custom command", func(t *testing.T) {
        customPath := "/opt/codex/bin/codex"
        reviewer := NewCodexReviewer(customPath)
        if reviewer.command != customPath {
            t.Errorf("command = %q, want %q", reviewer.command, customPath)
        }
    })
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (unit tests only)
- [x] No codex binary required (tests use parseOutput/parseLine directly)
- [x] Runs in <60 seconds

## NOT In Scope

- Integration tests with real codex binary
- JSON output parsing mode
- Suggestion field extraction (requires expanded output format)
- ANSI color stripping (assume clean output)
