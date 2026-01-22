---
task: 1
status: complete
backpressure: "go test ./internal/provider/... -run TestClaudeReviewer -v"
depends_on: []
---

# ClaudeReviewer Implementation

**Parent spec**: `/specs/CLAUDE-REVIEWER.md`
**Task**: #1 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the ClaudeReviewer struct that satisfies the Reviewer interface, including the constructor, Name() method, Review() method, and getDiff() helper.

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `Reviewer` interface, `ReviewResult`, `ReviewIssue`, `ProviderType`, `ProviderClaude`

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `context` (standard library)
- `os/exec` (standard library)
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/provider/
└── claude_reviewer.go    # CREATE: ClaudeReviewer implementation
```

### Types to Implement

```go
// internal/provider/claude_reviewer.go

package provider

import (
    "context"
    "fmt"
    "os/exec"
)

// ClaudeReviewer implements Reviewer using Claude with diff-based prompts.
// It retrieves the git diff, builds a prompt requesting JSON output,
// and parses the structured response into ReviewIssue types.
type ClaudeReviewer struct {
    command string // Path to claude CLI executable
}
```

### Functions to Implement

```go
// NewClaudeReviewer creates a ClaudeReviewer with optional command override.
// If command is empty, defaults to "claude" (resolved via PATH).
func NewClaudeReviewer(command string) *ClaudeReviewer {
    if command == "" {
        command = "claude"
    }
    return &ClaudeReviewer{command: command}
}

// Name returns ProviderClaude to identify this reviewer.
func (r *ClaudeReviewer) Name() ProviderType {
    return ProviderClaude
}

// Review performs code review by getting the diff, invoking Claude,
// and parsing the structured JSON response.
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

// getDiff retrieves the git diff between baseBranch and HEAD.
func (r *ClaudeReviewer) getDiff(ctx context.Context, workdir, baseBranch string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "diff", baseBranch+"...HEAD")
    cmd.Dir = workdir
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return string(output), nil
}
```

## Implementation Notes

- The Review method calls `BuildClaudeReviewPrompt` (Task #2) and `parseOutput` (Task #3)
- For this task, create stub implementations of these functions that will be replaced:
  - `BuildClaudeReviewPrompt(diff string) string` - return empty string
  - `parseOutput(output string) (*ReviewResult, error)` - return passed result
- The Claude CLI flags are critical for non-interactive operation:
  - `--dangerously-skip-permissions` bypasses permission prompts
  - `--print` outputs to stdout instead of interactive TUI
  - `-p` provides the prompt as an argument
- Context cancellation is respected via `exec.CommandContext`

## Backpressure

### Validation Command

```bash
go test ./internal/provider/... -run TestClaudeReviewer -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewClaudeReviewer_DefaultCommand` | Default command is "claude" when empty string provided |
| `TestNewClaudeReviewer_CustomCommand` | Custom command path is preserved |
| `TestClaudeReviewer_Name` | Returns `ProviderClaude` |
| `TestClaudeReviewer_Review_EmptyDiff` | Returns passed=true with "No changes to review" summary |

### CI Compatibility

- [x] No external API keys required (uses mock/stub for Claude CLI)
- [x] No network access required for unit tests
- [x] Runs in <60 seconds

## NOT In Scope

- BuildClaudeReviewPrompt implementation (Task #2)
- JSON extraction and parseOutput implementation (Task #3)
- Integration tests requiring actual Claude CLI
