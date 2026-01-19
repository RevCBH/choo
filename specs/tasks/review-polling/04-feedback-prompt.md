---
task: 4
status: pending
backpressure: "go test ./internal/worker/... -run TestBuildFeedbackPrompt"
depends_on: []
---

# BuildFeedbackPrompt Function

**Parent spec**: `/specs/REVIEW-POLLING.md`
**Task**: #4 of 5 in implementation plan

## Objective

Create the BuildFeedbackPrompt function that constructs Claude prompts for addressing PR feedback.

## Dependencies

### External Specs (must be implemented)
- github - PRComment type must exist

### Task Dependencies (within this unit)
- None (can be implemented in parallel with tasks 1-3)

## Deliverables

### Files to Create
```
internal/worker/
├── prompt_git.go      # NEW: Feedback prompt builder
└── prompt_git_test.go # NEW: Tests for feedback prompts
```

### Types and Functions to Implement

Create `internal/worker/prompt_git.go`:

```go
package worker

import (
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/github"
)

// BuildFeedbackPrompt constructs the Claude prompt for addressing PR feedback
func BuildFeedbackPrompt(prURL string, comments []github.PRComment) string {
	var commentText strings.Builder
	for _, c := range comments {
		fmt.Fprintf(&commentText, "- @%s: %s\n", c.Author, c.Body)
		if c.Path != "" {
			fmt.Fprintf(&commentText, "  (on %s:%d)\n", c.Path, c.Line)
		}
	}

	return fmt.Sprintf(`PR %s has received feedback. Please address the following comments:

%s
After making changes:
1. Stage and commit with a message like "address review feedback"
2. Push the changes

The orchestrator will continue polling for approval.`, prURL, commentText.String())
}
```

### Tests to Create

Create `internal/worker/prompt_git_test.go`:

```go
package worker

import (
	"testing"

	"github.com/RevCBH/choo/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestBuildFeedbackPrompt_IncludesAllComments(t *testing.T) {
	comments := []github.PRComment{
		{Author: "alice", Body: "Fix the null check", Path: "main.go", Line: 42},
		{Author: "bob", Body: "Add tests"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/123", comments)

	assert.Contains(t, prompt, "@alice: Fix the null check")
	assert.Contains(t, prompt, "(on main.go:42)")
	assert.Contains(t, prompt, "@bob: Add tests")
	assert.Contains(t, prompt, "pull/123")
}

func TestBuildFeedbackPrompt_HandlesNoPath(t *testing.T) {
	comments := []github.PRComment{
		{Author: "reviewer", Body: "General comment about the PR"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/456", comments)

	assert.Contains(t, prompt, "@reviewer: General comment about the PR")
	assert.NotContains(t, prompt, "(on ")
}

func TestBuildFeedbackPrompt_EmptyComments(t *testing.T) {
	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/789", []github.PRComment{})

	assert.Contains(t, prompt, "pull/789")
	assert.Contains(t, prompt, "address review feedback")
}

func TestBuildFeedbackPrompt_MultipleCommentsOnSameFile(t *testing.T) {
	comments := []github.PRComment{
		{Author: "alice", Body: "Fix line 10", Path: "main.go", Line: 10},
		{Author: "alice", Body: "Fix line 20", Path: "main.go", Line: 20},
		{Author: "bob", Body: "Fix util.go", Path: "util.go", Line: 5},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/100", comments)

	assert.Contains(t, prompt, "(on main.go:10)")
	assert.Contains(t, prompt, "(on main.go:20)")
	assert.Contains(t, prompt, "(on util.go:5)")
}

func TestBuildFeedbackPrompt_ContainsInstructions(t *testing.T) {
	comments := []github.PRComment{
		{Author: "reviewer", Body: "Please fix"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/1", comments)

	assert.Contains(t, prompt, "Stage and commit")
	assert.Contains(t, prompt, "Push the changes")
	assert.Contains(t, prompt, "orchestrator will continue polling")
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/worker/... -run TestBuildFeedbackPrompt
```

## NOT In Scope
- FeedbackHandler struct (task #5)
- Claude invocation
- Git operations
- Event emission
