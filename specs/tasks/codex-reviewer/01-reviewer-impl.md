---
task: 1
status: pending
backpressure: "go build ./internal/provider/..."
depends_on: []
---

# CodexReviewer Implementation

**Parent spec**: `/specs/CODEX-REVIEWER.md`
**Task**: #1 of 2 in implementation plan

## Objective

Implement the CodexReviewer struct that satisfies the Reviewer interface by invoking `codex review --base <baseBranch>` and handling exit codes appropriately.

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `Reviewer` interface, `ReviewResult`, `ReviewIssue` types

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `os/exec` for command execution
- Standard library only

## Deliverables

### Files to Create/Modify

```
internal/provider/
└── codex_reviewer.go    # CREATE: CodexReviewer implementation
```

### Types to Implement

```go
// internal/provider/codex_reviewer.go

package provider

// CodexReviewer implements Reviewer using Codex CLI.
type CodexReviewer struct {
    // command is the path to codex CLI, empty for system PATH
    command string
}
```

### Functions to Implement

```go
// NewCodexReviewer creates a CodexReviewer with optional command override.
// If command is empty, defaults to "codex" resolved via PATH.
func NewCodexReviewer(command string) *CodexReviewer {
    return &CodexReviewer{command: command}
}

// Name returns ProviderCodex to identify this reviewer.
func (r *CodexReviewer) Name() ProviderType {
    return ProviderCodex
}

// Review executes codex review and returns structured results.
// The review compares changes in workdir against baseBranch.
//
// Exit code handling:
// - 0: No issues found (Passed = true)
// - 1: Issues found (Passed = false, parse output for issues)
// - 2+: Execution error (return error)
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
```

## Implementation Notes

### Exit Code Semantics

The codex review command follows linter conventions:
- Exit 0: Clean review, no issues
- Exit 1: Issues found (this is NOT an error condition)
- Exit 2+: Command failure, configuration error, etc.

This matches tools like `grep` (exit 1 = no match) and `eslint` (exit 1 = lint issues).

### Context Propagation

Using `exec.CommandContext` ensures the subprocess is killed when context is cancelled. This is critical for graceful shutdown.

### Command Override

The `command` field allows users to specify a custom path to the codex binary:
- Empty string: Use "codex" from PATH
- Absolute path: Use that specific binary

### parseOutput Stub

This task creates a stub `parseOutput` method that returns basic results. The full parsing logic is implemented in Task #2.

```go
// parseOutput converts codex output into structured ReviewResult.
// Full parsing logic implemented in Task #2.
func (r *CodexReviewer) parseOutput(output string, exitCode int) (*ReviewResult, error) {
    result := &ReviewResult{
        RawOutput: output,
        Passed:    exitCode == 0,
        Issues:    []ReviewIssue{},
    }

    if exitCode == 0 {
        result.Summary = "No issues found"
    } else {
        result.Summary = "Review completed with findings"
    }

    return result, nil
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/provider/...
```

### Must Pass

| Check | Assertion |
|-------|-----------|
| Compilation | `go build ./internal/provider/...` succeeds |
| Interface satisfaction | `var _ Reviewer = (*CodexReviewer)(nil)` compiles |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## NOT In Scope

- Full output parsing (Task #2)
- Integration tests with real codex binary
- JSON output mode support
- Timeout configuration
