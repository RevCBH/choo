---
task: 2
status: complete
backpressure: "go test ./internal/worker/... -run Prompt"
depends_on: []
---

# Git Operation Prompt Builders

**Parent spec**: `/Users/bennett/conductor/workspaces/choo/las-vegas/specs/CLAUDE-GIT.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement prompt builder functions for commit, push, and PR creation operations.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

## Deliverables

### Files to Create/Modify

```
internal/worker/
├── prompt_git.go       # CREATE: Git operation prompt builders
└── prompt_git_test.go  # CREATE: Prompt tests
```

### Functions to Implement

```go
// internal/worker/prompt_git.go

package worker

import (
    "fmt"
    "strings"
)

// BuildCommitPrompt creates a prompt for Claude to commit changes
func BuildCommitPrompt(taskTitle string, files []string) string {
    return fmt.Sprintf(`Task "%s" is complete.

Stage and commit the changes:
1. Run: git add -A
2. Run: git commit with a conventional commit message

Guidelines for the commit message:
- Use conventional commit format (feat:, fix:, refactor:, etc.)
- First line: concise summary of what changed (50 chars or less)
- If needed, add a blank line then detailed explanation
- Explain WHY, not just WHAT

Files changed:
%s

Do NOT push yet. Just stage and commit.`, taskTitle, formatFileList(files))
}

// BuildPushPrompt creates a prompt for Claude to push the branch
func BuildPushPrompt(branch string) string {
    return fmt.Sprintf(`Push the branch to origin:

git push -u origin %s

If the push fails due to a transient error (network, etc.), that's okay -
the orchestrator will retry. Just attempt the push.`, branch)
}

// BuildPRPrompt creates a prompt for Claude to create a PR
func BuildPRPrompt(branch, targetBranch, unitTitle string) string {
    return fmt.Sprintf(`All tasks for unit "%s" are complete.

Create a pull request:
- Source branch: %s
- Target branch: %s

Use the gh CLI:
  gh pr create --base %s --head %s --title "..." --body "..."

Guidelines for the PR:
- Title: Clear, concise summary of the unit's purpose
- Body:
  - Brief overview of what was implemented
  - Key changes or decisions made
  - Any notes for reviewers

Print the PR URL when done so the orchestrator can capture it.`, unitTitle, branch, targetBranch, targetBranch, branch)
}

// formatFileList formats a list of files for inclusion in prompts
func formatFileList(files []string) string {
    if len(files) == 0 {
        return "(no files listed)"
    }
    var result strings.Builder
    for _, f := range files {
        result.WriteString("- ")
        result.WriteString(f)
        result.WriteString("\n")
    }
    return result.String()
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Prompt
```

### Must Pass

| Test | Assertion |
|------|-----------|
| TestBuildCommitPrompt_IncludesTaskTitle | prompt contains task title |
| TestBuildCommitPrompt_IncludesGitAdd | prompt contains "git add -A" |
| TestBuildCommitPrompt_IncludesConventionalCommit | prompt mentions "conventional commit" |
| TestBuildCommitPrompt_FormatsFileList | prompt contains all files |
| TestBuildCommitPrompt_EmptyFileList | prompt contains "(no files listed)" |
| TestBuildPushPrompt_IncludesBranch | prompt contains branch name |
| TestBuildPushPrompt_IncludesUpstream | prompt contains "-u origin" |
| TestBuildPRPrompt_IncludesAllDetails | prompt contains branch, target, unit title |
| TestBuildPRPrompt_IncludesGhCli | prompt contains "gh pr create" |
| TestFormatFileList_Empty | returns "(no files listed)" |
| TestFormatFileList_MultipleFiles | returns bullet list |

### Test Implementations

```go
// internal/worker/prompt_git_test.go

package worker

import (
    "strings"
    "testing"
)

func TestBuildCommitPrompt_IncludesTaskTitle(t *testing.T) {
    prompt := BuildCommitPrompt("Implement user authentication", []string{"auth.go", "auth_test.go"})

    if !strings.Contains(prompt, "Implement user authentication") {
        t.Error("prompt should contain task title")
    }
}

func TestBuildCommitPrompt_IncludesGitAdd(t *testing.T) {
    prompt := BuildCommitPrompt("Test task", []string{"file.go"})

    if !strings.Contains(prompt, "git add -A") {
        t.Error("prompt should instruct to stage changes")
    }
}

func TestBuildCommitPrompt_IncludesConventionalCommit(t *testing.T) {
    prompt := BuildCommitPrompt("Test task", []string{"file.go"})

    if !strings.Contains(prompt, "conventional commit") {
        t.Error("prompt should mention conventional commit format")
    }
}

func TestBuildCommitPrompt_FormatsFileList(t *testing.T) {
    files := []string{"src/main.go", "src/utils.go", "README.md"}
    prompt := BuildCommitPrompt("Test task", files)

    for _, f := range files {
        if !strings.Contains(prompt, f) {
            t.Errorf("prompt should contain file %s", f)
        }
    }
}

func TestBuildCommitPrompt_EmptyFileList(t *testing.T) {
    prompt := BuildCommitPrompt("Test task", nil)

    if !strings.Contains(prompt, "(no files listed)") {
        t.Error("prompt should handle empty file list")
    }
}

func TestBuildPushPrompt_IncludesBranch(t *testing.T) {
    prompt := BuildPushPrompt("ralph/my-feature-abc123")

    if !strings.Contains(prompt, "ralph/my-feature-abc123") {
        t.Error("prompt should contain branch name")
    }
}

func TestBuildPushPrompt_IncludesUpstream(t *testing.T) {
    prompt := BuildPushPrompt("ralph/my-feature-abc123")

    if !strings.Contains(prompt, "git push -u origin") {
        t.Error("prompt should instruct to push with upstream tracking")
    }
}

func TestBuildPRPrompt_IncludesAllDetails(t *testing.T) {
    prompt := BuildPRPrompt("ralph/feature-xyz", "main", "User Authentication")

    if !strings.Contains(prompt, "ralph/feature-xyz") {
        t.Error("prompt should contain source branch")
    }
    if !strings.Contains(prompt, "main") {
        t.Error("prompt should contain target branch")
    }
    if !strings.Contains(prompt, "User Authentication") {
        t.Error("prompt should contain unit title")
    }
}

func TestBuildPRPrompt_IncludesGhCli(t *testing.T) {
    prompt := BuildPRPrompt("ralph/feature-xyz", "main", "User Authentication")

    if !strings.Contains(prompt, "gh pr create") {
        t.Error("prompt should instruct to use gh CLI")
    }
}

func TestFormatFileList_Empty(t *testing.T) {
    result := formatFileList(nil)
    if result != "(no files listed)" {
        t.Errorf("expected empty message, got %q", result)
    }
}

func TestFormatFileList_MultipleFiles(t *testing.T) {
    files := []string{"a.go", "b.go", "c.go"}
    result := formatFileList(files)

    for _, f := range files {
        if !strings.Contains(result, "- "+f) {
            t.Errorf("result should contain '- %s'", f)
        }
    }
}
```

## NOT In Scope

- Prompt execution (handled in delegate tasks)
- Claude invocation (handled in delegate tasks)
- Template customization or configuration
