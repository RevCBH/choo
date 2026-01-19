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
