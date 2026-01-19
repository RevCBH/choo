---
task: 5
status: pending
backpressure: "go test ./internal/escalate/... -run Multi"
depends_on: [1]
---

# Multi Escalator

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the Multi escalator that fans out escalations to multiple backends concurrently, continuing even if some fail.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Core types and interface

### Package Dependencies
- `context` (standard library)
- `sync` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/escalate/
├── multi.go       # CREATE: Multi backend implementation
└── multi_test.go  # CREATE: Multi tests
```

### Types to Implement

```go
// internal/escalate/multi.go

package escalate

import (
    "context"
    "sync"
)

// Multi wraps multiple escalators and fans out to all of them
type Multi struct {
    escalators []Escalator
}

// NewMulti creates a Multi escalator that sends to all provided backends
func NewMulti(escalators ...Escalator) *Multi {
    return &Multi{escalators: escalators}
}

// Escalate sends the escalation to all backends concurrently.
// Returns the first error encountered, but continues sending to all backends.
func (m *Multi) Escalate(ctx context.Context, e Escalation) error {
    if len(m.escalators) == 0 {
        return nil
    }

    var (
        wg       sync.WaitGroup
        mu       sync.Mutex
        firstErr error
    )

    for _, esc := range m.escalators {
        wg.Add(1)
        go func(esc Escalator) {
            defer wg.Done()
            if err := esc.Escalate(ctx, e); err != nil {
                mu.Lock()
                if firstErr == nil {
                    firstErr = err
                }
                mu.Unlock()
            }
        }(esc)
    }

    wg.Wait()
    return firstErr
}

// Name returns "multi"
func (m *Multi) Name() string {
    return "multi"
}
```

### Tests to Implement

```go
// internal/escalate/multi_test.go

package escalate

import (
    "context"
    "errors"
    "sync/atomic"
    "testing"
)

type mockEscalator struct {
    name  string
    err   error
    calls int32
}

func (m *mockEscalator) Escalate(ctx context.Context, e Escalation) error {
    atomic.AddInt32(&m.calls, 1)
    return m.err
}

func (m *mockEscalator) Name() string {
    return m.name
}

func TestMulti_Escalate(t *testing.T) {
    mock1 := &mockEscalator{name: "mock1"}
    mock2 := &mockEscalator{name: "mock2"}
    mock3 := &mockEscalator{name: "mock3"}

    multi := NewMulti(mock1, mock2, mock3)
    err := multi.Escalate(context.Background(), Escalation{
        Severity: SeverityInfo,
        Unit:     "test",
        Title:    "Test",
        Message:  "Test message",
    })

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    if mock1.calls != 1 || mock2.calls != 1 || mock3.calls != 1 {
        t.Error("expected all escalators to be called once")
    }
}

func TestMulti_EscalateContinuesOnError(t *testing.T) {
    mock1 := &mockEscalator{name: "mock1"}
    mock2 := &mockEscalator{name: "mock2", err: errors.New("failed")}
    mock3 := &mockEscalator{name: "mock3"}

    multi := NewMulti(mock1, mock2, mock3)
    err := multi.Escalate(context.Background(), Escalation{
        Severity: SeverityInfo,
        Unit:     "test",
        Title:    "Test",
        Message:  "Test message",
    })

    // Should return an error
    if err == nil {
        t.Error("expected error from failing escalator")
    }

    // But all escalators should still be called
    if mock1.calls != 1 || mock2.calls != 1 || mock3.calls != 1 {
        t.Error("expected all escalators to be called despite errors")
    }
}

func TestMulti_Empty(t *testing.T) {
    multi := NewMulti()
    err := multi.Escalate(context.Background(), Escalation{
        Severity: SeverityInfo,
        Unit:     "test",
        Title:    "Test",
        Message:  "Test message",
    })

    if err != nil {
        t.Errorf("unexpected error for empty multi: %v", err)
    }
}

func TestMulti_Name(t *testing.T) {
    multi := NewMulti()
    if multi.Name() != "multi" {
        t.Errorf("expected 'multi', got %q", multi.Name())
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/escalate/... -run Multi
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestMulti_Escalate | All backends called once, no error |
| TestMulti_EscalateContinuesOnError | Returns error, all backends still called |
| TestMulti_Empty | No error with zero backends |
| TestMulti_Name | Returns "multi" |

## NOT In Scope
- Terminal escalator
- Slack escalator
- Webhook escalator
- Factory function
