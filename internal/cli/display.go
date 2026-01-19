package cli

import (
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
)

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
	SymbolComplete   StatusSymbol = "✓"
	SymbolInProgress StatusSymbol = "●"
	SymbolPending    StatusSymbol = "○"
	SymbolFailed     StatusSymbol = "✗"
	SymbolBlocked    StatusSymbol = "→"
)

// RenderProgressBar renders a progress bar of specified width
func RenderProgressBar(progress float64, width int) string {
	// Handle edge cases
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	// Calculate filled vs empty segments
	filled := int(progress * float64(width))
	empty := width - filled

	// Use Unicode block characters
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Append percentage
	percent := int(progress * 100)
	return fmt.Sprintf("[%s] %3d%%", bar, percent)
}

// GetStatusSymbol returns the symbol for a task status
func GetStatusSymbol(status discovery.TaskStatus) StatusSymbol {
	switch status {
	case discovery.TaskStatusComplete:
		return SymbolComplete
	case discovery.TaskStatusInProgress:
		return SymbolInProgress
	case discovery.TaskStatusPending:
		return SymbolPending
	case discovery.TaskStatusFailed:
		return SymbolFailed
	default:
		return SymbolPending
	}
}

// FormatUnitStatus formats a single unit's status for display
func FormatUnitStatus(unit *UnitDisplay, cfg DisplayConfig) string {
	var result strings.Builder

	// Format unit header with progress bar
	progressBar := RenderProgressBar(unit.Progress, cfg.Width)
	result.WriteString(fmt.Sprintf(" [%s] %s (%s)\n", unit.ID, progressBar, unit.Status))

	// Format each task with status symbol
	for _, task := range unit.Tasks {
		result.WriteString(FormatTaskLine(task, task.Active))
		result.WriteString("\n")
	}

	// Format PR info if present
	if unit.PRNumber != nil {
		result.WriteString(fmt.Sprintf("   PR #%d %s\n", *unit.PRNumber, unit.PRStatus))
	}

	// Format blocked-by info if present
	if len(unit.BlockedBy) > 0 {
		result.WriteString(fmt.Sprintf("   → blocked by: %s\n", strings.Join(unit.BlockedBy, ", ")))
	}

	return result.String()
}

// FormatTaskLine formats a single task line
func FormatTaskLine(task TaskDisplay, active bool) string {
	symbol := GetStatusSymbol(task.Status)
	line := fmt.Sprintf("   %s #%d  %s", symbol, task.Number, task.FileName)

	// Add arrow indicator if active
	if active {
		line += "         ← executing"
	}

	return line
}
