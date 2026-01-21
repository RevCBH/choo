package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/RevCBH/choo/internal/client"
	"github.com/RevCBH/choo/internal/events"
)

// displayEvent renders an event to the terminal with appropriate formatting
// based on event type. Handles unit events, task events, and system events.
func displayEvent(e events.Event) {
	// Format timestamp
	timestamp := formatTime(e.Time)

	// Build the event message based on type
	var msg string
	switch e.Type {
	case events.UnitStarted:
		msg = fmt.Sprintf("[%s] Unit started: %s", timestamp, e.Unit)
	case events.UnitCompleted:
		msg = fmt.Sprintf("[%s] Unit completed: %s", timestamp, e.Unit)
	case events.UnitFailed:
		msg = fmt.Sprintf("[%s] Unit failed: %s", timestamp, e.Unit)
		if e.Error != "" {
			msg += fmt.Sprintf(" - %s", e.Error)
		}
	case events.TaskStarted:
		taskNum := ""
		if e.Task != nil {
			taskNum = fmt.Sprintf("#%d", *e.Task)
		}
		msg = fmt.Sprintf("[%s] Task started: %s %s", timestamp, e.Unit, taskNum)
	case events.TaskCompleted:
		taskNum := ""
		if e.Task != nil {
			taskNum = fmt.Sprintf("#%d", *e.Task)
		}
		msg = fmt.Sprintf("[%s] Task completed: %s %s", timestamp, e.Unit, taskNum)
	case events.TaskFailed:
		taskNum := ""
		if e.Task != nil {
			taskNum = fmt.Sprintf("#%d", *e.Task)
		}
		msg = fmt.Sprintf("[%s] Task failed: %s %s", timestamp, e.Unit, taskNum)
		if e.Error != "" {
			msg += fmt.Sprintf(" - %s", e.Error)
		}
	case events.OrchStarted:
		msg = fmt.Sprintf("[%s] Orchestrator started", timestamp)
	case events.OrchCompleted:
		msg = fmt.Sprintf("[%s] Orchestrator completed", timestamp)
	case events.OrchFailed:
		msg = fmt.Sprintf("[%s] Orchestrator failed", timestamp)
		if e.Error != "" {
			msg += fmt.Sprintf(" - %s", e.Error)
		}
	default:
		// Generic format for unknown event types
		msg = fmt.Sprintf("[%s] %s: %s", timestamp, e.Type, e.Unit)
	}

	fmt.Println(msg)
}

// displayJobs renders a list of jobs in tabular format using tabwriter.
// Columns: ID, Status, Feature Branch, Units, Started
func displayJobs(jobs []*client.JobSummary) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header row
	fmt.Fprintln(w, "ID\tSTATUS\tFEATURE BRANCH\tUNITS\tSTARTED")

	// Print each job
	for _, job := range jobs {
		units := fmt.Sprintf("%d/%d", job.UnitsComplete, job.UnitsTotal)
		started := formatTime(job.StartedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			job.JobID,
			job.Status,
			job.FeatureBranch,
			units,
			started,
		)
	}
}

// boolToStatus converts a health boolean to a human-readable status string.
func boolToStatus(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}

// defaultSocketPath returns the standard daemon socket location.
// Uses ~/.choo/ for consistency with daemon config.
func defaultSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".choo", "daemon.sock")
}

// formatTime formats a timestamp for display
func formatTime(t time.Time) string {
	// Use consistent time format: HH:MM:SS
	return t.Format("15:04:05")
}
