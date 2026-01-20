package escalate

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Terminal writes escalations to stderr with visual severity indicators
type Terminal struct {
	mu sync.Mutex // Protects concurrent writes to stderr
}

// NewTerminal creates a terminal escalator
func NewTerminal() *Terminal {
	return &Terminal{}
}

// Escalate writes the escalation to stderr
func (t *Terminal) Escalate(ctx context.Context, e Escalation) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	prefix := ""
	switch e.Severity {
	case SeverityCritical, SeverityBlocking:
		prefix = "🚨 "
	case SeverityWarning:
		prefix = "⚠️  "
	default:
		prefix = "ℹ️  "
	}

	// Serialize writes to stderr to prevent concurrent write panics
	t.mu.Lock()
	defer t.mu.Unlock()

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
