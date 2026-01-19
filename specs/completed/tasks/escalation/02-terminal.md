---
task: 2
status: complete
backpressure: "go test ./internal/escalate/... -run Terminal"
depends_on: [1]
---

# Terminal Escalator

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement the Terminal escalator that writes formatted messages to stderr with emoji severity indicators.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Core types and interface

### Package Dependencies
- `context` (standard library)
- `fmt` (standard library)
- `os` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/escalate/
├── terminal.go       # CREATE: Terminal backend implementation
└── terminal_test.go  # CREATE: Terminal tests
```

### Types to Implement

```go
// internal/escalate/terminal.go

package escalate

import (
    "context"
    "fmt"
    "os"
)

// Terminal writes escalations to stderr with visual severity indicators
type Terminal struct{}

// NewTerminal creates a terminal escalator
func NewTerminal() *Terminal {
    return &Terminal{}
}

// Escalate writes the escalation to stderr
func (t *Terminal) Escalate(ctx context.Context, e Escalation) error {
    prefix := ""
    switch e.Severity {
    case SeverityCritical, SeverityBlocking:
        prefix = "🚨 "
    case SeverityWarning:
        prefix = "⚠️  "
    default:
        prefix = "ℹ️  "
    }

    fmt.Fprintf(os.Stderr, "\n%s[%s] %s\n", prefix, e.Severity, e.Title)
    fmt.Fprintf(os.Stderr, "   Unit: %s\n", e.Unit)
    fmt.Fprintf(os.Stderr, "   %s\n", e.Message)

    for k, v := range e.Context {
        fmt.Fprintf(os.Stderr, "   %s: %s\n", k, v)
    }

    return nil
}

// Name returns "terminal"
func (t *Terminal) Name() string {
    return "terminal"
}
```

### Tests to Implement

```go
// internal/escalate/terminal_test.go

package escalate

import (
    "bytes"
    "context"
    "os"
    "strings"
    "testing"
)

func TestTerminal_Escalate(t *testing.T) {
    // Capture stderr
    oldStderr := os.Stderr
    r, w, _ := os.Pipe()
    os.Stderr = w

    term := NewTerminal()
    err := term.Escalate(context.Background(), Escalation{
        Severity: SeverityCritical,
        Unit:     "auth-service",
        Title:    "Database connection failed",
        Message:  "Cannot connect to PostgreSQL after 3 retries",
        Context: map[string]string{
            "host":  "db.example.com",
            "error": "connection refused",
        },
    })

    w.Close()
    os.Stderr = oldStderr

    var buf bytes.Buffer
    buf.ReadFrom(r)
    output := buf.String()

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    if !strings.Contains(output, "[critical]") {
        t.Error("expected severity in output")
    }
    if !strings.Contains(output, "Database connection failed") {
        t.Error("expected title in output")
    }
    if !strings.Contains(output, "auth-service") {
        t.Error("expected unit in output")
    }
}

func TestTerminal_Name(t *testing.T) {
    term := NewTerminal()
    if term.Name() != "terminal" {
        t.Errorf("expected 'terminal', got %q", term.Name())
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/escalate/... -run Terminal
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestTerminal_Escalate | Output contains severity, title, unit |
| TestTerminal_Name | Returns "terminal" |

## NOT In Scope
- Network-based escalators (Slack, Webhook)
- Multi escalator
- Factory function
