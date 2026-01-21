package discovery

import (
	"testing"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	content := []byte(`---
task: 1
status: in_progress
backpressure: "go test ./..."
depends_on: []
---

# Title`)

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedFM := `task: 1
status: in_progress
backpressure: "go test ./..."
depends_on: []`
	if string(fm) != expectedFM {
		t.Errorf("frontmatter mismatch\nexpected: %q\ngot: %q", expectedFM, string(fm))
	}

	expectedBody := "\n# Title"
	if string(body) != expectedBody {
		t.Errorf("body mismatch\nexpected: %q\ngot: %q", expectedBody, string(body))
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte("# Just a title")

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got: %q", string(fm))
	}

	if string(body) != string(content) {
		t.Errorf("expected full body, got: %q", string(body))
	}
}

func TestParseFrontmatter_Unclosed(t *testing.T) {
	content := []byte(`---
task: 1
status: in_progress
# Title`)

	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter, got nil")
	}

	expectedErr := "unclosed frontmatter: missing closing '---'"
	if err.Error() != expectedErr {
		t.Errorf("error mismatch\nexpected: %q\ngot: %q", expectedErr, err.Error())
	}
}

func TestParseUnitFrontmatter_Complete(t *testing.T) {
	data := []byte(`unit: app-shell
depends_on: [core, utils]
orch_status: in_progress
orch_branch: unit/app-shell
orch_worktree: /path/to/worktree
orch_pr_number: 42
orch_started_at: "2024-01-15T10:30:00Z"
orch_completed_at: "2024-01-15T12:00:00Z"`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "app-shell" {
		t.Errorf("Unit: expected 'app-shell', got %q", uf.Unit)
	}

	if len(uf.DependsOn) != 2 || uf.DependsOn[0] != "core" || uf.DependsOn[1] != "utils" {
		t.Errorf("DependsOn: expected [core, utils], got %v", uf.DependsOn)
	}

	if uf.OrchStatus != "in_progress" {
		t.Errorf("OrchStatus: expected 'in_progress', got %q", uf.OrchStatus)
	}

	if uf.OrchBranch != "unit/app-shell" {
		t.Errorf("OrchBranch: expected 'unit/app-shell', got %q", uf.OrchBranch)
	}

	if uf.OrchWorktree != "/path/to/worktree" {
		t.Errorf("OrchWorktree: expected '/path/to/worktree', got %q", uf.OrchWorktree)
	}

	if uf.OrchPRNumber != 42 {
		t.Errorf("OrchPRNumber: expected 42, got %d", uf.OrchPRNumber)
	}

	if uf.OrchStartedAt != "2024-01-15T10:30:00Z" {
		t.Errorf("OrchStartedAt: expected '2024-01-15T10:30:00Z', got %q", uf.OrchStartedAt)
	}

	if uf.OrchCompletedAt != "2024-01-15T12:00:00Z" {
		t.Errorf("OrchCompletedAt: expected '2024-01-15T12:00:00Z', got %q", uf.OrchCompletedAt)
	}
}

func TestParseUnitFrontmatter_Minimal(t *testing.T) {
	data := []byte(`unit: minimal-unit`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "minimal-unit" {
		t.Errorf("Unit: expected 'minimal-unit', got %q", uf.Unit)
	}

	if len(uf.DependsOn) != 0 {
		t.Errorf("DependsOn: expected empty slice, got %v", uf.DependsOn)
	}

	if uf.OrchStatus != "" {
		t.Errorf("OrchStatus: expected empty string, got %q", uf.OrchStatus)
	}
}

func TestParseUnitFrontmatter_WithProvider(t *testing.T) {
	data := []byte(`unit: my-feature
provider: codex
depends_on:
  - base-types`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "my-feature" {
		t.Errorf("Unit: expected 'my-feature', got %q", uf.Unit)
	}

	if uf.Provider != "codex" {
		t.Errorf("Provider: expected 'codex', got %q", uf.Provider)
	}

	if len(uf.DependsOn) != 1 || uf.DependsOn[0] != "base-types" {
		t.Errorf("DependsOn: expected [base-types], got %v", uf.DependsOn)
	}
}

func TestParseUnitFrontmatter_WithoutProvider(t *testing.T) {
	data := []byte(`unit: my-feature
depends_on:
  - base-types`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "my-feature" {
		t.Errorf("Unit: expected 'my-feature', got %q", uf.Unit)
	}

	if uf.Provider != "" {
		t.Errorf("Provider: expected empty string, got %q", uf.Provider)
	}

	if len(uf.DependsOn) != 1 || uf.DependsOn[0] != "base-types" {
		t.Errorf("DependsOn: expected [base-types], got %v", uf.DependsOn)
	}
}

func TestParseUnitFrontmatter_ProviderClaude(t *testing.T) {
	data := []byte(`unit: claude-optimized
provider: claude
depends_on: []`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "claude-optimized" {
		t.Errorf("Unit: expected 'claude-optimized', got %q", uf.Unit)
	}

	if uf.Provider != "claude" {
		t.Errorf("Provider: expected 'claude', got %q", uf.Provider)
	}
}

func TestParseUnitFrontmatter_ProviderWithOrchFields(t *testing.T) {
	data := []byte(`unit: my-feature
provider: codex
depends_on: []
orch_status: in_progress
orch_branch: feature/my-feature
orch_pr_number: 42`)

	uf, err := ParseUnitFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if uf.Unit != "my-feature" {
		t.Errorf("Unit: expected 'my-feature', got %q", uf.Unit)
	}

	if uf.Provider != "codex" {
		t.Errorf("Provider: expected 'codex', got %q", uf.Provider)
	}

	if uf.OrchStatus != "in_progress" {
		t.Errorf("OrchStatus: expected 'in_progress', got %q", uf.OrchStatus)
	}

	if uf.OrchBranch != "feature/my-feature" {
		t.Errorf("OrchBranch: expected 'feature/my-feature', got %q", uf.OrchBranch)
	}

	if uf.OrchPRNumber != 42 {
		t.Errorf("OrchPRNumber: expected 42, got %d", uf.OrchPRNumber)
	}
}

func TestParseTaskFrontmatter_Complete(t *testing.T) {
	data := []byte(`task: 5
status: complete
backpressure: "go test ./internal/..."`)

	tf, err := ParseTaskFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if tf.Task != 5 {
		t.Errorf("Task: expected 5, got %d", tf.Task)
	}

	if tf.Status != "complete" {
		t.Errorf("Status: expected 'complete', got %q", tf.Status)
	}

	if tf.Backpressure != "go test ./internal/..." {
		t.Errorf("Backpressure: expected 'go test ./internal/...', got %q", tf.Backpressure)
	}

	if len(tf.DependsOn) != 0 {
		t.Errorf("DependsOn: expected empty slice, got %v", tf.DependsOn)
	}
}

func TestParseTaskFrontmatter_WithDeps(t *testing.T) {
	data := []byte(`task: 3
status: pending
backpressure: "make test"
depends_on: [1, 2]`)

	tf, err := ParseTaskFrontmatter(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if tf.Task != 3 {
		t.Errorf("Task: expected 3, got %d", tf.Task)
	}

	if len(tf.DependsOn) != 2 || tf.DependsOn[0] != 1 || tf.DependsOn[1] != 2 {
		t.Errorf("DependsOn: expected [1, 2], got %v", tf.DependsOn)
	}
}

func TestExtractTitle_Found(t *testing.T) {
	body := []byte("\n# Title\n\nSome content")

	title := extractTitle(body)
	if title != "Title" {
		t.Errorf("expected 'Title', got %q", title)
	}
}

func TestExtractTitle_NotFound(t *testing.T) {
	body := []byte("No heading here\n## H2 heading\nMore content")

	title := extractTitle(body)
	if title != "" {
		t.Errorf("expected empty string, got %q", title)
	}
}
