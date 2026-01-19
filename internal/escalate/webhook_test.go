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
