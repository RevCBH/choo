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

func TestTerminal_SeverityFormatting(t *testing.T) {
	tests := []struct {
		name           string
		severity       Severity
		expectedPrefix string
		expectedLabel  string
	}{
		{
			name:           "critical shows alert emoji",
			severity:       SeverityCritical,
			expectedPrefix: "🚨",
			expectedLabel:  "[critical]",
		},
		{
			name:           "blocking shows alert emoji",
			severity:       SeverityBlocking,
			expectedPrefix: "🚨",
			expectedLabel:  "[blocking]",
		},
		{
			name:           "warning shows warning emoji",
			severity:       SeverityWarning,
			expectedPrefix: "⚠️",
			expectedLabel:  "[warning]",
		},
		{
			name:           "info shows info emoji",
			severity:       SeverityInfo,
			expectedPrefix: "ℹ️",
			expectedLabel:  "[info]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			term := NewTerminal()
			err := term.Escalate(context.Background(), Escalation{
				Severity: tt.severity,
				Unit:     "test-unit",
				Title:    "Test Title",
				Message:  "Test message",
			})

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !strings.Contains(output, tt.expectedPrefix) {
				t.Errorf("expected prefix %q in output, got: %s", tt.expectedPrefix, output)
			}
			if !strings.Contains(output, tt.expectedLabel) {
				t.Errorf("expected label %q in output, got: %s", tt.expectedLabel, output)
			}
		})
	}
}
