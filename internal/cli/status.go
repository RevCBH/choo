package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/git"
	"github.com/spf13/cobra"
)

// StatusOptions holds flags for the status command
type StatusOptions struct {
	TasksDir string // Path to specs/tasks/ directory
	JSON     bool   // Output as JSON instead of formatted text
}

// NewStatusCmd creates the status command
func NewStatusCmd(app *App) *cobra.Command {
	opts := StatusOptions{
		TasksDir: "specs/tasks",
	}

	cmd := &cobra.Command{
		Use:   "status [tasks-dir]",
		Short: "Show current orchestration status",
		Long:  `Display the current orchestration progress with formatted output.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use provided directory argument if present
			if len(args) > 0 {
				opts.TasksDir = args[0]
			}
			return app.ShowStatus(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON instead of formatted text")

	return cmd
}

// ShowStatus displays the current orchestration status
func (a *App) ShowStatus(opts StatusOptions) error {
	// Load discovery from tasks directory
	units, err := discovery.Discover(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("failed to load discovery: %w", err)
	}

	// Refresh task statuses from worktrees if they exist
	wd, err := os.Getwd()
	if err == nil {
		refreshTaskStatusesFromWorktrees(context.Background(), wd, units)
	}

	// Convert to UnitDisplay
	unitDisplays := convertToUnitDisplays(units)

	// Output format
	if opts.JSON {
		return outputJSON(os.Stdout, unitDisplays)
	}

	// Format and print output
	cfg := DisplayConfig{
		Width:          20, // Progress bar width
		UseColor:       false,
		ShowTimestamps: false,
	}

	output := formatStatusOutput(unitDisplays, cfg)
	fmt.Fprint(os.Stdout, output)

	return nil
}

// formatStatusOutput produces the full status display
func formatStatusOutput(units []UnitDisplay, cfg DisplayConfig) string {
	var result strings.Builder

	// Header with separator
	separator := strings.Repeat("═", 63)
	result.WriteString(separator + "\n")
	result.WriteString("Ralph Orchestrator Status\n")
	result.WriteString("Target: main | Parallelism: 4\n")
	result.WriteString(separator + "\n")
	result.WriteString("\n")

	// Each unit with progress bar and tasks
	for _, unit := range units {
		result.WriteString(FormatUnitStatus(&unit, cfg))
		result.WriteString("\n")
	}

	// Summary footer
	unitStats := calculateUnitStats(units)
	taskStats := calculateTaskStats(units)

	thinSeparator := strings.Repeat("─", 63)
	result.WriteString(thinSeparator + "\n")
	result.WriteString(fmt.Sprintf(" Units: %d | Complete: %d | In Progress: %d | Pending: %d\n",
		unitStats.Total, unitStats.Complete, unitStats.InProgress, unitStats.Pending))
	result.WriteString(fmt.Sprintf(" Tasks: %d | Complete: %d | In Progress: %d | Pending: %d\n",
		taskStats.Total, taskStats.Complete, taskStats.InProgress, taskStats.Pending))
	result.WriteString(separator + "\n")

	return result.String()
}

// Stats holds statistics for units or tasks
type Stats struct {
	Total      int
	Complete   int
	InProgress int
	Pending    int
}

// calculateUnitStats computes unit statistics
func calculateUnitStats(units []UnitDisplay) Stats {
	stats := Stats{Total: len(units)}

	for _, unit := range units {
		switch unit.Status {
		case discovery.UnitStatusComplete:
			stats.Complete++
		case discovery.UnitStatusInProgress:
			stats.InProgress++
		case discovery.UnitStatusPending:
			stats.Pending++
		}
	}

	return stats
}

// calculateTaskStats computes task statistics across all units
func calculateTaskStats(units []UnitDisplay) Stats {
	stats := Stats{}

	for _, unit := range units {
		for _, task := range unit.Tasks {
			stats.Total++
			switch task.Status {
			case discovery.TaskStatusComplete:
				stats.Complete++
			case discovery.TaskStatusInProgress:
				stats.InProgress++
			case discovery.TaskStatusPending:
				stats.Pending++
			}
		}
	}

	return stats
}

// outputJSON writes unit displays as JSON
func outputJSON(w io.Writer, units []UnitDisplay) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(units)
}

// convertToUnitDisplays converts discovery units to display units
func convertToUnitDisplays(units []*discovery.Unit) []UnitDisplay {
	displays := make([]UnitDisplay, 0, len(units))

	for _, unit := range units {
		display := UnitDisplay{
			ID:     unit.ID,
			Status: unit.Status,
			Tasks:  make([]TaskDisplay, 0, len(unit.Tasks)),
		}

		// Calculate progress based on completed tasks and infer unit status
		if len(unit.Tasks) > 0 {
			completed := 0
			inProgress := 0
			for _, task := range unit.Tasks {
				switch task.Status {
				case discovery.TaskStatusComplete:
					completed++
				case discovery.TaskStatusInProgress:
					inProgress++
				}
			}
			display.Progress = float64(completed) / float64(len(unit.Tasks))

			// Infer unit status from task status (orch_ fields are runtime-only)
			if completed == len(unit.Tasks) {
				display.Status = discovery.UnitStatusComplete
			} else if inProgress > 0 || completed > 0 {
				display.Status = discovery.UnitStatusInProgress
			}
		}

		// Convert tasks
		for i, task := range unit.Tasks {
			taskDisplay := TaskDisplay{
				Number:   i + 1,
				FileName: task.FilePath,
				Status:   task.Status,
				Active:   task.Status == discovery.TaskStatusInProgress,
			}
			display.Tasks = append(display.Tasks, taskDisplay)
		}

		// Add PR info if present
		if unit.PRNumber > 0 {
			display.PRNumber = &unit.PRNumber
			display.PRStatus = "open"
			if unit.Status == discovery.UnitStatusComplete {
				display.PRStatus = "merged"
			}
		}

		displays = append(displays, display)
	}

	return displays
}

// refreshTaskStatusesFromWorktrees checks for existing worktrees and refreshes
// task statuses from them. This ensures status reflects actual progress in worktrees.
func refreshTaskStatusesFromWorktrees(ctx context.Context, repoRoot string, units []*discovery.Unit) {
	// Create worktree manager to find existing worktrees
	wtManager := git.NewWorktreeManager(repoRoot, nil)

	for _, unit := range units {
		// Check if a worktree exists for this unit
		wt, err := wtManager.GetWorktree(ctx, unit.ID)
		if err != nil || wt == nil {
			continue // No worktree for this unit
		}

		// Compute unit path relative to repo root
		unitPath := unit.Path
		if filepath.IsAbs(unitPath) {
			relPath, err := filepath.Rel(repoRoot, unitPath)
			if err != nil {
				continue
			}
			unitPath = relPath
		}

		// Re-parse each task from the worktree
		for _, task := range unit.Tasks {
			taskPath := filepath.Join(wt.Path, unitPath, task.FilePath)
			updated, err := discovery.ParseTaskFile(taskPath)
			if err != nil {
				continue // Could not parse, keep original status
			}

			// Update task status from worktree
			task.Status = updated.Status
		}
	}
}
