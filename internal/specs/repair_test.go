package specs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockRepairInvoker struct {
	output string
	err    error
}

func (m *mockRepairInvoker) Invoke(ctx context.Context, prompt string, workdir string) (string, error) {
	return m.output, m.err
}

func TestRepairFile_TaskSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	taskPath := filepath.Join(tmpDir, "01-test.md")
	content := "# Task Body\n\nDetails."
	if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	invoker := &mockRepairInvoker{
		output: `{"task":1,"backpressure":"go test ./...","status":"pending"}`,
	}
	repairer := &Repairer{Invoker: invoker}

	if _, err := repairer.RepairFile(context.Background(), taskPath, FileKindTask); err != nil {
		t.Fatalf("RepairFile failed: %v", err)
	}

	updated, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read updated task: %v", err)
	}
	if !strings.HasPrefix(string(updated), "---\n") {
		t.Fatalf("expected frontmatter at file start")
	}
	if !strings.Contains(string(updated), "task: 1") {
		t.Fatalf("expected task field in frontmatter")
	}
	if !strings.Contains(string(updated), "# Task Body") {
		t.Fatalf("expected body preserved")
	}
}

func TestRepairFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	taskPath := filepath.Join(tmpDir, "01-test.md")
	if err := os.WriteFile(taskPath, []byte("body"), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	invoker := &mockRepairInvoker{
		output: "not-json",
	}
	repairer := &Repairer{Invoker: invoker}

	if _, err := repairer.RepairFile(context.Background(), taskPath, FileKindTask); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestRepairFile_FencedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	taskPath := filepath.Join(tmpDir, "01-test.md")
	if err := os.WriteFile(taskPath, []byte("body"), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	invoker := &mockRepairInvoker{
		output: "```json\n{\"task\":1,\"backpressure\":\"go test ./...\"}\n```",
	}
	repairer := &Repairer{Invoker: invoker}

	if _, err := repairer.RepairFile(context.Background(), taskPath, FileKindTask); err != nil {
		t.Fatalf("expected fenced JSON to parse, got: %v", err)
	}
}

func TestRepairFile_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	taskPath := filepath.Join(tmpDir, "01-test.md")
	if err := os.WriteFile(taskPath, []byte("body"), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	invoker := &mockRepairInvoker{
		output: `{"task":0}`,
	}
	repairer := &Repairer{Invoker: invoker}

	if _, err := repairer.RepairFile(context.Background(), taskPath, FileKindTask); err == nil {
		t.Fatalf("expected error for missing required fields")
	}
}
