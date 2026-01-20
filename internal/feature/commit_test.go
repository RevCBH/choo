package feature

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/git"
)

// useRunner is a helper to set a fake runner for tests
func useRunner(t *testing.T, runner git.Runner) {
	t.Helper()
	git.SetDefaultRunner(runner)
	t.Cleanup(func() {
		git.SetDefaultRunner(nil) // Reset to default
	})
}

// TestCommitSpecs_Success tests successful spec commit flow
func TestCommitSpecs_Success(t *testing.T) {
	runner := newFakeRunner()

	// Stage all files
	runner.stub("add -A", "", nil)

	// Get staged files
	runner.stub("diff --cached --name-only", "specs/tasks/test-prd/01.md\nspecs/tasks/test-prd/02.md\n", nil)

	// Commit
	runner.stub("commit -m chore(feature): add specs for test-prd --no-verify", "", nil)

	// Get commit hash
	runner.stub("rev-parse HEAD", "abc123def456\n", nil)

	// Push (succeeds on first try)
	runner.stub("push", "", nil)

	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	result, err := CommitSpecs(context.Background(), client, "test-prd")

	if err != nil {
		t.Fatalf("CommitSpecs failed: %v", err)
	}

	if result.CommitHash != "abc123def456" {
		t.Errorf("expected commit hash 'abc123def456', got '%s'", result.CommitHash)
	}

	if result.FileCount != 2 {
		t.Errorf("expected file count 2, got %d", result.FileCount)
	}

	if !result.Pushed {
		t.Error("expected Pushed to be true")
	}
}

// TestCommitSpecs_StagingFailure tests failure during staging
func TestCommitSpecs_StagingFailure(t *testing.T) {
	runner := newFakeRunner()

	// Stage fails
	runner.stub("add -A", "", errors.New("staging failed"))

	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	_, err := CommitSpecs(context.Background(), client, "test-prd")

	if err == nil {
		t.Fatal("expected error on staging failure")
	}

	if !strings.Contains(err.Error(), "failed to stage specs") {
		t.Errorf("expected 'failed to stage specs' in error, got: %v", err)
	}
}

// TestCommitSpecs_CommitFailure tests failure during commit
func TestCommitSpecs_CommitFailure(t *testing.T) {
	runner := newFakeRunner()

	// Stage succeeds
	runner.stub("add -A", "", nil)

	// Get staged files
	runner.stub("diff --cached --name-only", "specs/tasks/test-prd/01.md\n", nil)

	// Commit fails
	runner.stub("commit -m chore(feature): add specs for test-prd --no-verify", "", errors.New("commit failed"))

	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	_, err := CommitSpecs(context.Background(), client, "test-prd")

	if err == nil {
		t.Fatal("expected error on commit failure")
	}

	if !strings.Contains(err.Error(), "failed to commit specs") {
		t.Errorf("expected 'failed to commit specs' in error, got: %v", err)
	}
}

// TestCommitSpecs_PushRetry tests push retry logic
func TestCommitSpecs_PushRetry(t *testing.T) {
	runner := newFakeRunner()

	// Stage succeeds
	runner.stub("add -A", "", nil)

	// Get staged files
	runner.stub("diff --cached --name-only", "specs/tasks/test-prd/01.md\n", nil)

	// Commit succeeds
	runner.stub("commit -m chore(feature): add specs for test-prd --no-verify", "", nil)

	// Get commit hash
	runner.stub("rev-parse HEAD", "abc123\n", nil)

	// First push fails
	runner.stub("push", "", errors.New("push failed"))

	// Second push succeeds (retry)
	runner.stub("push", "", nil)

	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	result, err := CommitSpecs(context.Background(), client, "test-prd")

	if err != nil {
		t.Fatalf("CommitSpecs failed: %v", err)
	}

	if !result.Pushed {
		t.Error("expected Pushed to be true after retry")
	}

	// Verify push was called twice
	if runner.callsFor("push") != 2 {
		t.Errorf("expected push to be called twice, got %d", runner.callsFor("push"))
	}
}

// TestCommitSpecs_PushFailsAfterRetry tests push failure after exhausting retries
func TestCommitSpecs_PushFailsAfterRetry(t *testing.T) {
	runner := newFakeRunner()

	// Stage succeeds
	runner.stub("add -A", "", nil)

	// Get staged files
	runner.stub("diff --cached --name-only", "specs/tasks/test-prd/01.md\n", nil)

	// Commit succeeds
	runner.stub("commit -m chore(feature): add specs for test-prd --no-verify", "", nil)

	// Get commit hash
	runner.stub("rev-parse HEAD", "abc123\n", nil)

	// Both push attempts fail
	runner.stub("push", "", errors.New("push failed"))
	runner.stub("push", "", errors.New("push failed again"))

	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	_, err := CommitSpecs(context.Background(), client, "test-prd")

	if err == nil {
		t.Fatal("expected error after push retry exhausted")
	}

	if !strings.Contains(err.Error(), "failed to push specs after retry") {
		t.Errorf("expected 'failed to push specs after retry' in error, got: %v", err)
	}

	// Verify push was called twice (initial + 1 retry)
	if runner.callsFor("push") != 2 {
		t.Errorf("expected push to be called twice, got %d", runner.callsFor("push"))
	}
}

// TestCommitSpecs_DryRun tests dry-run mode
func TestCommitSpecs_DryRun(t *testing.T) {
	runner := newFakeRunner()
	useRunner(t, runner)

	client := &git.Client{WorktreePath: "/tmp/worktree"}
	result, err := CommitSpecsWithOptions(context.Background(), client, "test-prd", CommitOptions{
		DryRun: true,
	})

	if err != nil {
		t.Fatalf("CommitSpecsWithOptions failed: %v", err)
	}

	if result.CommitHash != "" {
		t.Errorf("expected empty commit hash in dry-run, got '%s'", result.CommitHash)
	}

	if result.FileCount != 0 {
		t.Errorf("expected file count 0 in dry-run, got %d", result.FileCount)
	}

	if result.Pushed {
		t.Error("expected Pushed to be false in dry-run")
	}

	// Verify no git commands were executed
	if len(runner.calls) > 0 {
		t.Errorf("expected no git commands in dry-run, got %d calls", len(runner.calls))
	}
}

// TestGenerateCommitMessage tests the commit message format
func TestGenerateCommitMessage(t *testing.T) {
	msg := generateCommitMessage("test-prd")
	expected := "chore(feature): add specs for test-prd"

	if msg != expected {
		t.Errorf("expected message '%s', got '%s'", expected, msg)
	}
}

// fakeRunner implementation for testing
type fakeRunner struct {
	responses map[string][]fakeResponse
	calls     []fakeCall
}

type fakeResponse struct {
	out string
	err error
}

type fakeCall struct {
	dir  string
	args []string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		responses: make(map[string][]fakeResponse),
	}
}

func (f *fakeRunner) stub(args string, out string, err error) {
	f.responses[args] = append(f.responses[args], fakeResponse{out: out, err: err})
}

func (f *fakeRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, fakeCall{dir: dir, args: append([]string(nil), args...)})
	queue := f.responses[key]
	if len(queue) == 0 {
		return "", errors.New("unexpected git call: " + key)
	}
	resp := queue[0]
	f.responses[key] = queue[1:]
	return resp.out, resp.err
}

func (f *fakeRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return f.Exec(ctx, dir, args...)
}

func (f *fakeRunner) callsFor(args ...string) int {
	key := strings.Join(args, " ")
	count := 0
	for _, call := range f.calls {
		if strings.Join(call.args, " ") == key {
			count++
		}
	}
	return count
}
