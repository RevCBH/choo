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
