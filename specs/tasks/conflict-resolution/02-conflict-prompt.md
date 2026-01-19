---
task: 2
status: complete
backpressure: go test ./internal/worker/... -run ConflictPrompt
depends_on: []
---

# Conflict Resolution Prompt Builder

**Parent spec**: `/specs/CONFLICT-RESOLUTION.md`
**Task**: #2 of 4 in implementation plan

## Objective

Implement the `BuildConflictPrompt` function that creates prompts for Claude to resolve merge conflicts.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (can be implemented in parallel with Task 1)

## Deliverables

### Files to Create
```
internal/worker/
└── prompt_git.go       # NEW: Conflict resolution prompt builder
└── prompt_git_test.go  # NEW: Tests for prompt builder
```

### Functions to Implement

```go
// internal/worker/prompt_git.go
package worker

import (
	"fmt"
	"strings"
)

// BuildConflictPrompt creates the prompt for Claude to resolve merge conflicts
func BuildConflictPrompt(targetBranch string, conflictedFiles []string) string {
	return fmt.Sprintf(`The rebase onto %s resulted in merge conflicts.

Conflicted files:
%s

Please resolve all conflicts:
1. Open each conflicted file
2. Find the conflict markers (<<<<<<, =======, >>>>>>>)
3. Edit to resolve - keep the correct code, remove markers
4. Stage resolved files: git add <file>
5. Continue the rebase: git rebase --continue

If the rebase continues successfully, do NOT push - the orchestrator will handle that.

If you cannot resolve a conflict, explain why in your response.`, targetBranch, formatFileList(conflictedFiles))
}

// formatFileList formats a slice of file paths for display in prompts
func formatFileList(files []string) string {
	var sb strings.Builder
	for _, f := range files {
		fmt.Fprintf(&sb, "- %s\n", f)
	}
	return sb.String()
}
```

### Tests to Add

```go
// internal/worker/prompt_git_test.go
package worker

import (
	"strings"
	"testing"
)

func TestBuildConflictPrompt_SingleFile(t *testing.T) {
	prompt := BuildConflictPrompt("main", []string{"src/config.go"})

	if !strings.Contains(prompt, "main") {
		t.Error("prompt should contain target branch")
	}
	if !strings.Contains(prompt, "src/config.go") {
		t.Error("prompt should contain conflicted file")
	}
	if !strings.Contains(prompt, "git rebase --continue") {
		t.Error("prompt should instruct to continue rebase")
	}
}

func TestBuildConflictPrompt_MultipleFiles(t *testing.T) {
	files := []string{
		"src/config.go",
		"src/worker/merge.go",
		"internal/git/rebase.go",
	}

	prompt := BuildConflictPrompt("main", files)

	for _, f := range files {
		if !strings.Contains(prompt, f) {
			t.Errorf("prompt should contain file %s", f)
		}
	}
}

func TestBuildConflictPrompt_ContainsInstructions(t *testing.T) {
	prompt := BuildConflictPrompt("develop", []string{"file.go"})

	expectedPhrases := []string{
		"conflict markers",
		"<<<<<<",
		"=======",
		">>>>>>>",
		"git add",
		"git rebase --continue",
		"do NOT push",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("prompt should contain %q", phrase)
		}
	}
}

func TestFormatFileList(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go"}
	result := formatFileList(files)

	expected := "- a.go\n- b.go\n- c.go\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatFileList_Empty(t *testing.T) {
	result := formatFileList([]string{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatFileList_SingleFile(t *testing.T) {
	result := formatFileList([]string{"only.go"})
	expected := "- only.go\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
```

## Backpressure

### Validation Command
```bash
go test ./internal/worker/... -run ConflictPrompt
```

## NOT In Scope
- Git operations (Task 1)
- Merge flow orchestration (Task 3)
- Force push and PR merge (Task 4)
- Retry logic (Task 3)
