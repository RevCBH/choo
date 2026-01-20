package git

import (
	"context"
	"testing"
)

func TestStageAll_UsesGitAdd(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("add -A", "", nil)
	useRunner(t, runner)

	if err := StageAll(context.Background(), "/tmp/worktree"); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if runner.callsFor("add -A") != 1 {
		t.Errorf("expected git add -A to be called once")
	}
}

func TestCommit_BuildsArgs(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("commit -m test --no-verify --allow-empty", "", nil)
	useRunner(t, runner)

	err := Commit(context.Background(), "/tmp/worktree", CommitOptions{
		Message:    "test",
		NoVerify:   true,
		AllowEmpty: true,
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if runner.callsFor("commit -m test --no-verify --allow-empty") != 1 {
		t.Errorf("expected commit with no-verify and allow-empty")
	}
}

func TestHasUncommittedChanges_ParsesStatus(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("status --porcelain", " M file.txt\n", nil)
	useRunner(t, runner)

	hasChanges, err := HasUncommittedChanges(context.Background(), "/tmp/worktree")
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}
	if !hasChanges {
		t.Error("expected uncommitted changes")
	}
}

func TestGetStagedFiles_ParsesOutput(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("diff --cached --name-only", "a.txt\nb.txt\n", nil)
	useRunner(t, runner)

	files, err := GetStagedFiles(context.Background(), "/tmp/worktree")
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "a.txt" || files[1] != "b.txt" {
		t.Errorf("unexpected files: %v", files)
	}
}
