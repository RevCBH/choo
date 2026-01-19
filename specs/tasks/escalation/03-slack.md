---
task: 3
status: pending
backpressure: "go test ./internal/escalate/... -run Slack"
depends_on: [1]
---

# Slack Escalator

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement the Slack escalator that posts formatted messages to a Slack webhook URL with Block Kit formatting.

## Dependencies

### Task Dependencies (within this unit)
- Task #1: Core types and interface

### Package Dependencies
- `bytes` (standard library)
- `context` (standard library)
- `encoding/json` (standard library)
- `fmt` (standard library)
- `net/http` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify
```
internal/escalate/
├── slack.go       # CREATE: Slack backend implementation
└── slack_test.go  # CREATE: Slack tests
```

### Types to Implement

```go
// internal/escalate/slack.go

package escalate

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Slack posts escalations to a Slack webhook URL
type Slack struct {
    webhookURL string
    client     *http.Client
}

// NewSlack creates a Slack escalator with default HTTP client
func NewSlack(webhookURL string) *Slack {
    return &Slack{
        webhookURL: webhookURL,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// NewSlackWithClient creates a Slack escalator with custom HTTP client
func NewSlackWithClient(webhookURL string, client *http.Client) *Slack {
    return &Slack{
        webhookURL: webhookURL,
        client:     client,
    }
}

// Escalate posts the escalation to Slack
func (s *Slack) Escalate(ctx context.Context, e Escalation) error {
    emoji := map[Severity]string{
        SeverityInfo:     ":information_source:",
        SeverityWarning:  ":warning:",
        SeverityCritical: ":rotating_light:",
        SeverityBlocking: ":octagonal_sign:",
    }[e.Severity]

    // Build context fields for the message
    var contextFields []map[string]any
    for k, v := range e.Context {
        contextFields = append(contextFields, map[string]any{
            "type": "mrkdwn",
            "text": fmt.Sprintf("*%s:* %s", k, v),
        })
    }

    blocks := []map[string]any{
        {
            "type": "section",
            "text": map[string]string{
                "type": "mrkdwn",
                "text": fmt.Sprintf("*%s*\n%s", e.Title, e.Message),
            },
        },
    }

    // Add context block if we have context fields
    if len(contextFields) > 0 {
        blocks = append(blocks, map[string]any{
            "type":     "context",
            "elements": contextFields,
        })
    }

    payload := map[string]any{
        "text":   fmt.Sprintf("%s *[%s]* %s", emoji, e.Unit, e.Title),
        "blocks": blocks,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal slack payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("create slack request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return fmt.Errorf("slack webhook request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
    }
    return nil
}

// Name returns "slack"
func (s *Slack) Name() string {
    return "slack"
}
```

### Tests to Implement

```go
// internal/escalate/slack_test.go

package escalate

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestSlack_Escalate(t *testing.T) {
    var receivedPayload map[string]any

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            t.Errorf("expected POST, got %s", r.Method)
        }
        if r.Header.Get("Content-Type") != "application/json" {
            t.Error("expected Content-Type: application/json")
        }
        json.NewDecoder(r.Body).Decode(&receivedPayload)
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    slack := NewSlack(server.URL)
    err := slack.Escalate(context.Background(), Escalation{
        Severity: SeverityWarning,
        Unit:     "api-gateway",
        Title:    "High latency detected",
        Message:  "P99 latency exceeded 500ms",
    })

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    text, ok := receivedPayload["text"].(string)
    if !ok || text == "" {
        t.Error("expected text field in payload")
    }
}

func TestSlack_EscalateError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer server.Close()

    slack := NewSlack(server.URL)
    err := slack.Escalate(context.Background(), Escalation{
        Severity: SeverityInfo,
        Unit:     "test",
        Title:    "Test",
        Message:  "Test message",
    })

    if err == nil {
        t.Error("expected error for 500 response")
    }
}

func TestSlack_Name(t *testing.T) {
    slack := NewSlack("http://example.com")
    if slack.Name() != "slack" {
        t.Errorf("expected 'slack', got %q", slack.Name())
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/escalate/... -run Slack
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestSlack_Escalate | Sends POST with JSON payload, text field present |
| TestSlack_EscalateError | Returns error on 500 response |
| TestSlack_Name | Returns "slack" |

## NOT In Scope
- Terminal escalator
- Webhook escalator
- Multi escalator
- Factory function
