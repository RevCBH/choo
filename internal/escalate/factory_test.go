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
