package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestArchive_MovesCompletedSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	completeSpec := `---
status: complete
---
# Complete Spec`
	if err := os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Verify complete spec moved
	completedDir := filepath.Join(specsDir, "completed")
	if _, err := os.Stat(filepath.Join(completedDir, "COMPLETE.md")); os.IsNotExist(err) {
		t.Error("COMPLETE.md should be in completed directory")
	}
	if _, err := os.Stat(filepath.Join(specsDir, "COMPLETE.md")); !os.IsNotExist(err) {
		t.Error("COMPLETE.md should not be in specs directory")
	}

	if len(archived) != 1 || archived[0] != "COMPLETE.md" {
		t.Errorf("archived = %v, want [COMPLETE.md]", archived)
	}
}

func TestArchive_SkipsIncompleteSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	incompleteSpec := `---
status: in_progress
---
# Incomplete Spec`
	if err := os.WriteFile(filepath.Join(specsDir, "INCOMPLETE.md"), []byte(incompleteSpec), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Verify incomplete spec not moved
	if _, err := os.Stat(filepath.Join(specsDir, "INCOMPLETE.md")); os.IsNotExist(err) {
		t.Error("INCOMPLETE.md should remain in specs directory")
	}

	if len(archived) != 0 {
		t.Errorf("archived = %v, want []", archived)
	}
}

func TestArchive_SkipsNoFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	content := `# Spec Without Frontmatter`
	if err := os.WriteFile(filepath.Join(specsDir, "NOFM.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(specsDir, "NOFM.md")); os.IsNotExist(err) {
		t.Error("NOFM.md should remain in specs directory")
	}

	if len(archived) != 0 {
		t.Errorf("archived = %v, want []", archived)
	}
}

func TestArchive_SkipsReadme(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	readme := `---
status: complete
---
# README`
	if err := os.WriteFile(filepath.Join(specsDir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Verify README not moved
	if _, err := os.Stat(filepath.Join(specsDir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md should remain in specs directory")
	}

	if len(archived) != 0 {
		t.Errorf("archived = %v, want []", archived)
	}
}

func TestArchive_CreatesCompletedDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	_, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(specsDir, "completed")); err != nil {
		t.Fatalf("expected completed directory to exist: %v", err)
	}
}

func TestArchive_DryRunNoMove(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	completeSpec := `---
status: complete
---
# Complete Spec`
	if err := os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir, DryRun: true})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Verify file not actually moved
	if _, err := os.Stat(filepath.Join(specsDir, "COMPLETE.md")); os.IsNotExist(err) {
		t.Error("COMPLETE.md should remain in specs directory during dry run")
	}

	// But it should be reported as archived
	if len(archived) != 1 {
		t.Errorf("archived = %v, want [COMPLETE.md]", archived)
	}
}

func TestArchive_ReturnsArchivedList(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	completeOne := `---
status: complete
---
# Complete Spec 1`
	completeTwo := `---
status: complete
---
# Complete Spec 2`
	if err := os.WriteFile(filepath.Join(specsDir, "COMPLETE_ONE.md"), []byte(completeOne), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "COMPLETE_TWO.md"), []byte(completeTwo), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	archived, err := Archive(ArchiveOptions{SpecsDir: specsDir})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	sort.Strings(archived)
	want := []string{"COMPLETE_ONE.md", "COMPLETE_TWO.md"}
	if len(archived) != len(want) {
		t.Fatalf("archived = %v, want %v", archived, want)
	}
	for i := range want {
		if archived[i] != want[i] {
			t.Fatalf("archived = %v, want %v", archived, want)
		}
	}
}

func TestShouldArchive_CompleteStatus(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")

	content := `---
status: complete
---
# Test`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	if !shouldArchive(path) {
		t.Error("shouldArchive() = false, want true for status: complete")
	}
}

func TestShouldArchive_PendingStatus(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")

	content := `---
status: pending
---
# Test`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	if shouldArchive(path) {
		t.Error("shouldArchive() = true, want false for status: pending")
	}
}

func TestShouldArchive_NoFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")

	content := `# Test without frontmatter`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	if shouldArchive(path) {
		t.Error("shouldArchive() = true, want false for no frontmatter")
	}
}

func TestIsUnitComplete_AllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "test-unit")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	plan := `---
unit: test-unit
---
# Plan`
	task := `---
task: 1
status: complete
---
# Task`

	if err := os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(plan), 0644); err != nil {
		t.Fatalf("failed to write plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(task), 0644); err != nil {
		t.Fatalf("failed to write task: %v", err)
	}

	if !isUnitComplete(unitDir) {
		t.Error("isUnitComplete() = false, want true when all tasks complete")
	}
}

func TestIsUnitComplete_SomePending(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "test-unit")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("failed to create unit dir: %v", err)
	}

	completeTask := `---
task: 1
status: complete
---
# Task 1`
	pendingTask := `---
task: 2
status: pending
---
# Task 2`

	if err := os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(completeTask), 0644); err != nil {
		t.Fatalf("failed to write task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, "02-task.md"), []byte(pendingTask), 0644); err != nil {
		t.Fatalf("failed to write task: %v", err)
	}

	if isUnitComplete(unitDir) {
		t.Error("isUnitComplete() = true, want false when some tasks pending")
	}
}
