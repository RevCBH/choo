package review

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// mockTaskInvoker is a test double for TaskInvoker
type mockTaskInvoker struct {
	invokeFunc func(ctx context.Context, prompt string, subagentType string) (string, error)
	lastPrompt string
	lastType   string
}

func (m *mockTaskInvoker) InvokeTask(ctx context.Context, prompt string, subagentType string) (string, error) {
	m.lastPrompt = prompt
	m.lastType = subagentType
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, prompt, subagentType)
	}
	return "success", nil
}

func TestNewFeedbackApplier(t *testing.T) {
	mock := &mockTaskInvoker{}
	applier := NewFeedbackApplier(mock)

	if applier == nil {
		t.Fatal("NewFeedbackApplier returned nil")
	}

	if applier.taskTool != mock {
		t.Error("FeedbackApplier.taskTool not set correctly")
	}
}

func TestApplyFeedback_EmptyFeedback(t *testing.T) {
	mock := &mockTaskInvoker{}
	applier := NewFeedbackApplier(mock)

	err := applier.ApplyFeedback(context.Background(), "/path/to/specs", []ReviewFeedback{})

	if err != nil {
		t.Errorf("ApplyFeedback with empty feedback returned error: %v", err)
	}

	if mock.lastPrompt != "" {
		t.Error("ApplyFeedback should not invoke task tool with empty feedback")
	}
}

func TestApplyFeedback_CallsTaskTool(t *testing.T) {
	mock := &mockTaskInvoker{}
	applier := NewFeedbackApplier(mock)

	feedback := []ReviewFeedback{
		{
			Section:    "## Objective",
			Issue:      "Missing clarity",
			Suggestion: "Add more detail",
		},
	}

	err := applier.ApplyFeedback(context.Background(), "/path/to/specs", feedback)

	if err != nil {
		t.Fatalf("ApplyFeedback returned error: %v", err)
	}

	if mock.lastPrompt == "" {
		t.Fatal("ApplyFeedback did not invoke task tool")
	}

	if mock.lastType != "general-purpose" {
		t.Errorf("Expected subagentType 'general-purpose', got '%s'", mock.lastType)
	}
}

func TestApplyFeedback_PromptContainsPath(t *testing.T) {
	mock := &mockTaskInvoker{}
	applier := NewFeedbackApplier(mock)

	specsPath := "/custom/specs/path"
	feedback := []ReviewFeedback{
		{
			Section:    "## Test",
			Issue:      "Issue",
			Suggestion: "Fix",
		},
	}

	err := applier.ApplyFeedback(context.Background(), specsPath, feedback)

	if err != nil {
		t.Fatalf("ApplyFeedback returned error: %v", err)
	}

	if !strings.Contains(mock.lastPrompt, specsPath) {
		t.Errorf("Prompt does not contain specs path '%s'\nPrompt: %s", specsPath, mock.lastPrompt)
	}
}

func TestApplyFeedback_PromptContainsFeedback(t *testing.T) {
	mock := &mockTaskInvoker{}
	applier := NewFeedbackApplier(mock)

	feedback := []ReviewFeedback{
		{
			Section:    "## Implementation",
			Issue:      "Missing error handling",
			Suggestion: "Add error checks",
		},
		{
			Section:    "## Testing",
			Issue:      "No test coverage",
			Suggestion: "Add unit tests",
		},
	}

	err := applier.ApplyFeedback(context.Background(), "/path/to/specs", feedback)

	if err != nil {
		t.Fatalf("ApplyFeedback returned error: %v", err)
	}

	// Verify that the prompt contains JSON-serialized feedback
	// Check that all feedback fields are present
	for _, fb := range feedback {
		if !strings.Contains(mock.lastPrompt, fb.Section) {
			t.Errorf("Prompt does not contain feedback section '%s'", fb.Section)
		}
		if !strings.Contains(mock.lastPrompt, fb.Issue) {
			t.Errorf("Prompt does not contain feedback issue '%s'", fb.Issue)
		}
		if !strings.Contains(mock.lastPrompt, fb.Suggestion) {
			t.Errorf("Prompt does not contain feedback suggestion '%s'", fb.Suggestion)
		}
	}

	// Verify it's valid JSON by attempting to unmarshal
	// Extract JSON from the prompt (it's formatted with indentation)
	var foundJSON bool
	lines := strings.Split(mock.lastPrompt, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "[" {
			// Try to find the closing bracket
			for j := i; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "]" {
					jsonStr := strings.Join(lines[i:j+1], "\n")
					var parsedFeedback []ReviewFeedback
					if err := json.Unmarshal([]byte(jsonStr), &parsedFeedback); err != nil {
						t.Errorf("Feedback in prompt is not valid JSON: %v", err)
					} else {
						foundJSON = true
					}
					break
				}
			}
			break
		}
	}

	if !foundJSON {
		t.Error("Could not find valid JSON feedback in prompt")
	}
}

func TestApplyFeedback_TaskToolError(t *testing.T) {
	expectedErr := errors.New("task tool failed")
	mock := &mockTaskInvoker{
		invokeFunc: func(ctx context.Context, prompt string, subagentType string) (string, error) {
			return "", expectedErr
		},
	}
	applier := NewFeedbackApplier(mock)

	feedback := []ReviewFeedback{
		{
			Section:    "## Test",
			Issue:      "Issue",
			Suggestion: "Fix",
		},
	}

	err := applier.ApplyFeedback(context.Background(), "/path/to/specs", feedback)

	if err == nil {
		t.Fatal("ApplyFeedback should propagate task tool error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap task tool error, got: %v", err)
	}
}
