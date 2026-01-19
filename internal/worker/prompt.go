package worker

import (
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/discovery"
)

// TaskPrompt contains the constructed prompt for Claude
type TaskPrompt struct {
	Content    string
	ReadyTasks []*discovery.Task
}

// BuildTaskPrompt constructs the Claude prompt for ready tasks
// The prompt presents all ready tasks and instructs Claude to choose one
func BuildTaskPrompt(readyTasks []*discovery.Task) TaskPrompt {
	var taskList strings.Builder
	for _, t := range readyTasks {
		fmt.Fprintf(&taskList, "### Task #%d: %s\n", t.Number, t.Title)
		fmt.Fprintf(&taskList, "- File: %s\n", t.FilePath)
		fmt.Fprintf(&taskList, "- Backpressure: `%s`\n\n", t.Backpressure)
	}

	content := fmt.Sprintf(`You are executing a Ralph task. Follow these instructions exactly.

## Ready Tasks

The following tasks have all dependencies satisfied. Choose ONE to implement:

%s

## Instructions
1. Choose one task from the ready list above
2. Read that task's spec file completely
3. Implement ONLY what is specified - nothing more, nothing less
4. Run the backpressure validation command from the task's frontmatter
5. If validation fails, fix the issues and re-run until it passes
6. When the backpressure check passes, UPDATE THE FRONTMATTER STATUS to complete:
   - Edit the task spec file
   - Change `+"`status: in_progress`"+` to `+"`status: complete`"+`
7. Do NOT move on to other tasks - stop after completing one

## Critical
- Choose ONE task and complete it fully
- You MUST update the spec file's frontmatter status to `+"`complete`"+` when done
- The backpressure command MUST pass before marking complete
- Do not refactor unrelated code
- Do not add features not in the spec
- NEVER run tests in watch mode. Always use flags to run tests once and exit.
`,
		taskList.String(),
	)

	return TaskPrompt{
		Content:    content,
		ReadyTasks: readyTasks,
	}
}

// BuildBaselineFixPrompt constructs the prompt for fixing baseline failures
func BuildBaselineFixPrompt(checkOutput string, baselineCommands string) string {
	return fmt.Sprintf(`You are fixing baseline check failures. Follow these instructions exactly.

## Baseline Check Failures

The following baseline checks failed after completing all tasks:

%s

## Baseline Commands
%s

## Instructions

1. Review the error output above
2. Fix the issues (formatting, linting, type errors)
3. Re-run the baseline checks to verify fixes
4. Do NOT commit - the orchestrator will commit for you

## Critical
- Only fix issues reported by baseline checks
- Do not refactor or change logic
- Do not modify test assertions
- These are lint/format fixes only
`,
		checkOutput,
		baselineCommands,
	)
}
