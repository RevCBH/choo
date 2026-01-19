---
task: 6
status: complete
backpressure: "go test ./internal/escalate/... -run FromConfig"
depends_on: [1, 2, 3, 4, 5]
---

# Factory Function

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement the FromConfig factory function that creates an Escalator from configuration.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Core types and interface
- Task #2: Terminal escalator
- Task #3: Slack escalator
- Task #4: Webhook escalator
- Task #5: Multi escalator

### Package Dependencies
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/escalate/
├── factory.go       # CREATE: Factory function
└── factory_test.go  # CREATE: Factory tests
```

### Types to Implement

```go
// internal/escalate/factory.go

package escalate

import "fmt"

// Config holds escalation configuration
type Config struct {
    Backends     []string
    SlackWebhook string
    WebhookURL   string
}

// FromConfig creates an Escalator from configuration
func FromConfig(cfg Config) (Escalator, error) {
    var escalators []Escalator

    for _, backend := range cfg.Backends {
        switch backend {
        case "terminal":
            escalators = append(escalators, NewTerminal())
        case "slack":
            if cfg.SlackWebhook == "" {
                return nil, fmt.Errorf("slack backend requires webhook URL")
            }
            escalators = append(escalators, NewSlack(cfg.SlackWebhook))
        case "webhook":
            if cfg.WebhookURL == "" {
                return nil, fmt.Errorf("webhook backend requires URL")
            }
            escalators = append(escalators, NewWebhook(cfg.WebhookURL))
        default:
            return nil, fmt.Errorf("unknown escalation backend: %s", backend)
        }
    }

    if len(escalators) == 0 {
        return NewTerminal(), nil
    }

    if len(escalators) == 1 {
        return escalators[0], nil
    }

    return NewMulti(escalators...), nil
}
```

### Tests to Implement

```go
// internal/escalate/factory_test.go

package escalate

import (
    "testing"
)

func TestFromConfig_Empty(t *testing.T) {
    esc, err := FromConfig(Config{})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if esc.Name() != "terminal" {
        t.Errorf("expected default terminal, got %q", esc.Name())
    }
}

func TestFromConfig_Terminal(t *testing.T) {
    esc, err := FromConfig(Config{
        Backends: []string{"terminal"},
    })
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if esc.Name() != "terminal" {
        t.Errorf("expected terminal, got %q", esc.Name())
    }
}

func TestFromConfig_Slack(t *testing.T) {
    esc, err := FromConfig(Config{
        Backends:     []string{"slack"},
        SlackWebhook: "https://hooks.slack.com/services/xxx",
    })
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if esc.Name() != "slack" {
        t.Errorf("expected slack, got %q", esc.Name())
    }
}

func TestFromConfig_SlackMissingURL(t *testing.T) {
    _, err := FromConfig(Config{
        Backends: []string{"slack"},
    })
    if err == nil {
        t.Error("expected error for missing slack webhook URL")
    }
}

func TestFromConfig_Webhook(t *testing.T) {
    esc, err := FromConfig(Config{
        Backends:   []string{"webhook"},
        WebhookURL: "https://example.com/webhook",
    })
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if esc.Name() != "webhook" {
        t.Errorf("expected webhook, got %q", esc.Name())
    }
}

func TestFromConfig_WebhookMissingURL(t *testing.T) {
    _, err := FromConfig(Config{
        Backends: []string{"webhook"},
    })
    if err == nil {
        t.Error("expected error for missing webhook URL")
    }
}

func TestFromConfig_Multi(t *testing.T) {
    esc, err := FromConfig(Config{
        Backends:     []string{"terminal", "slack"},
        SlackWebhook: "https://hooks.slack.com/services/xxx",
    })
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if esc.Name() != "multi" {
        t.Errorf("expected multi, got %q", esc.Name())
    }
}

func TestFromConfig_UnknownBackend(t *testing.T) {
    _, err := FromConfig(Config{
        Backends: []string{"unknown"},
    })
    if err == nil {
        t.Error("expected error for unknown backend")
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/escalate/... -run FromConfig
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestFromConfig_Empty | Returns terminal as default |
| TestFromConfig_Terminal | Returns terminal escalator |
| TestFromConfig_Slack | Returns slack escalator when URL provided |
| TestFromConfig_SlackMissingURL | Returns error when URL missing |
| TestFromConfig_Webhook | Returns webhook escalator when URL provided |
| TestFromConfig_WebhookMissingURL | Returns error when URL missing |
| TestFromConfig_Multi | Returns multi when multiple backends |
| TestFromConfig_UnknownBackend | Returns error for unknown backend |

## NOT In Scope
- Backend implementations (already completed in tasks 2-5)
- Integration with main config system
