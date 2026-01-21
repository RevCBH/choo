package provider

import (
	"testing"
)

func TestCodexReviewer_Name(t *testing.T) {
	reviewer := NewCodexReviewer("")
	if got := reviewer.Name(); got != ProviderCodex {
		t.Errorf("Name() = %v, want %v", got, ProviderCodex)
	}
}

func TestCodexReviewer_ParseOutput_NoIssues(t *testing.T) {
	reviewer := NewCodexReviewer("")
	output := "Review complete.\nNo issues found."

	result, err := reviewer.parseOutput(output, 0)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if !result.Passed {
		t.Error("Passed = false, want true")
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues count = %d, want 0", len(result.Issues))
	}
	if result.Summary != "No issues found" {
		t.Errorf("Summary = %q, want %q", result.Summary, "No issues found")
	}
}

func TestCodexReviewer_ParseOutput_WithIssues(t *testing.T) {
	reviewer := NewCodexReviewer("")
	output := `main.go:10: error: undefined variable x
main.go:15: warning: unused import "fmt"
Review complete.`

	result, err := reviewer.parseOutput(output, 1)
	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if result.Passed {
		t.Error("Passed = true, want false")
	}
	if len(result.Issues) != 2 {
		t.Fatalf("Issues count = %d, want 2", len(result.Issues))
	}

	// Verify first issue
	issue := result.Issues[0]
	if issue.File != "main.go" {
		t.Errorf("Issue[0].File = %q, want %q", issue.File, "main.go")
	}
	if issue.Line != 10 {
		t.Errorf("Issue[0].Line = %d, want %d", issue.Line, 10)
	}
	if issue.Severity != "error" {
		t.Errorf("Issue[0].Severity = %q, want %q", issue.Severity, "error")
	}
}

func TestCodexReviewer_ParseLine_AllSeverities(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantFile string
		wantLine int
		wantSev  string
		wantMsg  string
	}{
		{
			name:     "error severity",
			line:     "file.go:42: error: message here",
			wantFile: "file.go",
			wantLine: 42,
			wantSev:  "error",
			wantMsg:  "message here",
		},
		{
			name:     "warning with column",
			line:     "path/to/file.go:10:5: warning: unused var",
			wantFile: "path/to/file.go",
			wantLine: 10,
			wantSev:  "warning",
			wantMsg:  "unused var",
		},
		{
			name:     "suggestion severity",
			line:     "util.go:25: suggestion: use constant for magic number",
			wantFile: "util.go",
			wantLine: 25,
			wantSev:  "suggestion",
			wantMsg:  "use constant for magic number",
		},
		{
			name:     "info severity",
			line:     "test.go:1: info: consider renaming",
			wantFile: "test.go",
			wantLine: 1,
			wantSev:  "info",
			wantMsg:  "consider renaming",
		},
	}

	reviewer := NewCodexReviewer("")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := reviewer.parseLine(tt.line)
			if issue == nil {
				t.Fatal("parseLine() returned nil, want issue")
			}
			if issue.File != tt.wantFile {
				t.Errorf("File = %q, want %q", issue.File, tt.wantFile)
			}
			if issue.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", issue.Line, tt.wantLine)
			}
			if issue.Severity != tt.wantSev {
				t.Errorf("Severity = %q, want %q", issue.Severity, tt.wantSev)
			}
			if issue.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", issue.Message, tt.wantMsg)
			}
		})
	}
}

func TestCodexReviewer_ParseLine_Invalid(t *testing.T) {
	lines := []string{
		"",
		"   ",
		"not an issue line",
		"file.go: missing line number",
		"Review complete.",
	}

	reviewer := NewCodexReviewer("")
	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			issue := reviewer.parseLine(line)
			if issue != nil {
				t.Errorf("parseLine(%q) = %+v, want nil", line, issue)
			}
		})
	}
}

func TestCodexReviewer_CommandConfig(t *testing.T) {
	t.Run("default command", func(t *testing.T) {
		reviewer := NewCodexReviewer("")
		if reviewer.command != "" {
			t.Errorf("command = %q, want empty", reviewer.command)
		}
	})

	t.Run("custom command", func(t *testing.T) {
		customPath := "/opt/codex/bin/codex"
		reviewer := NewCodexReviewer(customPath)
		if reviewer.command != customPath {
			t.Errorf("command = %q, want %q", reviewer.command, customPath)
		}
	})
}
