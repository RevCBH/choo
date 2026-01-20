package git

import (
	"context"
	"errors"
	"testing"
)

type fakeClaude struct {
	calls int
	err   error
}

func (f *fakeClaude) Invoke(ctx context.Context, opts InvokeOptions) (string, error) {
	f.calls++
	return "", f.err
}

func useRunner(t *testing.T, runner Runner) {
	t.Helper()
	prev := DefaultRunner()
	SetDefaultRunner(runner)
	t.Cleanup(func() {
		SetDefaultRunner(prev)
	})
}

func TestRebase_ConflictDetected(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("rebase origin/main", "", errors.New("CONFLICT (content): merge conflict"))
	useRunner(t, runner)

	hasConflicts, err := Rebase(context.Background(), "/tmp/worktree", "origin/main")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hasConflicts {
		t.Error("expected conflicts to be detected")
	}
}

func TestRebase_Error(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("rebase origin/main", "", errors.New("fatal: bad revision"))
	useRunner(t, runner)

	hasConflicts, err := Rebase(context.Background(), "/tmp/worktree", "origin/main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if hasConflicts {
		t.Error("expected no conflicts for non-conflict error")
	}
}

func TestResolveConflicts_ContinuesRebase(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("diff --name-only --diff-filter=U", "conflict.txt\n", nil)
	runner.stub("diff --name-only --diff-filter=U", "", nil)
	runner.stub("rebase --continue", "", nil)
	useRunner(t, runner)

	claude := &fakeClaude{}
	mgr := NewMergeManager("/tmp/repo", claude)

	if err := mgr.ResolveConflicts(context.Background(), "/tmp/worktree"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claude.calls != 1 {
		t.Errorf("expected Claude to be invoked once, got %d", claude.calls)
	}
	if runner.callsFor("rebase --continue") != 1 {
		t.Errorf("expected rebase --continue to be called once")
	}
}

func TestMerge_FetchOnlyWithoutWorktree(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("fetch origin main", "", nil)
	useRunner(t, runner)

	mgr := NewMergeManager("/tmp/repo", nil)
	result, err := mgr.Merge(context.Background(), &Branch{
		Name:         "feature",
		TargetBranch: "main",
		Worktree:     "",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Success {
		t.Error("expected success for fetch-only merge")
	}
}

func TestMerge_ConflictWithoutClaude_Aborts(t *testing.T) {
	runner := newFakeRunner()
	runner.stub("fetch origin main", "", nil)
	runner.stub("rebase origin/main", "", errors.New("CONFLICT (content): merge conflict"))
	runner.stub("rebase --abort", "", nil)
	useRunner(t, runner)

	mgr := NewMergeManager("/tmp/repo", nil)
	_, err := mgr.Merge(context.Background(), &Branch{
		Name:         "feature",
		TargetBranch: "main",
		Worktree:     "/tmp/worktree",
	})
	if err == nil {
		t.Fatal("expected error when conflicts occur without Claude")
	}
	if runner.callsFor("rebase --abort") != 1 {
		t.Errorf("expected rebase --abort to be called")
	}
}
