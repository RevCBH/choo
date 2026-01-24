package specs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize_TaskMetadataBlockToFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "specs", "tasks", "unit-a")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	taskPath := filepath.Join(unitDir, "01-test.md")
	taskContent := "# Intro\n\n## Metadata\n```yaml\ntask: 1\nbackpressure: go test ./...\n```\n\n# Task Title\n"
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	report, err := Normalize(NormalizeOptions{
		TasksDir: filepath.Join(tmpDir, "specs", "tasks"),
		RepoRoot: tmpDir,
		Apply:    true,
	})
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("expected no errors, got %d", len(report.Errors))
	}
	if !report.HasChanges() {
		t.Fatalf("expected normalization to apply")
	}

	updated, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read updated task: %v", err)
	}
	if !strings.HasPrefix(string(updated), "---\n") {
		t.Fatalf("expected frontmatter at file start")
	}
	if strings.Contains(string(updated), "## Metadata") {
		t.Fatalf("metadata block should be removed")
	}
}

func TestNormalize_FrontmatterRemovesMetadataBlock(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "specs", "tasks", "unit-b")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	taskPath := filepath.Join(unitDir, "01-test.md")
	taskContent := "---\ntask: 1\nbackpressure: go test ./...\n---\n\n# Task Title\n\n## Metadata\n```yaml\ntask: 2\nbackpressure: nope\n```\n"
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	report, err := Normalize(NormalizeOptions{
		TasksDir: filepath.Join(tmpDir, "specs", "tasks"),
		RepoRoot: tmpDir,
		Apply:    true,
	})
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("expected no errors, got %d", len(report.Errors))
	}

	updated, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read updated task: %v", err)
	}
	if strings.Contains(string(updated), "## Metadata") {
		t.Fatalf("metadata block should be removed")
	}
}

func TestNormalize_InvalidMetadataBlockReportsError(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "specs", "tasks", "unit-c")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	taskPath := filepath.Join(unitDir, "01-test.md")
	taskContent := "# Task\n\n## Metadata\n```yaml\ntask: 1\nbackpressure: go test ./...\n"
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	report, err := Normalize(NormalizeOptions{
		TasksDir: filepath.Join(tmpDir, "specs", "tasks"),
		RepoRoot: tmpDir,
		Apply:    false,
	})
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if len(report.Errors) == 0 {
		t.Fatalf("expected errors for invalid metadata block")
	}
}

func TestNormalize_UnitPlanMetadataBlock(t *testing.T) {
	tmpDir := t.TempDir()
	unitDir := filepath.Join(tmpDir, "specs", "tasks", "unit-d")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	planPath := filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md")
	planContent := "# Plan\n\n## Metadata\n```yaml\nunit: unit-d\n```\n\n# Body\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	report, err := Normalize(NormalizeOptions{
		TasksDir: filepath.Join(tmpDir, "specs", "tasks"),
		RepoRoot: tmpDir,
		Apply:    true,
	})
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("expected no errors, got %d", len(report.Errors))
	}

	updated, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read updated plan: %v", err)
	}
	if !strings.HasPrefix(string(updated), "---\n") {
		t.Fatalf("expected frontmatter at file start")
	}
	if strings.Contains(string(updated), "## Metadata") {
		t.Fatalf("metadata block should be removed")
	}
}
