package worker

import (
	"fmt"
	"strings"

	"github.com/RevCBH/choo/internal/github"
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
