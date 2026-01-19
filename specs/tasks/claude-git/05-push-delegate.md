---
task: 5
status: pending
backpressure: "go test ./internal/worker/... -run PushVia"
depends_on: [1, 2, 3]
---

# Push Delegation

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the pushViaClaudeCode worker method that invokes Claude to push the branch to remote, with retry, verification, escalation, and event emission.

## Dependencies

### External Specs (must be implemented)
- ESCALATION - provides Escalator interface and Escalation type
- EVENTS - provides Bus and BranchPushed event type
- WORKER - provides Worker struct with events bus

### Task Dependencies (within this unit)
- Task 1 (01-retry.md) - RetryWithBackoff, RetryConfig
- Task 2 (02-prompts.md) - BuildPushPrompt
- Task 3 (03-git-helpers.md) - branchExistsOnRemote

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── git_delegate.go      # MODIFY: Add pushViaClaudeCode method
└── git_delegate_test.go # MODIFY: Add push delegation tests
```

### Functions to Implement

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "fmt"

    "github.com/anthropics/choo/internal/escalate"
    "github.com/anthropics/choo/internal/events"
)

// pushViaClaudeCode invokes Claude to push the branch to remote
func (w *Worker) pushViaClaudeCode(ctx context.Context) error {
    prompt := BuildPushPrompt(w.branch)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify branch exists on remote
        exists, err := w.branchExistsOnRemote(ctx, w.branch)
        if err != nil {
            return err
        }
        if !exists {
            return fmt.Errorf("branch not found on remote after push")
        }
        return nil
    })

    if !result.Success {
        if w.escalator != nil {
            w.escalator.Escalate(ctx, escalate.Escalation{
                Severity: escalate.SeverityBlocking,
                Unit:     w.unit.ID,
                Title:    "Failed to push branch",
                Message:  fmt.Sprintf("Claude could not push after %d attempts", result.Attempts),
                Context: map[string]string{
                    "branch": w.branch,
                    "error":  result.LastErr.Error(),
                },
            })
        }
        return result.LastErr
    }

    // Emit BranchPushed event on success
    if w.events != nil {
        evt := events.NewEvent(events.BranchPushed, w.unit.ID).
            WithPayload(map[string]interface{}{"branch": w.branch})
        w.events.Emit(evt)
    }

    return nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run PushVia
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestPushViaClaudeCode_Success | returns nil, branch on remote |
| TestPushViaClaudeCode_EmitsEvent | BranchPushed event emitted with branch name |
| TestPushViaClaudeCode_RetriesOnFailure | retries when push fails |
| TestPushViaClaudeCode_EscalatesOnExhaustion | calls escalator after max retries |
| TestPushViaClaudeCode_VerifiesBranch | fails if branch not on remote after push |

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
    "github.com/anthropics/choo/internal/events"
)

// mockEventBus records emitted events for testing
type mockEventBus struct {
    emitted []events.Event
}

func (m *mockEventBus) Emit(e events.Event) {
    m.emitted = append(m.emitted, e)
}

func (m *mockEventBus) Subscribe(handler events.Handler) {}
func (m *mockEventBus) Close() error                     { return nil }

func TestPushViaClaudeCode_Success(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    // Setup remote for testing (use local bare repo as "remote")
    remoteDir, err := os.MkdirTemp("", "git-remote-*")
    if err != nil {
        t.Fatalf("failed to create remote dir: %v", err)
    }
    defer os.RemoveAll(remoteDir)

    // Initialize bare remote
    cmd := exec.Command("git", "init", "--bare")
    cmd.Dir = remoteDir
    if err := cmd.Run(); err != nil {
        t.Fatalf("failed to init remote: %v", err)
    }

    // Add remote to test repo
    cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
    cmd.Dir = dir
    if err := cmd.Run(); err != nil {
        t.Fatalf("failed to add remote: %v", err)
    }

    eventBus := &mockEventBus{}
    w := &Worker{
        worktreePath: dir,
        branch:       "test-branch",
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    &mockEscalator{},
        events:       eventBus,
    }

    // Create branch
    cmd = exec.Command("git", "checkout", "-b", "test-branch")
    cmd.Dir = dir
    if err := cmd.Run(); err != nil {
        t.Fatalf("failed to create branch: %v", err)
    }

    // Mock invokeClaude to actually push
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        cmd := exec.Command("git", "push", "-u", "origin", "test-branch")
        cmd.Dir = dir
        return cmd.Run()
    }

    err = w.pushViaClaudeCode(context.Background())
    if err != nil {
        t.Errorf("expected success, got error: %v", err)
    }
}

func TestPushViaClaudeCode_EmitsEvent(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    // Setup remote
    remoteDir, _ := os.MkdirTemp("", "git-remote-*")
    defer os.RemoveAll(remoteDir)

    exec.Command("git", "init", "--bare").Run()
    cmd := exec.Command("git", "init", "--bare")
    cmd.Dir = remoteDir
    cmd.Run()

    cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
    cmd.Dir = dir
    cmd.Run()

    cmd = exec.Command("git", "checkout", "-b", "feature-branch")
    cmd.Dir = dir
    cmd.Run()

    eventBus := &mockEventBus{}
    w := &Worker{
        worktreePath: dir,
        branch:       "feature-branch",
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    &mockEscalator{},
        events:       eventBus,
    }

    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        cmd := exec.Command("git", "push", "-u", "origin", "feature-branch")
        cmd.Dir = dir
        return cmd.Run()
    }

    w.pushViaClaudeCode(context.Background())

    if len(eventBus.emitted) == 0 {
        t.Error("expected BranchPushed event to be emitted")
    }

    if len(eventBus.emitted) > 0 {
        e := eventBus.emitted[0]
        if e.Type != events.BranchPushed {
            t.Errorf("expected BranchPushed event, got %v", e.Type)
        }
        payload, ok := e.Payload.(map[string]interface{})
        if !ok {
            t.Error("expected payload to be map")
        }
        if payload["branch"] != "feature-branch" {
            t.Errorf("expected branch in payload, got %v", payload)
        }
    }
}

func TestPushViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        branch:       "test-branch",
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    esc,
    }

    // Mock invokeClaude to always fail
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        return errors.New("network error")
    }

    err := w.pushViaClaudeCode(context.Background())
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
        if e.Title != "Failed to push branch" {
            t.Errorf("unexpected title: %s", e.Title)
        }
        if e.Context["branch"] != "test-branch" {
            t.Errorf("expected branch in context: %v", e.Context)
        }
    }
}

func TestPushViaClaudeCode_VerifiesBranch(t *testing.T) {
    dir, cleanup := setupTestGitRepo(t)
    defer cleanup()

    esc := &mockEscalator{}
    w := &Worker{
        worktreePath: dir,
        branch:       "test-branch",
        unit:         &discovery.Unit{ID: "test-unit"},
        escalator:    esc,
    }

    // Mock invokeClaude to succeed but NOT actually push
    w.invokeClaudeForTask = func(ctx context.Context, prompt TaskPrompt) error {
        return nil // Success but no push
    }

    err := w.pushViaClaudeCode(context.Background())
    if err == nil {
        t.Error("expected error when branch not on remote")
    }
}
```

## NOT In Scope

- Commit operations (handled in task 4)
- PR creation (handled in task 6)
- Force push handling
