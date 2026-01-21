package provider

import "testing"

func TestReviewResult_HasErrors_NoIssues(t *testing.T) {
	result := &ReviewResult{
		Issues: nil,
	}
	if result.HasErrors() {
		t.Error("Expected HasErrors() to return false when Issues is nil")
	}

	result = &ReviewResult{
		Issues: []ReviewIssue{},
	}
	if result.HasErrors() {
		t.Error("Expected HasErrors() to return false when Issues is empty")
	}
}

func TestReviewResult_HasErrors_OnlyWarnings(t *testing.T) {
	result := &ReviewResult{
		Issues: []ReviewIssue{
			{Severity: SeverityWarning, Message: "unused variable"},
			{Severity: SeveritySuggestion, Message: "could be simplified"},
			{Severity: SeverityInfo, Message: "note about code"},
		},
	}
	if result.HasErrors() {
		t.Error("Expected HasErrors() to return false when only warnings present")
	}
}

func TestReviewResult_HasErrors_WithError(t *testing.T) {
	result := &ReviewResult{
		Issues: []ReviewIssue{
			{Severity: SeverityWarning, Message: "unused variable"},
			{Severity: SeverityError, Message: "nil pointer dereference"},
		},
	}
	if !result.HasErrors() {
		t.Error("Expected HasErrors() to return true when error issue exists")
	}
}

func TestReviewResult_ErrorCount(t *testing.T) {
	tests := []struct {
		name     string
		issues   []ReviewIssue
		expected int
	}{
		{
			name:     "no issues",
			issues:   nil,
			expected: 0,
		},
		{
			name: "no errors",
			issues: []ReviewIssue{
				{Severity: SeverityWarning, Message: "warning 1"},
				{Severity: SeverityInfo, Message: "info 1"},
			},
			expected: 0,
		},
		{
			name: "one error",
			issues: []ReviewIssue{
				{Severity: SeverityError, Message: "error 1"},
				{Severity: SeverityWarning, Message: "warning 1"},
			},
			expected: 1,
		},
		{
			name: "multiple errors",
			issues: []ReviewIssue{
				{Severity: SeverityError, Message: "error 1"},
				{Severity: SeverityWarning, Message: "warning 1"},
				{Severity: SeverityError, Message: "error 2"},
				{Severity: SeverityError, Message: "error 3"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ReviewResult{Issues: tt.issues}
			got := result.ErrorCount()
			if got != tt.expected {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestReviewResult_WarningCount(t *testing.T) {
	tests := []struct {
		name     string
		issues   []ReviewIssue
		expected int
	}{
		{
			name:     "no issues",
			issues:   nil,
			expected: 0,
		},
		{
			name: "no warnings",
			issues: []ReviewIssue{
				{Severity: SeverityError, Message: "error 1"},
				{Severity: SeverityInfo, Message: "info 1"},
			},
			expected: 0,
		},
		{
			name: "one warning",
			issues: []ReviewIssue{
				{Severity: SeverityWarning, Message: "warning 1"},
				{Severity: SeverityError, Message: "error 1"},
			},
			expected: 1,
		},
		{
			name: "multiple warnings",
			issues: []ReviewIssue{
				{Severity: SeverityWarning, Message: "warning 1"},
				{Severity: SeverityError, Message: "error 1"},
				{Severity: SeverityWarning, Message: "warning 2"},
				{Severity: SeverityWarning, Message: "warning 3"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ReviewResult{Issues: tt.issues}
			got := result.WarningCount()
			if got != tt.expected {
				t.Errorf("WarningCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestReviewResult_IssuesByFile(t *testing.T) {
	result := &ReviewResult{
		Issues: []ReviewIssue{
			{File: "main.go", Line: 10, Message: "issue 1"},
			{File: "main.go", Line: 20, Message: "issue 2"},
			{File: "util.go", Line: 5, Message: "issue 3"},
		},
	}
	grouped := result.IssuesByFile()

	// Verify main.go has 2 issues
	mainIssues, ok := grouped["main.go"]
	if !ok {
		t.Fatal("Expected main.go in grouped results")
	}
	if len(mainIssues) != 2 {
		t.Errorf("Expected 2 issues for main.go, got %d", len(mainIssues))
	}

	// Verify util.go has 1 issue
	utilIssues, ok := grouped["util.go"]
	if !ok {
		t.Fatal("Expected util.go in grouped results")
	}
	if len(utilIssues) != 1 {
		t.Errorf("Expected 1 issue for util.go, got %d", len(utilIssues))
	}

	// Verify correct grouping
	if mainIssues[0].Message != "issue 1" || mainIssues[1].Message != "issue 2" {
		t.Error("main.go issues not grouped correctly")
	}
	if utilIssues[0].Message != "issue 3" {
		t.Error("util.go issue not grouped correctly")
	}
}

func TestReviewResult_IssuesByFile_Empty(t *testing.T) {
	result := &ReviewResult{
		Issues: nil,
	}
	grouped := result.IssuesByFile()

	if grouped == nil {
		t.Error("Expected non-nil map for empty issues")
	}
	if len(grouped) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(grouped))
	}
}

func TestIsValidSeverity_Valid(t *testing.T) {
	validSeverities := []string{"error", "warning", "suggestion", "info"}
	for _, severity := range validSeverities {
		t.Run(severity, func(t *testing.T) {
			if !IsValidSeverity(severity) {
				t.Errorf("Expected IsValidSeverity(%q) to return true", severity)
			}
		})
	}
}

func TestIsValidSeverity_Invalid(t *testing.T) {
	invalidSeverities := []string{"critical", "", "ERROR", "Warning", "SUGGESTION"}
	for _, severity := range invalidSeverities {
		t.Run(severity, func(t *testing.T) {
			if IsValidSeverity(severity) {
				t.Errorf("Expected IsValidSeverity(%q) to return false", severity)
			}
		})
	}
}
