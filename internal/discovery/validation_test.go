package discovery

import (
	"strings"
	"testing"
)

func TestValidateUnit_MissingUnitField(t *testing.T) {
	unit := &Unit{
		ID: "", // Missing unit field
		Tasks: []*Task{
			{Number: 1, Backpressure: "echo test", FilePath: "01-test.md"},
		},
	}

	result := ValidateUnit(unit)

	if result.IsValid() {
		t.Fatal("expected validation error for missing unit field")
	}

	// Check that error mentions the unit field
	found := false
	for _, err := range result.Errors {
		if err.Field == "unit" && strings.Contains(err.Message, "unit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about missing 'unit' field, got: %v", result.Errors)
	}
}

func TestValidateUnit_MissingTaskField(t *testing.T) {
	unit := &Unit{
		ID: "test-unit",
		Tasks: []*Task{
			{Number: 0, Backpressure: "echo test", FilePath: "01-test.md"}, // Missing task number
		},
	}

	result := ValidateUnit(unit)

	if result.IsValid() {
		t.Fatal("expected validation error for missing task field")
	}

	// Check that error mentions the task field
	found := false
	for _, err := range result.Errors {
		if err.Field == "task" && strings.Contains(err.Message, "task") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about missing 'task' field, got: %v", result.Errors)
	}
}

func TestValidateUnit_MissingBackpressure(t *testing.T) {
	unit := &Unit{
		ID: "test-unit",
		Tasks: []*Task{
			{Number: 1, Backpressure: "", FilePath: "01-test.md"}, // Missing backpressure
		},
	}

	result := ValidateUnit(unit)

	if result.IsValid() {
		t.Fatal("expected validation error for missing backpressure")
	}

	// Check that error mentions backpressure
	found := false
	for _, err := range result.Errors {
		if err.Field == "backpressure" && strings.Contains(err.Message, "backpressure") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about missing 'backpressure' field, got: %v", result.Errors)
	}
}

func TestValidateTaskSequence_Valid(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md"},
		{Number: 2, FilePath: "02-b.md"},
		{Number: 3, FilePath: "03-c.md"},
	}

	result := ValidateTaskSequence(tasks)

	if !result.IsValid() {
		t.Errorf("expected no errors for valid sequence, got: %v", result.Errors)
	}
}

func TestValidateTaskSequence_StartsFrom2(t *testing.T) {
	tasks := []*Task{
		{Number: 2, FilePath: "02-a.md"},
		{Number: 3, FilePath: "03-b.md"},
	}

	result := ValidateTaskSequence(tasks)

	if result.IsValid() {
		t.Fatal("expected validation error for sequence starting at 2")
	}

	// Check that error mentions starting at 1
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "start at 1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about sequence starting at 1, got: %v", result.Errors)
	}
}

func TestValidateTaskSequence_Gap(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md"},
		{Number: 3, FilePath: "03-c.md"},
	}

	result := ValidateTaskSequence(tasks)

	if result.IsValid() {
		t.Fatal("expected validation error for gap in sequence")
	}

	// Check that error mentions missing task 2
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "gap") && strings.Contains(err.Message, "2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about gap and missing task 2, got: %v", result.Errors)
	}
}

func TestValidateTaskSequence_Duplicate(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md"},
		{Number: 1, FilePath: "01-b.md"},
		{Number: 2, FilePath: "02-c.md"},
	}

	result := ValidateTaskSequence(tasks)

	if result.IsValid() {
		t.Fatal("expected validation error for duplicate task numbers")
	}

	// Check that error mentions duplicate
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about duplicate task number, got: %v", result.Errors)
	}
}

func TestValidateTaskDependencies_Valid(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md", DependsOn: []int{}},
		{Number: 2, FilePath: "02-b.md", DependsOn: []int{1}},
		{Number: 3, FilePath: "03-c.md", DependsOn: []int{1, 2}},
	}

	result := ValidateTaskDependencies(tasks)

	if !result.IsValid() {
		t.Errorf("expected no errors for valid task dependencies, got: %v", result.Errors)
	}
}

func TestValidateTaskDependencies_InvalidRef(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md", DependsOn: []int{}},
		{Number: 2, FilePath: "02-b.md", DependsOn: []int{5}}, // Task 5 doesn't exist
	}

	result := ValidateTaskDependencies(tasks)

	if result.IsValid() {
		t.Fatal("expected validation error for invalid task reference")
	}

	// Check that error mentions non-existent task
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "non-existent") && strings.Contains(err.Message, "5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about non-existent task 5, got: %v", result.Errors)
	}
}

func TestValidateTaskDependencies_SelfRef(t *testing.T) {
	tasks := []*Task{
		{Number: 1, FilePath: "01-a.md", DependsOn: []int{1}}, // Self-reference
	}

	result := ValidateTaskDependencies(tasks)

	if result.IsValid() {
		t.Fatal("expected validation error for self-referencing task")
	}

	// Check that error mentions self-dependency
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "itself") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about task depending on itself, got: %v", result.Errors)
	}
}

func TestValidateUnitDependencies_Valid(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}

	result := ValidateUnitDependencies(units)

	if !result.IsValid() {
		t.Errorf("expected no errors for valid unit dependencies, got: %v", result.Errors)
	}
}

func TestValidateUnitDependencies_Missing(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{"x"}}, // Unit "x" doesn't exist
		{ID: "b", DependsOn: []string{"a"}},
	}

	result := ValidateUnitDependencies(units)

	if result.IsValid() {
		t.Fatal("expected validation error for missing unit dependency")
	}

	// Check that error mentions non-existent unit
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "non-existent") && strings.Contains(err.Message, "x") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about non-existent unit x, got: %v", result.Errors)
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	result := DetectCycles(units)

	if !result.IsValid() {
		t.Errorf("expected no errors for linear chain, got: %v", result.Errors)
	}
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	result := DetectCycles(units)

	if result.IsValid() {
		t.Fatal("expected validation error for simple cycle")
	}

	// Check that error mentions circular dependency
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "circular") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about circular dependency, got: %v", result.Errors)
	}
}

func TestDetectCycles_ThreeNodeCycle(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"c"}},
		{ID: "c", DependsOn: []string{"a"}},
	}

	result := DetectCycles(units)

	if result.IsValid() {
		t.Fatal("expected validation error for three-node cycle")
	}

	// Check that error mentions circular dependency
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "circular") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about circular dependency, got: %v", result.Errors)
	}
}

func TestDetectCycles_CycleMessage(t *testing.T) {
	units := []*Unit{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	result := DetectCycles(units)

	if result.IsValid() {
		t.Fatal("expected validation error for cycle")
	}

	// Check that error message shows the cycle path (a -> b -> a or b -> a -> b)
	errorMsg := result.Error()
	hasArrows := strings.Contains(errorMsg, "->")
	if !hasArrows {
		t.Errorf("expected error message to show cycle path with arrows, got: %s", errorMsg)
	}
}

func TestValidationResult_Error(t *testing.T) {
	result := &ValidationResult{}

	taskNum1 := 1
	taskNum2 := 2

	result.Add(ValidationError{
		Unit:    "app-shell",
		Task:    &taskNum1,
		File:    "01-nav.md",
		Field:   "backpressure",
		Message: "missing required 'backpressure' field",
	})

	result.Add(ValidationError{
		Unit:    "deck-list",
		Task:    &taskNum2,
		Message: "task sequence gap, missing task 2",
	})

	result.Add(ValidationError{
		Message: "circular dependency: app-shell -> deck-list -> app-shell",
	})

	errorMsg := result.Error()

	// Check format
	if !strings.Contains(errorMsg, "validation failed with 3 errors:") {
		t.Errorf("expected header with error count, got: %s", errorMsg)
	}

	// Check that all errors are present
	if !strings.Contains(errorMsg, "app-shell") {
		t.Errorf("expected error message to contain 'app-shell', got: %s", errorMsg)
	}

	if !strings.Contains(errorMsg, "backpressure") {
		t.Errorf("expected error message to contain 'backpressure', got: %s", errorMsg)
	}

	if !strings.Contains(errorMsg, "circular dependency") {
		t.Errorf("expected error message to contain 'circular dependency', got: %s", errorMsg)
	}
}

func TestValidationResult_IsValid(t *testing.T) {
	result := &ValidationResult{}

	if !result.IsValid() {
		t.Error("expected IsValid to return true for empty errors")
	}

	result.Add(ValidationError{Message: "some error"})

	if result.IsValid() {
		t.Error("expected IsValid to return false when errors present")
	}
}
