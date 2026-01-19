---
task: 1
status: complete
backpressure: "go build ./internal/escalate/..."
depends_on: []
---

# Core Types and Interface

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define the core Escalator interface, Severity type, and Escalation struct that all backends will implement.

## Dependencies

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `context` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/escalate/
└── escalate.go    # CREATE: Core types and interface
```

### Types to Implement

```go
// internal/escalate/escalate.go

package escalate

import "context"

// Severity indicates how urgent the escalation is
type Severity string

const (
    SeverityInfo     Severity = "info"     // FYI, no action needed
    SeverityWarning  Severity = "warning"  // May need attention
    SeverityCritical Severity = "critical" // Requires immediate action
    SeverityBlocking Severity = "blocking" // Cannot proceed without user
)

// Escalation represents something that needs user attention
type Escalation struct {
    Severity Severity          // How urgent is this?
    Unit     string            // Which unit is affected
    Title    string            // Short summary (one line)
    Message  string            // Detailed explanation
    Context  map[string]string // Additional context (PR URL, error details, etc.)
}

// Escalator is the interface for notifying users
type Escalator interface {
    // Escalate sends a notification to the user.
    // Returns nil if notification was sent successfully.
    // Implementations should respect context cancellation.
    Escalate(ctx context.Context, e Escalation) error

    // Name returns the escalator type for logging
    Name() string
}
```

## Backpressure

### Validation Command
```bash
go build ./internal/escalate/...
```

### Must Pass
| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Severity constants defined | Four severity levels exist |
| Escalator interface compiles | Interface has Escalate and Name methods |

## NOT In Scope
- Any backend implementations (Terminal, Slack, Webhook, Multi)
- Factory function
- Tests (no behavior to test, just type definitions)
