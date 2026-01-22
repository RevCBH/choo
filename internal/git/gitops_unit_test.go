package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/RevCBH/choo/internal/testutil"
)

func newTestGitOps(t *testing.T, runner *testutil.StubRunner, opts GitOpsOpts) GitOps {
	t.Helper()

	dir := t.TempDir()
	toplevel := dir
	runner.StubDefault("rev-parse --show-toplevel", toplevel, nil)
	if !opts.AllowRepoRoot {
		runner.StubDefault("rev-parse --absolute-git-dir", filepath.Join(dir, ".git", "worktrees", "unit"), nil)
	}

	ops, err := newGitOpsWithRunner(dir, opts, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return ops
}

func TestNewGitOps_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	runner := testutil.NewStubRunner()
	runner.Stub("rev-parse --show-toplevel", "", errors.New("no git"))

	_, err := newGitOpsWithRunner(dir, GitOpsOpts{AllowRepoRoot: true}, runner)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestParseStatusOutput(t *testing.T) {
	result := parseStatusOutput("")
	if !result.Clean {
		t.Fatalf("expected clean for empty status output")
	}

	result = parseStatusOutput(" M file.txt\n")
	if result.Clean || len(result.Modified) != 1 || result.Modified[0] != "file.txt" {
		t.Fatalf("expected modified file, got %+v", result)
	}

	result = parseStatusOutput("A  staged.txt\n")
	if result.Clean || len(result.Staged) != 1 || result.Staged[0] != "staged.txt" {
		t.Fatalf("expected staged file, got %+v", result)
	}

	result = parseStatusOutput("?? new.txt\n")
	if result.Clean || len(result.Untracked) != 1 || result.Untracked[0] != "new.txt" {
		t.Fatalf("expected untracked file, got %+v", result)
	}

	result = parseStatusOutput("UU conflict.txt\n")
	if result.Clean || len(result.Conflicted) != 1 || result.Conflicted[0] != "conflict.txt" {
		t.Fatalf("expected conflicted file, got %+v", result)
	}
}

func TestBranchExists_Local(t *testing.T) {
	runner := testutil.NewStubRunner()
	ops := newTestGitOps(t, runner, GitOpsOpts{AllowRepoRoot: true})

	runner.Stub("rev-parse --verify feature", "sha", nil)
	exists, err := ops.BranchExists(context.Background(), "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected branch to exist locally")
	}
}

func TestBranchExists_Remote(t *testing.T) {
	runner := testutil.NewStubRunner()
	ops := newTestGitOps(t, runner, GitOpsOpts{AllowRepoRoot: true})

	runner.Stub("rev-parse --verify feature", "", errors.New("not found"))
	runner.Stub("rev-parse --verify origin/feature", "sha", nil)

	exists, err := ops.BranchExists(context.Background(), "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected branch to exist on remote")
	}
}

func TestBranchExists_Missing(t *testing.T) {
	runner := testutil.NewStubRunner()
	ops := newTestGitOps(t, runner, GitOpsOpts{AllowRepoRoot: true})

	runner.Stub("rev-parse --verify missing", "", errors.New("not found"))
	runner.Stub("rev-parse --verify origin/missing", "", errors.New("not found"))

	exists, err := ops.BranchExists(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected branch to be missing")
	}
}
