package provider

import (
	"strings"
	"testing"
)

func TestBuildClaudeReviewPrompt_ContainsReviewInstruction(t *testing.T) {
	diff := "sample diff content"
	prompt := BuildClaudeReviewPrompt(diff)

	if !strings.Contains(prompt, "Review the following code changes") {
		t.Error("Expected prompt to contain 'Review the following code changes'")
	}
}

func TestBuildClaudeReviewPrompt_ContainsFocusAreas(t *testing.T) {
	diff := "sample diff content"
	prompt := BuildClaudeReviewPrompt(diff)

	focusAreas := []string{
		"Bugs or logical errors",
		"Security vulnerabilities",
		"Performance problems",
		"Code style and best practices",
	}

	for _, area := range focusAreas {
		if !strings.Contains(prompt, area) {
			t.Errorf("Expected prompt to contain focus area: %s", area)
		}
	}
}

func TestBuildClaudeReviewPrompt_ContainsJSONSchema(t *testing.T) {
	diff := "sample diff content"
	prompt := BuildClaudeReviewPrompt(diff)

	if !strings.Contains(prompt, `"passed": true/false`) {
		t.Error("Expected prompt to contain JSON schema with '\"passed\": true/false'")
	}
}

func TestBuildClaudeReviewPrompt_ContainsDiff(t *testing.T) {
	diff := "this is my unique diff content with special markers"
	prompt := BuildClaudeReviewPrompt(diff)

	if !strings.Contains(prompt, "DIFF:") {
		t.Error("Expected prompt to contain 'DIFF:' marker")
	}

	if !strings.Contains(prompt, diff) {
		t.Error("Expected prompt to contain the provided diff content")
	}
}

func TestBuildClaudeReviewPrompt_ContainsSeverityOptions(t *testing.T) {
	diff := "sample diff content"
	prompt := BuildClaudeReviewPrompt(diff)

	severityOptions := []string{"error", "warning", "suggestion"}

	for _, severity := range severityOptions {
		if !strings.Contains(prompt, severity) {
			t.Errorf("Expected prompt to contain severity option: %s", severity)
		}
	}
}
