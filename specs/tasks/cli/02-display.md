---
task: 2
status: pending
backpressure: "go test ./internal/cli/... -run Display"
depends_on: []
---

# Display Formatting

**Parent spec**: `/specs/CLI.md`
**Task**: #2 of 9 in implementation plan

## Objective

Implement progress bar rendering and status symbol formatting for CLI output.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `UnitStatus`, `TaskStatus` types

### Task Dependencies (within this unit)
- None (can run in parallel with task #1)

### Package Dependencies
- None beyond standard library

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── display.go    # CREATE: Progress bars and status formatting
```

### Types to Implement

```go
// DisplayConfig controls status output formatting
type DisplayConfig struct {
    Width          int  // Terminal width for progress bars
    UseColor       bool // Enable ANSI color codes
    ShowTimestamps bool // Include timestamps in output
}

// UnitDisplay represents a unit's display state
type UnitDisplay struct {
    ID        string
    Status    discovery.UnitStatus
    Progress  float64 // 0.0 to 1.0
    Tasks     []TaskDisplay
    PRNumber  *int
    PRStatus  string   // "open", "merged", etc.
    BlockedBy []string // unit IDs blocking this unit
}

// TaskDisplay represents a task's display state
type TaskDisplay struct {
    Number   int
    FileName string
    Status   discovery.TaskStatus
    Active   bool // true if currently executing
}

// StatusSymbol returns the appropriate symbol for a task status
type StatusSymbol string

const (
    SymbolComplete   StatusSymbol = "checkmark"
    SymbolInProgress StatusSymbol = "filled_circle"
    SymbolPending    StatusSymbol = "empty_circle"
    SymbolFailed     StatusSymbol = "x_mark"
    SymbolBlocked    StatusSymbol = "arrow"
)
```

### Functions to Implement

```go
// RenderProgressBar renders a progress bar of specified width
func RenderProgressBar(progress float64, width int) string {
    // Calculate filled vs empty segments
    // Use Unicode block characters
    // Append percentage
}

// GetStatusSymbol returns the symbol for a task status
func GetStatusSymbol(status discovery.TaskStatus) StatusSymbol {
    // Map status to symbol
}

// FormatUnitStatus formats a single unit's status for display
func FormatUnitStatus(unit *UnitDisplay, cfg DisplayConfig) string {
    // Format unit header with progress bar
    // Format each task with status symbol
    // Format PR info if present
    // Format blocked-by info if present
}

// FormatTaskLine formats a single task line
func FormatTaskLine(task TaskDisplay, active bool) string {
    // Symbol + task number + filename
    // Add arrow indicator if active
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Display -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRenderProgressBar_Empty` | `RenderProgressBar(0.0, 10)` contains 0% and no filled blocks |
| `TestRenderProgressBar_Half` | `RenderProgressBar(0.5, 10)` contains 50% and 5 filled blocks |
| `TestRenderProgressBar_Full` | `RenderProgressBar(1.0, 10)` contains 100% and all filled blocks |
| `TestGetStatusSymbol_Complete` | Returns `SymbolComplete` for complete status |
| `TestGetStatusSymbol_InProgress` | Returns `SymbolInProgress` for in_progress status |
| `TestGetStatusSymbol_Pending` | Returns `SymbolPending` for pending status |
| `TestGetStatusSymbol_Failed` | Returns `SymbolFailed` for failed status |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use Unicode block characters for progress bar: filled block and light shade
- Progress bar format: `[bar] XXX%` where bar is `width` characters
- Handle edge cases: progress < 0 treated as 0, progress > 1 treated as 1
- Color support is optional for MVP; focus on correct character output first
- Status symbols should be the actual Unicode characters (checkmark, bullet, etc.)

## NOT In Scope

- ANSI color codes (optional enhancement)
- Terminal width detection (use provided width)
- Live-updating display (future enhancement)
