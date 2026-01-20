package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/worker"
	"github.com/spf13/cobra"
)

// PromptOptions holds flags for the prompt command
type PromptOptions struct {
	TasksDir string // Path to specs/tasks/ directory
	Task     int    // Specific task number (0 = all ready tasks)
}

// NewPromptCmd creates the prompt command
func NewPromptCmd(app *App) *cobra.Command {
	opts := PromptOptions{
		TasksDir: "specs/tasks",
		Task:     0,
	}

	cmd := &cobra.Command{
		Use:   "prompt <unit>",
		Short: "Show the prompt that would be sent to Claude for a unit",
		Long: `Prompt generates and displays the exact prompt that would be sent to Claude
when executing tasks for a specific unit. This is useful for debugging
why Claude might be behaving unexpectedly.

Examples:
  choo prompt web                    # Show prompt for all ready tasks in 'web' unit
  choo prompt web --task 1           # Show prompt for task #1 specifically
  choo prompt web-frontend           # Show prompt for web-frontend unit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			unitID := args[0]
			return app.ShowPrompt(unitID, opts)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&opts.TasksDir, "tasks-dir", "specs/tasks", "Path to tasks directory")
	cmd.Flags().IntVar(&opts.Task, "task", 0, "Show prompt for specific task number (0 = all ready)")

	return cmd
}

// ShowPrompt generates and displays the prompt for a unit
func (a *App) ShowPrompt(unitID string, opts PromptOptions) error {
	// Discover units
	units, err := discovery.Discover(opts.TasksDir)
	if err != nil {
		return fmt.Errorf("failed to discover units: %w", err)
	}

	// Find the specified unit
	var unit *discovery.Unit
	for _, u := range units {
		if u.ID == unitID {
			unit = u
			break
		}
	}

	if unit == nil {
		fmt.Fprintf(os.Stderr, "Unit '%s' not found. Available units:\n", unitID)
		for _, u := range units {
			fmt.Fprintf(os.Stderr, "  - %s (%d tasks)\n", u.ID, len(u.Tasks))
		}
		return fmt.Errorf("unit not found: %s", unitID)
	}

	// Print unit info
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("Unit: %s\n", unit.ID)
	fmt.Printf("Path: %s\n", unit.Path)
	fmt.Printf("Tasks: %d\n", len(unit.Tasks))
	if len(unit.DependsOn) > 0 {
		fmt.Printf("Depends on: %v\n", unit.DependsOn)
	}
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Determine which tasks to show
	var readyTasks []*discovery.Task

	if opts.Task > 0 {
		// Show specific task
		for _, t := range unit.Tasks {
			if t.Number == opts.Task {
				readyTasks = append(readyTasks, t)
				break
			}
		}
		if len(readyTasks) == 0 {
			return fmt.Errorf("task #%d not found in unit %s", opts.Task, unitID)
		}
	} else {
		// Find ready tasks (tasks with all dependencies satisfied and pending status)
		completedSet := make(map[int]bool)
		for _, t := range unit.Tasks {
			if t.Status == discovery.TaskStatusComplete {
				completedSet[t.Number] = true
			}
		}

		for _, t := range unit.Tasks {
			if t.Status != discovery.TaskStatusPending {
				continue
			}
			// Check if all dependencies are satisfied
			allDepsComplete := true
			for _, dep := range t.DependsOn {
				if !completedSet[dep] {
					allDepsComplete = false
					break
				}
			}
			if allDepsComplete {
				readyTasks = append(readyTasks, t)
			}
		}

		if len(readyTasks) == 0 {
			fmt.Println("No ready tasks found. Task statuses:")
			for _, t := range unit.Tasks {
				deps := ""
				if len(t.DependsOn) > 0 {
					deps = fmt.Sprintf(" (depends on: %v)", t.DependsOn)
				}
				fmt.Printf("  #%d: %s [%s]%s\n", t.Number, t.Title, t.Status, deps)
			}
			return nil
		}
	}

	// Print task info
	fmt.Printf("Ready Tasks (%d):\n", len(readyTasks))
	for _, t := range readyTasks {
		fmt.Printf("  #%d: %s\n", t.Number, t.Title)
		fmt.Printf("      File: %s\n", t.FilePath)
		fmt.Printf("      Status: %s\n", t.Status)
		fmt.Printf("      Backpressure: %s\n", t.Backpressure)
	}
	fmt.Println()

	// Build and print the prompt
	prompt := worker.BuildTaskPrompt(readyTasks)

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("CLAUDE PROMPT:\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Println(prompt.Content)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	// Also print the spec file content for context
	if len(readyTasks) == 1 {
		task := readyTasks[0]
		// The FilePath is relative to the unit, so we need to construct full path
		specPath := filepath.Join(unit.Path, filepath.Base(task.FilePath))
		fmt.Printf("\nSpec File Content (%s):\n", specPath)
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		content, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Printf("(Could not read spec file: %v)\n", err)
		} else {
			fmt.Println(string(content))
		}
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	}

	return nil
}
