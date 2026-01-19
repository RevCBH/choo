---
task: 4
status: pending
backpressure: "go test ./internal/worker/... -run CommitVia"
depends_on: [1, 2, 3]
---

# Commit Delegation

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the commitViaClaudeCode worker method that invokes Claude to stage and commit changes, with retry and escalation on failure.

## Dependencies

### External Specs (must be implemented)
- ESCALATION - provides Escalator interface and Escalation type
- WORKER - provides Worker struct

### Task Dependencies (within this unit)
- Task 1 (01-retry.md) - RetryWithBackoff, RetryConfig
- Task 2 (02-prompts.md) - BuildCommitPrompt
- Task 3 (03-git-helpers.md) - getHeadRef, hasNewCommit, getChangedFiles

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── git_delegate.go      # MODIFY: Add commitViaClaudeCode method
├── git_delegate_test.go # MODIFY: Add commit delegation tests
└── worker.go            # MODIFY: Add escalator field to Worker
```

### Worker Struct Addition

```go
// internal/worker/worker.go (additions)

import "github.com/anthropics/choo/internal/escalate"

// Add to Worker struct
type Worker struct {
    // ... existing fields ...
    escalator escalate.Escalator
}

// Add to WorkerDeps
type WorkerDeps struct {
    // ... existing fields ...
    Escalator escalate.Escalator
}

// Update NewWorker to accept escalator
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
    return &Worker{
        // ... existing assignments ...
        escalator: deps.Escalator,
    }
}
```

### Functions to Implement

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "fmt"

    "github.com/anthropics/choo/internal/escalate"
)

// commitViaClaudeCode invokes Claude to stage and commit changes
func (w *Worker) commitViaClaudeCode(ctx context.Context, taskTitle string) error {
    // Get the HEAD ref before invoking Claude
    headBefore, err := w.getHeadRef(ctx)
    if err != nil {
        return fmt.Errorf("failed to get HEAD ref: %w", err)
    }

    files, _ := w.getChangedFiles(ctx)
    prompt := BuildCommitPrompt(taskTitle, files)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify commit was created
        hasCommit, err := w.hasNewCommit(ctx, headBefore)
        if err != nil {
            return err
        }
        if !hasCommit {
            return fmt.Errorf("claude did not create a commit")
        }
        return nil
    })

    if !result.Success {
        if w.escalator != nil {
            w.escalator.Escalate(ctx, escalate.Escalation{
                Severity: escalate.SeverityBlocking,
                Unit:     w.unit.ID,
                Title:    "Failed to commit changes",
                Message:  fmt.Sprintf("Claude could not commit after %d attempts", result.Attempts),
                Context: map[string]string{
                    "task":  taskTitle,
                    "error": result.LastErr.Error(),
                },
            })
        }
        return result.LastErr
    }

    return nil
}

// invokeClaude invokes Claude CLI with the given prompt (no output capture)
func (w *Worker) invokeClaude(ctx context.Context, prompt string) error {
    taskPrompt := TaskPrompt{Content: prompt}
    return w.invokeClaudeForTask(ctx, taskPrompt)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run CommitVia
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestCommitViaClaudeCode_Success | returns nil, commit exists |
| TestCommitViaClaudeCode_RetriesOnFailure | retries when Claude fails |
| TestCommitViaClaudeCode_EscalatesOnExhaustion | calls escalator after max retries |
| TestCommitViaClaudeCode_VerifiesCommit | fails if no new commit after Claude |

### Test Implementations

```go
// internal/worker/git_delegate_test.go (additions)

package worker

import (
    "context"
    "errors"
    "testing"

    "github.com/anthropics/choo/internal/discovery"
    "github.com/anthropics/choo/internal/escalate"
)

// mockEscalator records escalations for testing
type mockEscalator struct {
    escalations []escalate.Escalation
}

func (m *mockEscalator) Escalate(ctx context.Context, e escalate.Escalation) error {
    m.escalations = append(m.escalations, e)
    return nil
}

func (m *mockEscalator) Name() string {
    return "mock"
}

// mockClaudeInvoker allows controlling Claude invocation behavior
type mockClaudeInvoker struct {
    invokeErr    error
    invokeCount  int
    failUntil    int
    commitAfter  int
    headRef      string
    changedFiles []string
}

func TestCommitViaClaudeCode_Success(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    // Create a file to commit
    testFile := filepath.Join(dir, "new_feature.go")
    if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
        t.Fatalf("failed to write test file: %v", err)
    }

    w := &Worker{
        worktreePath: dir,
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    &mockEscalator{},
    }

    // Mock invokeClaude to actually create a commit
    originalInvoke := w.invokeClaudeForTask
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        cmd := exec.Command("git", "add", "-A")
        cmd.Dir = dir
        if err := cmd.Run(); err != nil {
            return err
        }
        cmd = exec.Command("git", "commit", "-m", "feat: add new feature")
        cmd.Dir = dir
        return cmd.Run()
    }
    defer func() { w.invokeClaudeForTask = originalInvoke }()

    err := w.commitViaClaudeCode(context.Background(), "Add new feature")
    if err != nil {
        t.Errorf("expected success, got error: %v", err)
    }
}

func TestCommitViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    esc,
    }

    // Mock invokeClaude to always fail
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        return errors.New("claude unavailable")
    }

    err := w.commitViaClaudeCode(context.Background(), "Test task")
    if err == nil {
        t.Error("expected error after exhausting retries")
    }

    if len(esc.escalations) == 0 {
        t.Error("expected escalation to be called")
    }

    if len(esc.escalations) > 0 {
        e := esc.escalations[0]
        if e.Severity != escalate.SeverityBlocking {
            t.Errorf("expected SeverityBlocking, got %v", e.Severity)
        }
        if e.Title != "Failed to commit changes" {
            t.Errorf("unexpected title: %s", e.Title)
        }
    }
}

func TestCommitViaClaudeCode_VerifiesCommit(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    esc,
    }

    // Mock invokeClaude to succeed but NOT create a commit
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        return nil // Success but no commit
    }

    err := w.commitViaClaudeCode(context.Background(), "Test task")
    if err == nil {
        t.Error("expected error when no commit created")
    }

    if !strings.Contains(err.Error(), "did not create a commit") {
        t.Errorf("error should mention missing commit: %v", err)
    }
}
```

## NOT In Scope

- Push operations (handled in task 5)
- PR creation (handled in task 6)
- Event emission for commit operations (can be added later)
