package discovery

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDir creates a test directory structure for discovery tests
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")

	// Create app-shell unit with tasks
	appShellDir := filepath.Join(tasksDir, "app-shell")
	if err := os.MkdirAll(appShellDir, 0755); err != nil {
		t.Fatalf("failed to create app-shell dir: %v", err)
	}

	implPlan := `---
unit: app-shell
depends_on: [core]
orch_status: in_progress
orch_branch: feature/app-shell
orch_worktree: /tmp/worktree
orch_pr_number: 42
orch_started_at: 2024-01-15T10:00:00Z
orch_completed_at: 2024-01-15T12:00:00Z
---

# App Shell Implementation Plan

This is the implementation plan.
`
	if err := os.WriteFile(filepath.Join(appShellDir, "IMPLEMENTATION_PLAN.md"), []byte(implPlan), 0644); err != nil {
		t.Fatalf("failed to write IMPLEMENTATION_PLAN.md: %v", err)
	}

	task01 := `---
task: 1
status: complete
backpressure: go test ./...
depends_on: []
---

# Nav Types

Task content here.
`
	if err := os.WriteFile(filepath.Join(appShellDir, "01-nav-types.md"), []byte(task01), 0644); err != nil {
		t.Fatalf("failed to write 01-nav-types.md: %v", err)
	}

	task02 := `---
task: 2
status: in_progress
backpressure: npm run build
depends_on: [1]
---

# Navigation

Navigation implementation.
`
	if err := os.WriteFile(filepath.Join(appShellDir, "02-navigation.md"), []byte(task02), 0644); err != nil {
		t.Fatalf("failed to write 02-navigation.md: %v", err)
	}

	// Create deck-list unit with one task
	deckListDir := filepath.Join(tasksDir, "deck-list")
	if err := os.MkdirAll(deckListDir, 0755); err != nil {
		t.Fatalf("failed to create deck-list dir: %v", err)
	}

	deckImplPlan := `---
unit: deck-list
---

# Deck List Implementation
`
	if err := os.WriteFile(filepath.Join(deckListDir, "IMPLEMENTATION_PLAN.md"), []byte(deckImplPlan), 0644); err != nil {
		t.Fatalf("failed to write deck-list IMPLEMENTATION_PLAN.md: %v", err)
	}

	deckTask01 := `---
task: 1
status: pending
backpressure: go test ./deck/...
---

# Deck Card

Deck card component.
`
	if err := os.WriteFile(filepath.Join(deckListDir, "01-deck-card.md"), []byte(deckTask01), 0644); err != nil {
		t.Fatalf("failed to write deck-list 01-deck-card.md: %v", err)
	}

	// Create no-impl-plan directory (should be skipped)
	noImplPlanDir := filepath.Join(tasksDir, "no-impl-plan")
	if err := os.MkdirAll(noImplPlanDir, 0755); err != nil {
		t.Fatalf("failed to create no-impl-plan dir: %v", err)
	}

	noImplTask := `---
task: 1
status: pending
backpressure: echo test
---

# Task Without Impl Plan
`
	if err := os.WriteFile(filepath.Join(noImplPlanDir, "01-task.md"), []byte(noImplTask), 0644); err != nil {
		t.Fatalf("failed to write no-impl-plan task: %v", err)
	}

	// Create no-tasks directory (should be skipped)
	noTasksDir := filepath.Join(tasksDir, "no-tasks")
	if err := os.MkdirAll(noTasksDir, 0755); err != nil {
		t.Fatalf("failed to create no-tasks dir: %v", err)
	}

	noTasksImplPlan := `---
unit: no-tasks
---

# No Tasks Plan
`
	if err := os.WriteFile(filepath.Join(noTasksDir, "IMPLEMENTATION_PLAN.md"), []byte(noTasksImplPlan), 0644); err != nil {
		t.Fatalf("failed to write no-tasks IMPLEMENTATION_PLAN.md: %v", err)
	}

	return tasksDir
}

func TestDiscover_SingleUnit(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "specs", "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	implPlan := `---
unit: test-unit
---

# Test Unit
`
	if err := os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(implPlan), 0644); err != nil {
		t.Fatalf("failed to write IMPLEMENTATION_PLAN.md: %v", err)
	}

	task01 := `---
task: 1
status: pending
backpressure: go test
---

# Test Task
`
	if err := os.WriteFile(filepath.Join(unitDir, "01-test.md"), []byte(task01), 0644); err != nil {
		t.Fatalf("failed to write task: %v", err)
	}

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}

	unit := units[0]
	if unit.ID != "test-unit" {
		t.Errorf("expected unit ID 'test-unit', got %q", unit.ID)
	}

	if len(unit.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(unit.Tasks))
	}
}

func TestDiscover_MultipleUnits(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Should discover app-shell and deck-list (2 units)
	// Should skip no-impl-plan and no-tasks
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}

	// Units might not be in any particular order, so check both exist
	unitIDs := make(map[string]bool)
	for _, unit := range units {
		unitIDs[unit.ID] = true
	}

	if !unitIDs["app-shell"] {
		t.Error("expected to find app-shell unit")
	}
	if !unitIDs["deck-list"] {
		t.Error("expected to find deck-list unit")
	}
}

func TestDiscover_SkipsNoImplPlan(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Verify no-impl-plan was skipped
	for _, unit := range units {
		if unit.ID == "no-impl-plan" {
			t.Error("should not have discovered no-impl-plan unit")
		}
	}
}

func TestDiscover_SkipsNoTasks(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Verify no-tasks was skipped
	for _, unit := range units {
		if unit.ID == "no-tasks" {
			t.Error("should not have discovered no-tasks unit")
		}
	}
}

func TestDiscover_TaskOrdering(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Find app-shell unit
	var appShell *Unit
	for _, unit := range units {
		if unit.ID == "app-shell" {
			appShell = unit
			break
		}
	}

	if appShell == nil {
		t.Fatal("app-shell unit not found")
	}

	if len(appShell.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(appShell.Tasks))
	}

	// Verify ordering by filename
	if appShell.Tasks[0].FilePath != "01-nav-types.md" {
		t.Errorf("expected first task to be 01-nav-types.md, got %s", appShell.Tasks[0].FilePath)
	}
	if appShell.Tasks[1].FilePath != "02-navigation.md" {
		t.Errorf("expected second task to be 02-navigation.md, got %s", appShell.Tasks[1].FilePath)
	}
}

func TestDiscover_TaskContent(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Find app-shell unit
	var appShell *Unit
	for _, unit := range units {
		if unit.ID == "app-shell" {
			appShell = unit
			break
		}
	}

	if appShell == nil {
		t.Fatal("app-shell unit not found")
	}

	// Check that Content contains the full file content
	task := appShell.Tasks[0]
	if task.Content == "" {
		t.Error("task Content should not be empty")
	}
	// Content should include frontmatter
	if len(task.Content) < 50 {
		t.Error("task Content seems too short, should contain frontmatter and body")
	}
}

func TestDiscover_TaskTitle(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Find app-shell unit
	var appShell *Unit
	for _, unit := range units {
		if unit.ID == "app-shell" {
			appShell = unit
			break
		}
	}

	if appShell == nil {
		t.Fatal("app-shell unit not found")
	}

	// Check task titles are extracted from H1
	if appShell.Tasks[0].Title != "Nav Types" {
		t.Errorf("expected title 'Nav Types', got %q", appShell.Tasks[0].Title)
	}
	if appShell.Tasks[1].Title != "Navigation" {
		t.Errorf("expected title 'Navigation', got %q", appShell.Tasks[1].Title)
	}
}

func TestDiscover_UnitDependsOn(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Find app-shell unit
	var appShell *Unit
	for _, unit := range units {
		if unit.ID == "app-shell" {
			appShell = unit
			break
		}
	}

	if appShell == nil {
		t.Fatal("app-shell unit not found")
	}

	// Check DependsOn is populated
	if len(appShell.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(appShell.DependsOn))
	}
	if appShell.DependsOn[0] != "core" {
		t.Errorf("expected dependency 'core', got %q", appShell.DependsOn[0])
	}
}

func TestDiscover_OrchFields(t *testing.T) {
	tasksDir := setupTestDir(t)

	units, err := Discover(tasksDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Find app-shell unit
	var appShell *Unit
	for _, unit := range units {
		if unit.ID == "app-shell" {
			appShell = unit
			break
		}
	}

	if appShell == nil {
		t.Fatal("app-shell unit not found")
	}

	// Check orchestrator fields
	if appShell.Status != UnitStatusInProgress {
		t.Errorf("expected status in_progress, got %v", appShell.Status)
	}
	if appShell.Branch != "feature/app-shell" {
		t.Errorf("expected branch 'feature/app-shell', got %q", appShell.Branch)
	}
	if appShell.Worktree != "/tmp/worktree" {
		t.Errorf("expected worktree '/tmp/worktree', got %q", appShell.Worktree)
	}
	if appShell.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", appShell.PRNumber)
	}

	// Check timestamps
	if appShell.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	} else {
		expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		if !appShell.StartedAt.Equal(expectedStart) {
			t.Errorf("expected StartedAt %v, got %v", expectedStart, *appShell.StartedAt)
		}
	}

	if appShell.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	} else {
		expectedComplete := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		if !appShell.CompletedAt.Equal(expectedComplete) {
			t.Errorf("expected CompletedAt %v, got %v", expectedComplete, *appShell.CompletedAt)
		}
	}
}

func TestDiscoverTaskFiles_Pattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"01-valid.md",
		"02-also-valid.md",
		"99-max.md",
		"invalid.md",        // Should not match
		"1-single-digit.md", // Should not match
		"001-triple.md",     // Should not match
		"README.md",         // Should not match
	}

	for _, file := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	files, err := discoverTaskFiles(tmpDir)
	if err != nil {
		t.Fatalf("discoverTaskFiles failed: %v", err)
	}

	// Should only match [0-9][0-9]-*.md pattern
	expectedCount := 3
	if len(files) != expectedCount {
		t.Errorf("expected %d files, got %d: %v", expectedCount, len(files), files)
	}

	expectedFiles := map[string]bool{
		"01-valid.md":      true,
		"02-also-valid.md": true,
		"99-max.md":        true,
	}

	for _, file := range files {
		if !expectedFiles[file] {
			t.Errorf("unexpected file matched: %s", file)
		}
	}
}

func TestDiscoverTaskFiles_Sorting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in non-sorted order
	testFiles := []string{
		"03-third.md",
		"01-first.md",
		"02-second.md",
	}

	for _, file := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	files, err := discoverTaskFiles(tmpDir)
	if err != nil {
		t.Fatalf("discoverTaskFiles failed: %v", err)
	}

	// Should be sorted
	expected := []string{"01-first.md", "02-second.md", "03-third.md"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d", len(expected), len(files))
	}

	for i, file := range files {
		if file != expected[i] {
			t.Errorf("expected files[%d] = %s, got %s", i, expected[i], file)
		}
	}
}

func TestDiscoverUnit_NotExists(t *testing.T) {
	unit, err := DiscoverUnit("/nonexistent/path/to/unit")
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
	if unit != nil {
		t.Error("expected nil unit for non-existent directory")
	}
}
