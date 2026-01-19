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
