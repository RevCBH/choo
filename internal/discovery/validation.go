package discovery

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation failure
type ValidationError struct {
	Unit    string // unit ID (empty for cross-unit errors)
	Task    *int   // task number (nil for unit-level errors)
	File    string // file path where error occurred
	Field   string // field name that failed validation
	Message string // human-readable error description
}

// ValidationResult collects all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

// IsValid returns true if no validation errors occurred
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// HasWarnings returns true if any warnings were recorded.
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// Error returns a formatted string of all validation errors
func (r *ValidationResult) Error() string {
	if r.IsValid() {
		return ""
	}

	var lines []string
	count := len(r.Errors)
	if count == 1 {
		lines = append(lines, "validation failed with 1 error:")
	} else {
		lines = append(lines, fmt.Sprintf("validation failed with %d errors:", count))
	}

	for _, err := range r.Errors {
		var parts []string

		// Build error message components
		if err.Unit != "" {
			parts = append(parts, fmt.Sprintf("unit %q", err.Unit))
		}
		if err.Task != nil {
			parts = append(parts, fmt.Sprintf("task %d", *err.Task))
		}
		if err.File != "" {
			parts = append(parts, fmt.Sprintf("file %q", err.File))
		}

		var prefix string
		if len(parts) > 0 {
			prefix = strings.Join(parts, ", ") + ": "
		}

		lines = append(lines, fmt.Sprintf("  - %s%s", prefix, err.Message))
	}

	return strings.Join(lines, "\n")
}

// Add appends an error to the result
func (r *ValidationResult) Add(err ValidationError) {
	r.Errors = append(r.Errors, err)
}

// AddWarning appends a warning to the result
func (r *ValidationResult) AddWarning(warn ValidationError) {
	r.Warnings = append(r.Warnings, warn)
}

// Merge combines another ValidationResult into this one
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other != nil {
		r.Errors = append(r.Errors, other.Errors...)
		r.Warnings = append(r.Warnings, other.Warnings...)
	}
}

// ValidateUnit validates a single unit's structure
func ValidateUnit(unit *Unit) *ValidationResult {
	result := &ValidationResult{}

	// Check unit field
	if unit.ID == "" {
		result.Add(ValidationError{
			Unit:    unit.ID,
			File:    "IMPLEMENTATION_PLAN.md",
			Field:   "unit",
			Message: "missing required 'unit' field",
		})
	}

	// Check each task
	for _, task := range unit.Tasks {
		// Validate task number
		if task.Number == 0 {
			taskNum := task.Number
			result.Add(ValidationError{
				Unit:    unit.ID,
				Task:    &taskNum,
				File:    task.FilePath,
				Field:   "task",
				Message: "missing required 'task' field",
			})
		}

		// Validate backpressure
		if task.Backpressure == "" {
			taskNum := task.Number
			result.Add(ValidationError{
				Unit:    unit.ID,
				Task:    &taskNum,
				File:    task.FilePath,
				Field:   "backpressure",
				Message: "missing required 'backpressure' field",
			})
		}
	}

	// Validate task sequence
	result.Merge(ValidateTaskSequence(unit.Tasks))

	// Validate task dependencies
	result.Merge(ValidateTaskDependencies(unit.Tasks))

	return result
}

// ValidateUnits validates all units including cross-unit dependencies
func ValidateUnits(units []*Unit) *ValidationResult {
	result := &ValidationResult{}

	// Validate each unit individually
	for _, unit := range units {
		result.Merge(ValidateUnit(unit))
	}

	// Validate unit dependencies
	result.Merge(ValidateUnitDependencies(units))

	// Detect cycles
	result.Merge(DetectCycles(units))

	return result
}

// ValidateTaskSequence ensures task numbers are sequential from 1
func ValidateTaskSequence(tasks []*Task) *ValidationResult {
	result := &ValidationResult{}

	if len(tasks) == 0 {
		return result
	}

	// Track seen task numbers to detect duplicates
	seen := make(map[int]bool)
	for _, task := range tasks {
		if seen[task.Number] {
			result.Add(ValidationError{
				Message: fmt.Sprintf("duplicate task number %d", task.Number),
			})
		}
		seen[task.Number] = true
	}

	// Check if sequence starts at 1
	if !seen[1] {
		result.Add(ValidationError{
			Message: "task sequence must start at 1",
		})
		return result
	}

	// Check for gaps in sequence
	expectedNext := 1
	for expectedNext <= len(tasks) {
		if !seen[expectedNext] {
			result.Add(ValidationError{
				Message: fmt.Sprintf("task sequence gap, missing task %d", expectedNext),
			})
			break
		}
		expectedNext++
	}

	return result
}

// ValidateTaskDependencies ensures task depends_on references are valid
func ValidateTaskDependencies(tasks []*Task) *ValidationResult {
	result := &ValidationResult{}

	// Build set of valid task numbers
	validTasks := make(map[int]bool)
	for _, task := range tasks {
		validTasks[task.Number] = true
	}

	// Validate each task's dependencies
	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			// Check self-reference
			if dep == task.Number {
				taskNum := task.Number
				result.Add(ValidationError{
					Task:    &taskNum,
					File:    task.FilePath,
					Field:   "depends_on",
					Message: "task cannot depend on itself",
				})
				continue
			}

			// Check if referenced task exists
			if !validTasks[dep] {
				taskNum := task.Number
				result.Add(ValidationError{
					Task:    &taskNum,
					File:    task.FilePath,
					Field:   "depends_on",
					Message: fmt.Sprintf("depends_on references non-existent task %d", dep),
				})
			}
		}
	}

	return result
}

// ValidateUnitDependencies ensures unit depends_on references exist
func ValidateUnitDependencies(units []*Unit) *ValidationResult {
	result := &ValidationResult{}

	// Build set of valid unit IDs
	validUnits := make(map[string]bool)
	for _, unit := range units {
		validUnits[unit.ID] = true
	}

	for _, unit := range units {
		for _, dep := range unit.DependsOn {
			if !validUnits[dep] {
				result.AddWarning(ValidationError{
					Unit:    unit.ID,
					Field:   "depends_on",
					Message: fmt.Sprintf("depends_on references missing unit %q (ignored)", dep),
				})
			}
		}
	}

	return result
}

// DetectCycles checks for circular dependencies in the unit graph
func DetectCycles(units []*Unit) *ValidationResult {
	result := &ValidationResult{}

	// Build adjacency list
	graph := make(map[string][]string)
	for _, unit := range units {
		graph[unit.ID] = unit.DependsOn
	}

	// Track visit states: 0 = unvisited, 1 = visiting, 2 = visited
	state := make(map[string]int)

	// DFS function that returns true if cycle detected
	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		if state[node] == 1 {
			// Found a cycle - build the cycle path
			cyclePath := append(path, node)
			result.Add(ValidationError{
				Message: fmt.Sprintf("circular dependency: %s", strings.Join(cyclePath, " -> ")),
			})
			return true
		}
		if state[node] == 2 {
			// Already fully explored
			return false
		}

		// Mark as visiting
		state[node] = 1
		path = append(path, node)

		// Visit neighbors
		for _, neighbor := range graph[node] {
			if dfs(neighbor, path) {
				return true
			}
		}

		// Mark as visited
		state[node] = 2
		return false
	}

	// Check each unit as potential starting point
	for _, unit := range units {
		if state[unit.ID] == 0 {
			dfs(unit.ID, []string{})
		}
	}

	return result
}
