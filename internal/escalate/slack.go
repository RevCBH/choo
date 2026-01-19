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
