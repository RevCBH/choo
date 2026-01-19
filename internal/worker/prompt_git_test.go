package worker

import (
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/github"
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

func TestFormatFileList(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go"}
	result := formatFileList(files)

	// Check that all files are present in the result
	for _, f := range files {
		if !strings.Contains(result, "- "+f) {
			t.Errorf("result should contain '- %s'", f)
		}
	}
}

func TestFormatFileList_Empty(t *testing.T) {
	result := formatFileList(nil)
	if result != "(no files listed)" {
		t.Errorf("expected empty message, got %q", result)
	}
}

func TestFormatFileList_SingleFile(t *testing.T) {
	result := formatFileList([]string{"only.go"})
	if !strings.Contains(result, "- only.go") {
		t.Errorf("result should contain '- only.go', got %q", result)
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

func TestBuildFeedbackPrompt_IncludesAllComments(t *testing.T) {
	comments := []github.PRComment{
		{Author: "alice", Body: "Fix the null check", Path: "main.go", Line: 42},
		{Author: "bob", Body: "Add tests"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/123", comments)

	if !strings.Contains(prompt, "@alice: Fix the null check") {
		t.Error("prompt should contain @alice: Fix the null check")
	}
	if !strings.Contains(prompt, "(on main.go:42)") {
		t.Error("prompt should contain (on main.go:42)")
	}
	if !strings.Contains(prompt, "@bob: Add tests") {
		t.Error("prompt should contain @bob: Add tests")
	}
	if !strings.Contains(prompt, "pull/123") {
		t.Error("prompt should contain pull/123")
	}
}

func TestBuildFeedbackPrompt_HandlesNoPath(t *testing.T) {
	comments := []github.PRComment{
		{Author: "reviewer", Body: "General comment about the PR"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/456", comments)

	if !strings.Contains(prompt, "@reviewer: General comment about the PR") {
		t.Error("prompt should contain @reviewer: General comment about the PR")
	}
	if strings.Contains(prompt, "(on ") {
		t.Error("prompt should not contain '(on ' for comments without a path")
	}
}

func TestBuildFeedbackPrompt_EmptyComments(t *testing.T) {
	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/789", []github.PRComment{})

	if !strings.Contains(prompt, "pull/789") {
		t.Error("prompt should contain pull/789")
	}
	if !strings.Contains(prompt, "address review feedback") {
		t.Error("prompt should contain address review feedback")
	}
}

func TestBuildFeedbackPrompt_MultipleCommentsOnSameFile(t *testing.T) {
	comments := []github.PRComment{
		{Author: "alice", Body: "Fix line 10", Path: "main.go", Line: 10},
		{Author: "alice", Body: "Fix line 20", Path: "main.go", Line: 20},
		{Author: "bob", Body: "Fix util.go", Path: "util.go", Line: 5},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/100", comments)

	if !strings.Contains(prompt, "(on main.go:10)") {
		t.Error("prompt should contain (on main.go:10)")
	}
	if !strings.Contains(prompt, "(on main.go:20)") {
		t.Error("prompt should contain (on main.go:20)")
	}
	if !strings.Contains(prompt, "(on util.go:5)") {
		t.Error("prompt should contain (on util.go:5)")
	}
}

func TestBuildFeedbackPrompt_ContainsInstructions(t *testing.T) {
	comments := []github.PRComment{
		{Author: "reviewer", Body: "Please fix"},
	}

	prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/1", comments)

	if !strings.Contains(prompt, "Stage and commit") {
		t.Error("prompt should contain 'Stage and commit'")
	}
	if !strings.Contains(prompt, "Push the changes") {
		t.Error("prompt should contain 'Push the changes'")
	}
	if !strings.Contains(prompt, "orchestrator will continue polling") {
		t.Error("prompt should contain 'orchestrator will continue polling'")
	}
}
