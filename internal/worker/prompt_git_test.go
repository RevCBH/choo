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
