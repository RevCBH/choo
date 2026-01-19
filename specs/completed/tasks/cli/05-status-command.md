---
task: 5
status: complete
backpressure: "go test ./internal/cli/... -run Status"
depends_on: [1, 2]
---

# Status Command

**Parent spec**: `/specs/CLI.md`
**Task**: #5 of 9 in implementation plan

## Objective

Implement the status command that displays current orchestration progress with formatted output.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Discovery`, `Unit`, `Task` types
- CONFIG - provides `Config` type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)
- Task #2 must be complete (provides: display formatting functions)

### Package Dependencies
- None beyond standard library and internal packages

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── status.go    # CREATE: Status command implementation
```

### Types to Implement

```go
// StatusOptions holds flags for the status command
type StatusOptions struct {
    TasksDir string // Path to specs/tasks/ directory
    JSON     bool   // Output as JSON instead of formatted text
}
```

### Functions to Implement

```go
// NewStatusCmd creates the status command
func NewStatusCmd(app *App) *cobra.Command {
    // Create command with Use: "status [tasks-dir]"
    // Add --json flag
    // Set default tasks-dir to "specs/tasks"
    // Call ShowStatus in RunE
}

// ShowStatus displays the current orchestration status
func (a *App) ShowStatus(opts StatusOptions) error {
    // Load discovery from tasks directory
    // Convert to UnitDisplay/TaskDisplay
    // Format and print output
}

// formatStatusOutput produces the full status display
func formatStatusOutput(units []UnitDisplay, cfg DisplayConfig) string {
    // Header with separator
    // Each unit with progress bar and tasks
    // Summary footer
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Status -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestStatusCmd_DefaultDir` | Uses "specs/tasks" when no arg provided |
| `TestStatusCmd_CustomDir` | Uses provided directory argument |
| `TestStatusCmd_JSONFlag` | --json flag is recognized |
| `TestShowStatus_NoUnits` | Handles empty directory gracefully |
| `TestShowStatus_WithUnits` | Displays units with correct formatting |
| `TestFormatStatusOutput_Header` | Output includes header with separators |
| `TestFormatStatusOutput_Summary` | Output includes unit/task counts |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock discovery data | In-memory | Test display formatting |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Status output format from spec:
  ```
  ═══════════════════════════════════════════════════════════════
  Ralph Orchestrator Status
  Target: main | Parallelism: 4
  ═══════════════════════════════════════════════════════════════

   [unit-name] ████████████████████ 100% (complete)
     checkmark #1  01-task.md
     checkmark #2  02-task.md
     PR #42 merged

   [unit-name] ████████░░░░░░░░░░░░  40% (in_progress)
     checkmark #1  01-task.md
     bullet #2  02-task.md         <- executing
     circle #3  03-task.md

  ───────────────────────────────────────────────────────────────
   Units: 3 | Complete: 1 | In Progress: 1 | Pending: 1
   Tasks: 9 | Complete: 5 | In Progress: 1 | Pending: 3
  ═══════════════════════════════════════════════════════════════
  ```

- Default terminal width: 60 characters for progress bar
- JSON output should include all the same data in structured format

## NOT In Scope

- Live-updating display (future enhancement)
- Color output (optional enhancement)
- Filtering by unit or status
