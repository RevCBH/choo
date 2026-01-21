---
task: 1
status: complete
backpressure: "go test ./internal/cli/... -run TestDisplay"
depends_on: []
---

# Display Helpers

**Parent spec**: `specs/DAEMON-CLI.md`
**Task**: #1 of 6 in implementation plan

## Objective

Implement display helper functions for formatting daemon events and job listings in the terminal.

## Dependencies

### Task Dependencies (within this unit)
- None (foundation task)

### Package Dependencies
- `github.com/RevCBH/choo/internal/events` - Event types
- `github.com/RevCBH/choo/internal/client` - JobSummary type (when available)
- `text/tabwriter` - Table formatting

## Deliverables

### Files to Create/Modify

```
internal/cli/
└── daemon_display.go    # CREATE: Display helpers for daemon CLI
```

### Functions to Implement

```go
// displayEvent renders an event to the terminal with appropriate formatting
// based on event type. Handles unit events, task events, and system events.
func displayEvent(e events.Event) {
    // Format timestamp
    // Switch on event type for appropriate formatting
    // Print to stdout with color coding based on event severity
}

// displayJobs renders a list of jobs in tabular format using tabwriter.
// Columns: ID, Status, Feature Branch, Units, Started
func displayJobs(jobs []*client.JobSummary) {
    // Create tabwriter for aligned columns
    // Print header row
    // Print each job with formatted columns
    // Flush writer
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

// formatDuration formats a duration in human-readable form (e.g., "2m30s")
func formatDuration(d time.Duration) string {
    // Truncate to seconds for display
    // Format appropriately based on magnitude
}

// formatTime formats a timestamp for display
func formatTime(t time.Time) string {
    // Use consistent time format
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run TestDisplay
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDisplayEvent_UnitStarted` | Verifies unit started events are formatted with unit name |
| `TestDisplayEvent_TaskComplete` | Verifies task events include task name and status |
| `TestDisplayJobs_Empty` | Verifies empty job list displays header only |
| `TestDisplayJobs_MultipleJobs` | Verifies jobs display in aligned columns |
| `TestBoolToStatus` | Verifies true returns "healthy", false returns "unhealthy" |
| `TestDefaultSocketPath` | Verifies path ends with `.choo/daemon.sock` |

## NOT In Scope

- Actual client integration (deferred to command tasks)
- Color output (can be added later)
- Event streaming logic (task #4)
- Job filtering logic (task #3)
