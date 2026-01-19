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
