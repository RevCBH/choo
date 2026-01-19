package events

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogConfig configures the logging handler
type LogConfig struct {
	// Writer is where logs are written (default: os.Stderr)
	Writer io.Writer

	// IncludePayload includes event payload in log output
	IncludePayload bool

	// TimeFormat is the timestamp format (default: RFC3339)
	TimeFormat string
}

// StateConfig configures the state persistence handler
type StateConfig struct {
	// Units is the map of unit ID to Unit pointer for state updates
	Units map[string]Unit

	// OnError is called when state persistence fails
	OnError func(error)
}

// Unit interface for state updates (matches discovery.Unit)
// Define locally to avoid circular imports
type Unit interface {
	SetStatus(status string)
	SetPRNumber(pr int)
	Persist() error
}

// LogHandler returns a handler that logs events to the configured writer
// Format: [event.type] unit task=#N pr=#M
func LogHandler(cfg LogConfig) Handler {
	if cfg.Writer == nil {
		cfg.Writer = os.Stderr
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = time.RFC3339
	}

	return func(e Event) {
		var buf strings.Builder
		buf.WriteString("[")
		buf.WriteString(string(e.Type))
		buf.WriteString("]")

		if e.Unit != "" {
			buf.WriteString(" ")
			buf.WriteString(e.Unit)
		}
		if e.Task != nil {
			fmt.Fprintf(&buf, " task=#%d", *e.Task)
		}
		if e.PR != nil {
			fmt.Fprintf(&buf, " pr=#%d", *e.PR)
		}
		if cfg.IncludePayload && e.Payload != nil {
			fmt.Fprintf(&buf, " payload=%v", e.Payload)
		}
		buf.WriteString("\n")

		fmt.Fprint(cfg.Writer, buf.String())
	}
}

// StateHandler returns a handler that persists unit state changes to frontmatter
// Maps events to unit/task status updates
func StateHandler(cfg StateConfig) Handler {
	return func(e Event) {
		// Look up the unit
		unit, ok := cfg.Units[e.Unit]
		if !ok {
			// Ignore events for unknown units
			return
		}

		// Map event types to state changes
		switch e.Type {
		case UnitStarted:
			unit.SetStatus("in_progress")
		case UnitCompleted:
			unit.SetStatus("complete")
		case UnitFailed:
			unit.SetStatus("failed")
		case UnitBlocked:
			unit.SetStatus("blocked")
		case PRCreated:
			if e.PR != nil {
				unit.SetPRNumber(*e.PR)
			}
			unit.SetStatus("pr_open")
		case PRReviewInProgress:
			unit.SetStatus("in_review")
		case PRMergeQueued:
			unit.SetStatus("merging")
		default:
			// No state change for this event type
			return
		}

		// Persist the state change
		if err := unit.Persist(); err != nil {
			if cfg.OnError != nil {
				cfg.OnError(err)
			}
		}
	}
}
