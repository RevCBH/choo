---
task: 6
status: pending
backpressure: "go test ./internal/worker/... -run PRVia"
depends_on: [1, 2]
---

# PR Creation Delegation

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement createPRViaClaudeCode worker method that invokes Claude to create a PR and extract the PR URL from output, with retry and escalation.

## Dependencies

### External Specs (must be implemented)
- ESCALATION - provides Escalator interface and Escalation type
- WORKER - provides Worker struct with config.TargetBranch

### Task Dependencies (within this unit)
- Task 1 (01-retry.md) - RetryWithBackoff, RetryConfig
- Task 2 (02-prompts.md) - BuildPRPrompt

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── git_delegate.go      # MODIFY: Add createPRViaClaudeCode, extractPRURL, invokeClaudeWithOutput
└── git_delegate_test.go # MODIFY: Add PR delegation tests
```

### Functions to Implement

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"

    "github.com/anthropics/choo/internal/escalate"
)

// prURLPattern matches GitHub PR URLs
var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

// extractPRURL extracts a GitHub PR URL from Claude's output
func extractPRURL(output string) string {
    match := prURLPattern.FindString(output)
    return match
}

// createPRViaClaudeCode invokes Claude to create the PR
func (w *Worker) createPRViaClaudeCode(ctx context.Context) (string, error) {
    prompt := BuildPRPrompt(w.branch, w.config.TargetBranch, w.unit.Title)

    var prURL string

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        output, err := w.invokeClaudeWithOutput(ctx, prompt)
        if err != nil {
            return err
        }

        // Extract PR URL from output
        url := extractPRURL(output)
        if url == "" {
            return fmt.Errorf("could not find PR URL in claude output")
        }

        prURL = url
        return nil
    })

    if !result.Success {
        if w.escalator != nil {
            w.escalator.Escalate(ctx, escalate.Escalation{
                Severity: escalate.SeverityBlocking,
                Unit:     w.unit.ID,
                Title:    "Failed to create PR",
                Message:  fmt.Sprintf("Claude could not create PR after %d attempts", result.Attempts),
                Context: map[string]string{
                    "branch": w.branch,
                    "target": w.config.TargetBranch,
                    "error":  result.LastErr.Error(),
                },
            })
        }
        return "", result.LastErr
    }

    return prURL, nil
}

// invokeClaudeWithOutput invokes Claude CLI and captures stdout
func (w *Worker) invokeClaudeWithOutput(ctx context.Context, prompt string) (string, error) {
    cmd := exec.CommandContext(ctx, "claude",
        "--dangerously-skip-permissions",
        "-p", prompt,
    )
    cmd.Dir = w.worktreePath

    out, err := cmd.Output()
    if err != nil {
        return "", err
    }

    return string(out), nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run PRVia
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestExtractPRURL_ValidURL | extracts correct URL from output |
| TestExtractPRURL_NoURL | returns empty string when no URL |
| TestExtractPRURL_MultipleURLs | returns first URL found |
| TestCreatePRViaClaudeCode_Success | returns PR URL |
| TestCreatePRViaClaudeCode_RetriesOnFailure | retries when Claude fails |
| TestCreatePRViaClaudeCode_EscalatesOnExhaustion | calls escalator after max retries |
| TestCreatePRViaClaudeCode_FailsWithoutURL | fails if no URL in output |

### Test Implementations

```go
// internal/worker/git_delegate_test.go (additions)

package worker

import (
    "context"
    "errors"
    "strings"
    "testing"

    "github.com/anthropics/choo/internal/discovery"
    "github.com/anthropics/choo/internal/escalate"
)

func TestExtractPRURL_ValidURL(t *testing.T) {
    output := `Creating pull request...
https://github.com/anthropics/choo/pull/42
Done!`

    url := extractPRURL(output)
    if url != "https://github.com/anthropics/choo/pull/42" {
        t.Errorf("expected PR URL, got %q", url)
    }
}

func TestExtractPRURL_NoURL(t *testing.T) {
    output := "Error: could not create PR"

    url := extractPRURL(output)
    if url != "" {
        t.Errorf("expected empty string, got %q", url)
    }
}

func TestExtractPRURL_MultipleURLs(t *testing.T) {
    // Should return the first match
    output := `https://github.com/anthropics/choo/pull/41
https://github.com/anthropics/choo/pull/42`

    url := extractPRURL(output)
    if url != "https://github.com/anthropics/choo/pull/41" {
        t.Errorf("expected first PR URL, got %q", url)
    }
}

func TestExtractPRURL_URLInMiddleOfText(t *testing.T) {
    output := `Pull request created successfully at https://github.com/owner/repo/pull/123 - please review`

    url := extractPRURL(output)
    if url != "https://github.com/owner/repo/pull/123" {
        t.Errorf("expected PR URL from middle of text, got %q", url)
    }
}

func TestExtractPRURL_DifferentOwnerRepo(t *testing.T) {
    output := "https://github.com/my-org/my-project/pull/999"

    url := extractPRURL(output)
    if url != "https://github.com/my-org/my-project/pull/999" {
        t.Errorf("expected PR URL with different org, got %q", url)
    }
}

func TestCreatePRViaClaudeCode_Success(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    w := &Worker{
        worktreePath: dir,
        branch:       "feature/test",
        unit:         &discovery.Unit{ID: "test-unit", Title: "Test Unit"},
        config:       WorkerConfig{TargetBranch: "main"},
        escalator:    &mockEscalator{},
    }

    // Mock invokeClaudeWithOutput to return PR URL
    w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
        return "Created PR: https://github.com/test/repo/pull/42", nil
    }

    url, err := w.createPRViaClaudeCode(context.Background())
    if err != nil {
        t.Errorf("expected success, got error: %v", err)
    }
    if url != "https://github.com/test/repo/pull/42" {
        t.Errorf("expected PR URL, got %q", url)
    }
}

func TestCreatePRViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        branch:       "feature/test",
        unit:         &discovery.Unit{ID: "test-unit", Title: "Test Unit"},
        config:       WorkerConfig{TargetBranch: "main"},
        escalator:    esc,
    }

    // Mock invokeClaudeWithOutput to always fail
    w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
        return "", errors.New("gh CLI error")
    }

    url, err := w.createPRViaClaudeCode(context.Background())
    if err == nil {
        t.Error("expected error after exhausting retries")
    }
    if url != "" {
        t.Errorf("expected empty URL on error, got %q", url)
    }

    if len(esc.escalations) == 0 {
        t.Error("expected escalation to be called")
    }

    if len(esc.escalations) > 0 {
        e := esc.escalations[0]
        if e.Severity != escalate.SeverityBlocking {
            t.Errorf("expected SeverityBlocking, got %v", e.Severity)
        }
        if e.Title != "Failed to create PR" {
            t.Errorf("unexpected title: %s", e.Title)
        }
        if e.Context["branch"] != "feature/test" {
            t.Errorf("expected branch in context: %v", e.Context)
        }
        if e.Context["target"] != "main" {
            t.Errorf("expected target in context: %v", e.Context)
        }
    }
}

func TestCreatePRViaClaudeCode_FailsWithoutURL(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        branch:       "feature/test",
        unit:         &discovery.Unit{ID: "test-unit", Title: "Test Unit"},
        config:       WorkerConfig{TargetBranch: "main"},
        escalator:    esc,
    }

    // Mock invokeClaudeWithOutput to return no URL
    w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
        return "PR creation completed but no URL printed", nil
    }

    url, err := w.createPRViaClaudeCode(context.Background())
    if err == nil {
        t.Error("expected error when no URL in output")
    }
    if url != "" {
        t.Errorf("expected empty URL, got %q", url)
    }

    if !strings.Contains(err.Error(), "could not find PR URL") {
        t.Errorf("error should mention missing URL: %v", err)
    }
}

func TestCreatePRViaClaudeCode_RetriesOnFailure(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    callCount := 0
    w := &Worker{
        worktreePath: dir,
        branch:       "feature/test",
        unit:         &discovery.Unit{ID: "test-unit", Title: "Test Unit"},
        config:       WorkerConfig{TargetBranch: "main"},
        escalator:    &mockEscalator{},
    }

    // Mock to fail twice then succeed
    w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
        callCount++
        if callCount < 3 {
            return "", errors.New("transient error")
        }
        return "https://github.com/test/repo/pull/42", nil
    }

    url, err := w.createPRViaClaudeCode(context.Background())
    if err != nil {
        t.Errorf("expected success after retries, got error: %v", err)
    }
    if url != "https://github.com/test/repo/pull/42" {
        t.Errorf("expected PR URL, got %q", url)
    }
    if callCount != 3 {
        t.Errorf("expected 3 calls, got %d", callCount)
    }
}
```

## NOT In Scope

- Commit operations (handled in task 4)
- Push operations (handled in task 5)
- PR merge handling (uses GitHub API directly)
- PR description templates (future enhancement)
