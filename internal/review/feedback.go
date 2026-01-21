package review

import (
	"context"
	"encoding/json"
	"fmt"
)

// TaskInvoker abstracts Task tool invocation for testing
type TaskInvoker interface {
	InvokeTask(ctx context.Context, prompt string, subagentType string) (string, error)
}

// FeedbackApplier applies review feedback to specs
type FeedbackApplier struct {
	taskTool TaskInvoker
}

// NewFeedbackApplier creates a new FeedbackApplier
func NewFeedbackApplier(taskTool TaskInvoker) *FeedbackApplier {
	return &FeedbackApplier{
		taskTool: taskTool,
	}
}

// ApplyFeedback applies the given feedback to specs at the path
func (f *FeedbackApplier) ApplyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error {
	// Empty feedback is a no-op
	if len(feedback) == 0 {
		return nil
	}

	// Serialize feedback to JSON
	feedbackJSON, err := json.MarshalIndent(feedback, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize feedback: %w", err)
	}

	// Construct prompt for Task tool
	prompt := fmt.Sprintf(`Apply the following review feedback to the specs.

Specs directory: %s

Feedback to apply:
%s

For each feedback item:
1. Locate the specified section in the specs
2. Address the issue according to the suggestion
3. Maintain consistency with the rest of the spec

Make the minimal changes necessary to address each issue.`, specsPath, string(feedbackJSON))

	// Invoke task tool with general-purpose subagent
	_, err = f.taskTool.InvokeTask(ctx, prompt, "general-purpose")
	if err != nil {
		return fmt.Errorf("failed to apply feedback via task tool: %w", err)
	}

	return nil
}
