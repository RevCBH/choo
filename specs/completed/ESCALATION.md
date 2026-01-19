# ESCALATION — User Notification Interface

## Overview

The escalation system provides a pluggable notification interface for alerting users when Claude cannot complete operations. When automated retries fail or human intervention is required, the system needs a clean abstraction to reach users through their preferred channels.

Choo operates autonomously on long-running tasks, but some situations require human attention: merge conflicts that need manual resolution, API rate limits that persist, or blocking decisions that only a human can make. The escalation system bridges this gap by supporting multiple notification backends (terminal, Slack, webhook) with a consistent interface.

```
┌─────────────────────────────────────────────────────────────────┐
│                     Escalation Interface                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                         Escalator
                              │
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
     ┌───────────┐     ┌───────────┐     ┌───────────┐
     │  Terminal │     │   Slack   │     │  Webhook  │
     │ Escalator │     │ Escalator │     │ Escalator │
     └───────────┘     └───────────┘     └───────────┘
            │                 │                 │
            ▼                 ▼                 ▼
         stderr          Slack API        HTTP POST
```

## Requirements

### Functional Requirements

1. Define an `Escalator` interface that all notification backends implement
2. Support four severity levels: info, warning, critical, and blocking
3. Include structured context in escalations (unit name, title, message, key-value metadata)
4. Provide a Terminal escalator that writes to stderr with visual severity indicators
5. Provide a Slack escalator that posts formatted messages to a webhook URL
6. Provide a Webhook escalator that sends JSON payloads to arbitrary HTTP endpoints
7. Provide a Multi escalator that fans out notifications to multiple backends
8. Return errors from failed escalations without blocking other backends in Multi mode

### Performance Requirements

| Metric | Target |
|--------|--------|
| Terminal escalation latency | < 1ms |
| HTTP escalation timeout | 10 seconds |
| Multi fan-out | Concurrent delivery to all backends |

### Constraints

- Terminal escalator must work without network access
- Slack and Webhook escalators require network connectivity
- HTTP clients must respect context cancellation
- No external dependencies beyond standard library for core interface

## Design

### Module Structure

```
internal/escalate/
├── escalate.go   # Interface and types
├── terminal.go   # Terminal (stderr) backend
├── slack.go      # Slack webhook backend
├── webhook.go    # Generic HTTP webhook backend
└── multi.go      # Fan-out multiplexer
```

### Core Types

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

### API Surface

```go
// Constructor functions
func NewTerminal() *Terminal
func NewSlack(webhookURL string) *Slack
func NewSlackWithClient(webhookURL string, client *http.Client) *Slack
func NewWebhook(url string) *Webhook
func NewWebhookWithClient(url string, client *http.Client) *Webhook
func NewMulti(escalators ...Escalator) *Multi

// Interface methods (all types)
func (t *Type) Escalate(ctx context.Context, e Escalation) error
func (t *Type) Name() string
```

### Terminal Escalator

The default escalator writes formatted messages to stderr with emoji indicators for visual scanning.

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

### Slack Escalator

Posts formatted messages to a Slack incoming webhook URL with Block Kit formatting.

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

### Webhook Escalator

Posts JSON payloads to arbitrary HTTP endpoints for integration with custom systems.

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

### Multi Escalator

Fans out escalations to multiple backends, continuing even if some fail.

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

## Implementation Notes

### Context Cancellation

All HTTP-based escalators must respect context cancellation. If the parent operation is cancelled, in-flight HTTP requests should be aborted promptly. The `http.NewRequestWithContext` function handles this automatically.

### Error Handling Strategy

The Multi escalator continues delivering to all backends even if some fail. This ensures critical notifications reach at least some destinations. The first error is returned for logging purposes, but all backends are attempted.

### Thread Safety

- Terminal escalator: Thread-safe (os.Stderr handles synchronization)
- Slack escalator: Thread-safe (stateless, uses HTTP client)
- Webhook escalator: Thread-safe (stateless, uses HTTP client)
- Multi escalator: Thread-safe (concurrent goroutines with proper synchronization)

### Stderr vs Stdout

Terminal output goes to stderr intentionally. This keeps escalation messages separate from any structured output (JSON, etc.) that might go to stdout, allowing proper output redirection in shell pipelines.

### Map Iteration Order

The Context map iteration order is non-deterministic in Go. For Terminal output, this is acceptable since the context fields are supplementary information. For Slack, context fields appear in a horizontal layout where order is less important.

## Testing Strategy

### Unit Tests

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
```

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
```

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
```

### Integration Tests

1. **Slack webhook integration**: Test against Slack's webhook API (requires valid webhook URL)
2. **Context cancellation**: Verify HTTP requests abort when context is cancelled
3. **Concurrent escalations**: Send multiple escalations simultaneously through Multi

### Manual Testing

- [ ] Terminal escalator displays correctly with each severity level
- [ ] Slack messages render with proper Block Kit formatting
- [ ] Webhook payloads are valid JSON with all expected fields
- [ ] Multi escalator delivers to all backends when one fails
- [ ] Long messages and special characters render correctly

## Configuration

The escalation system is configured through the choo configuration file:

```yaml
escalation:
  backends:
    - terminal                    # Always enabled by default
    - slack                       # Optional, requires webhook URL
  slack_webhook: ""               # Set via CHOO_SLACK_WEBHOOK env var
  webhook_url: ""                 # Set via CHOO_WEBHOOK_URL env var
```

### Factory Function

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

## Design Decisions

### Why an Interface?

The `Escalator` interface enables:
1. Easy testing with mock implementations
2. Adding new backends without modifying existing code
3. Runtime composition through Multi escalator
4. Clear contract for what backends must implement

### Why Fan-Out Instead of First-Success?

Multi escalator sends to all backends rather than stopping at first success because:
1. Different backends serve different purposes (terminal for local dev, Slack for team visibility)
2. Backend failures are often transient; we want redundancy
3. Users expect all configured channels to receive notifications

### Why Return First Error Instead of All Errors?

Returning only the first error keeps the interface simple. For debugging, each backend logs its own errors. A multi-error pattern would complicate error handling for callers who just need to know "did it work?"

### Why Stderr for Terminal?

Stdout is reserved for structured output (JSON status, etc.). Stderr is the standard destination for diagnostic and informational messages, allowing clean output redirection.

## Future Enhancements

1. **Email escalator**: SMTP-based notifications for critical escalations
2. **PagerDuty escalator**: Integration with incident management platforms
3. **Escalation throttling**: Rate limiting to prevent notification storms
4. **Escalation history**: Persistent log of all escalations for audit
5. **Acknowledgment flow**: Allow users to acknowledge escalations, affecting retry behavior
6. **Template customization**: User-defined message templates per backend

## References

- PRD Section 3: Escalation System
- PRD Section 10: Configuration
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- [Slack Block Kit](https://api.slack.com/block-kit)
