package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Discover finds all units and tasks in the given tasks directory
// Returns an error if the directory doesn't exist or validation fails
func Discover(tasksDir string) ([]*Unit, error) {
	// Verify tasksDir exists and is a directory
	info, err := os.Stat(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("tasks directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("tasks path is not a directory: %s", tasksDir)
	}

	// List all subdirectories of tasksDir
	unitDirs, err := discoverUnitDirs(tasksDir)
	if err != nil {
		return nil, err
	}

	var units []*Unit

	// For each subdirectory
	for _, unitDir := range unitDirs {
		unit, err := DiscoverUnit(unitDir)
		if err != nil {
			// If DiscoverUnit returns an error, it means the directory is invalid
			return nil, err
		}
		// unit can be nil if the directory should be skipped
		if unit != nil {
			units = append(units, unit)
		}
	}

	return units, nil
}

// DiscoverUnit discovers a single unit by directory path
// Useful for targeted re-discovery after file changes
func DiscoverUnit(unitDir string) (*Unit, error) {
	// First verify the unit directory itself exists
	if _, err := os.Stat(unitDir); err != nil {
		return nil, fmt.Errorf("error accessing unit directory %s: %w", unitDir, err)
	}

	// Check for IMPLEMENTATION_PLAN.md
	implPlanPath := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	if _, err := os.Stat(implPlanPath); os.IsNotExist(err) {
		// Skip directory - not a unit
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error checking IMPLEMENTATION_PLAN.md in %s: %w", unitDir, err)
	}

	// Glob for task files
	taskFiles, err := discoverTaskFiles(unitDir)
	if err != nil {
		return nil, err
	}
	if len(taskFiles) == 0 {
		// Skip directory - no tasks
		return nil, nil
	}

	// Parse IMPLEMENTATION_PLAN.md
	implContent, err := os.ReadFile(implPlanPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", implPlanPath, err)
	}

	frontmatter, _, err := ParseFrontmatter(implContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing frontmatter in %s: %w", implPlanPath, err)
	}

	unitFrontmatter, err := ParseUnitFrontmatter(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("error parsing unit frontmatter in %s: %w", implPlanPath, err)
	}

	// Create Unit
	unit := &Unit{
		ID:        filepath.Base(unitDir),
		Path:      unitDir,
		DependsOn: unitFrontmatter.DependsOn,
		Branch:    unitFrontmatter.OrchBranch,
		Worktree:  unitFrontmatter.OrchWorktree,
		PRNumber:  unitFrontmatter.OrchPRNumber,
	}

	// Parse orchestrator status
	if unitFrontmatter.OrchStatus != "" {
		status, err := parseUnitStatus(unitFrontmatter.OrchStatus)
		if err != nil {
			return nil, fmt.Errorf("error in %s: %w", implPlanPath, err)
		}
		unit.Status = status
	} else {
		unit.Status = UnitStatusPending
	}

	// Parse orchestrator timestamps
	if unitFrontmatter.OrchStartedAt != "" {
		t, err := time.Parse(time.RFC3339, unitFrontmatter.OrchStartedAt)
		if err != nil {
			return nil, fmt.Errorf("error parsing orch_started_at in %s: %w", implPlanPath, err)
		}
		unit.StartedAt = &t
	}

	if unitFrontmatter.OrchCompletedAt != "" {
		t, err := time.Parse(time.RFC3339, unitFrontmatter.OrchCompletedAt)
		if err != nil {
			return nil, fmt.Errorf("error parsing orch_completed_at in %s: %w", implPlanPath, err)
		}
		unit.CompletedAt = &t
	}

	// For each task file (sorted by filename)
	for _, taskFile := range taskFiles {
		taskPath := filepath.Join(unitDir, taskFile)
		taskContent, err := os.ReadFile(taskPath)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", taskPath, err)
		}

		frontmatter, body, err := ParseFrontmatter(taskContent)
		if err != nil {
			return nil, fmt.Errorf("error parsing frontmatter in %s: %w", taskPath, err)
		}

		taskFrontmatter, err := ParseTaskFrontmatter(frontmatter)
		if err != nil {
			return nil, fmt.Errorf("error parsing task frontmatter in %s: %w", taskPath, err)
		}

		// Parse task status
		status, err := parseTaskStatus(taskFrontmatter.Status)
		if err != nil {
			return nil, fmt.Errorf("error in %s: %w", taskPath, err)
		}

		// Extract title
		title := extractTitle(body)

		task := &Task{
			Number:       taskFrontmatter.Task,
			Status:       status,
			Backpressure: taskFrontmatter.Backpressure,
			DependsOn:    taskFrontmatter.DependsOn,
			FilePath:     taskFile,
			Title:        title,
			Content:      string(taskContent),
		}

		unit.Tasks = append(unit.Tasks, task)
	}

	return unit, nil
}

// discoverTaskFiles finds all task files matching [0-9][0-9]-*.md pattern
func discoverTaskFiles(unitDir string) ([]string, error) {
	pattern := filepath.Join(unitDir, "[0-9][0-9]-*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("error globbing task files in %s: %w", unitDir, err)
	}

	// Convert to relative paths (basenames only)
	var taskFiles []string
	for _, match := range matches {
		taskFiles = append(taskFiles, filepath.Base(match))
	}

	// Sort for deterministic order
	sort.Strings(taskFiles)

	return taskFiles, nil
}

// discoverUnitDirs finds all subdirectories of tasksDir
func discoverUnitDirs(tasksDir string) ([]string, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("error reading tasks directory %s: %w", tasksDir, err)
	}

	var unitDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			unitDirs = append(unitDirs, filepath.Join(tasksDir, entry.Name()))
		}
	}

	return unitDirs, nil
}

// ParseTaskFile parses a single task file and returns the task
func ParseTaskFile(taskPath string) (*Task, error) {
	taskContent, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", taskPath, err)
	}

	frontmatter, body, err := ParseFrontmatter(taskContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing frontmatter in %s: %w", taskPath, err)
	}

	taskFrontmatter, err := ParseTaskFrontmatter(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("error parsing task frontmatter in %s: %w", taskPath, err)
	}

	// Parse task status
	status, err := parseTaskStatus(taskFrontmatter.Status)
	if err != nil {
		return nil, fmt.Errorf("error in %s: %w", taskPath, err)
	}

	// Extract title
	title := extractTitle(body)

	task := &Task{
		Number:       taskFrontmatter.Task,
		Status:       status,
		Backpressure: taskFrontmatter.Backpressure,
		DependsOn:    taskFrontmatter.DependsOn,
		FilePath:     taskPath,
		Title:        title,
		Content:      string(taskContent),
	}

	return task, nil
}
