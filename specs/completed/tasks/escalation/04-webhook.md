---
task: 4
status: complete
backpressure: "go test ./internal/escalate/... -run Webhook"
depends_on: [1]
---

# Webhook Escalator

**Parent spec**: `/specs/ESCALATION.md`
**Task**: #4 of 6 in implementation plan

## Objective

Implement the generic Webhook escalator that posts JSON payloads to arbitrary HTTP endpoints.

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
├── webhook.go       # CREATE: Webhook backend implementation
└── webhook_test.go  # CREATE: Webhook tests
```

### Types to Implement

```go
// internal/escalate/webhook.go

package escalate

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// WebhookPayload is the JSON structure sent to webhook endpoints
type WebhookPayload struct {
    Severity string            `json:"severity"`
    Unit     string            `json:"unit"`
    Title    string            `json:"title"`
    Message  string            `json:"message"`
    Context  map[string]string `json:"context,omitempty"`
}

// Webhook posts escalations to an HTTP endpoint as JSON
type Webhook struct {
    url    string
    client *http.Client
}

// NewWebhook creates a Webhook escalator with default HTTP client
func NewWebhook(url string) *Webhook {
    return &Webhook{
        url: url,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// NewWebhookWithClient creates a Webhook escalator with custom HTTP client
func NewWebhookWithClient(url string, client *http.Client) *Webhook {
    return &Webhook{
        url:    url,
        client: client,
    }
}

// Escalate posts the escalation as JSON to the webhook URL
func (w *Webhook) Escalate(ctx context.Context, e Escalation) error {
    payload := WebhookPayload{
        Severity: string(e.Severity),
        Unit:     e.Unit,
        Title:    e.Title,
        Message:  e.Message,
        Context:  e.Context,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal webhook payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("create webhook request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := w.client.Do(req)
    if err != nil {
        return fmt.Errorf("webhook request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}

// Name returns "webhook"
func (w *Webhook) Name() string {
    return "webhook"
}
```

### Tests to Implement

```go
// internal/escalate/webhook_test.go

package escalate

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestWebhook_Escalate(t *testing.T) {
    var receivedPayload WebhookPayload

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

    webhook := NewWebhook(server.URL)
    err := webhook.Escalate(context.Background(), Escalation{
        Severity: SeverityCritical,
        Unit:     "payment-service",
        Title:    "Payment processing failed",
        Message:  "Stripe API returned 503",
        Context: map[string]string{
            "transaction_id": "txn_123",
        },
    })

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    if receivedPayload.Severity != "critical" {
        t.Errorf("expected severity 'critical', got %q", receivedPayload.Severity)
    }
    if receivedPayload.Unit != "payment-service" {
        t.Errorf("expected unit 'payment-service', got %q", receivedPayload.Unit)
    }
    if receivedPayload.Context["transaction_id"] != "txn_123" {
        t.Error("expected context to include transaction_id")
    }
}

func TestWebhook_EscalateError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusBadRequest)
    }))
    defer server.Close()

    webhook := NewWebhook(server.URL)
    err := webhook.Escalate(context.Background(), Escalation{
        Severity: SeverityInfo,
        Unit:     "test",
        Title:    "Test",
        Message:  "Test message",
    })

    if err == nil {
        t.Error("expected error for 400 response")
    }
}

func TestWebhook_Name(t *testing.T) {
    webhook := NewWebhook("http://example.com")
    if webhook.Name() != "webhook" {
        t.Errorf("expected 'webhook', got %q", webhook.Name())
    }
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/escalate/... -run Webhook
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestWebhook_Escalate | Sends POST with WebhookPayload JSON, fields match input |
| TestWebhook_EscalateError | Returns error on 400 response |
| TestWebhook_Name | Returns "webhook" |

## NOT In Scope
- Terminal escalator
- Slack escalator (uses different payload format)
- Multi escalator
- Factory function
