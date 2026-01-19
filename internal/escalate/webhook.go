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
