package worker

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCodeReview_NilReviewer(t *testing.T) {
	w := &Worker{
		reviewer: nil, // Disabled
		events:   events.NewBus(100),
	}

	// Should return immediately without error or panic
	ctx := context.Background()
	w.runCodeReview(ctx)
	// No assertions needed - just verify no panic
}

func TestRunCodeReview_ReviewerError(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{err: errors.New("reviewer unavailable")},
		events:   eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled: true,
			Verbose: true,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	// Should emit started, then failed
	evts := collected.Get()
	require.Len(t, evts, 2)
	assert.Equal(t, events.CodeReviewStarted, evts[0].Type)
	assert.Equal(t, events.CodeReviewFailed, evts[1].Type)
}

func TestRunCodeReview_Passed(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed:  true,
				Summary: "All checks passed",
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled: true,
			Verbose: true,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	evts := collected.Get()
	require.Len(t, evts, 2)
	assert.Equal(t, events.CodeReviewStarted, evts[0].Type)
	assert.Equal(t, events.CodeReviewPassed, evts[1].Type)
}

func TestRunCodeReview_IssuesFound(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed: false,
				Issues: []provider.ReviewIssue{
					{File: "test.go", Line: 1, Message: "Test issue"},
				},
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	// Should emit: started, issues_found, fix_attempt (fails because no provider)
	evts := collected.Get()
	require.Len(t, evts, 3)
	assert.Equal(t, events.CodeReviewStarted, evts[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, evts[1].Type)
	assert.Equal(t, events.CodeReviewFixAttempt, evts[2].Type)
}

func TestRunCodeReview_IssuesFound_ZeroIterations(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	w := &Worker{
		unit: &discovery.Unit{ID: "test-unit"},
		reviewer: &mockReviewer{
			result: &provider.ReviewResult{
				Passed: false,
				Issues: []provider.ReviewIssue{
					{File: "test.go", Line: 1, Message: "Test issue"},
				},
			},
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 0, // Review-only mode
		},
	}

	ctx := context.Background()
	w.runCodeReview(ctx)

	// Wait for events to be processed
	waitForEvents(eventBus)

	evts := collected.Get()
	require.Len(t, evts, 2)
	assert.Equal(t, events.CodeReviewStarted, evts[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, evts[1].Type)
	// The logic ensures fix loop is not called when MaxFixIterations=0
}

// collectEvents subscribes to the event bus and collects events for testing.
// Returns an EventCollector with thread-safe access to events.
func collectEvents(bus *events.Bus) *events.EventCollector {
	return events.NewEventCollector(bus)
}

// waitForEvents waits for the event bus to process all pending events
func waitForEvents(bus *events.Bus) {
	bus.Wait()
}

// mockReviewer for testing
type mockReviewer struct {
	result *provider.ReviewResult
	err    error
}

func (m *mockReviewer) Review(ctx context.Context, workdir, baseBranch string) (*provider.ReviewResult, error) {
	return m.result, m.err
}

func (m *mockReviewer) Name() provider.ProviderType {
	return "mock"
}

func TestReviewFixLoop_Success(t *testing.T) {
	eventBus := events.NewBus(100)
	collected := collectEvents(eventBus)

	prov := &mockProvider{} // Succeeds
	worktreePath := t.TempDir()

	// Create a file so commitReviewFixes has something to commit
	testFile := filepath.Join(worktreePath, "test.go")
	os.WriteFile(testFile, []byte("package test"), 0644)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       eventBus,
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
			Verbose:          true,
		},
	}

	// Initialize git repo for test
	exec.Command("git", "init", worktreePath).Run()
	exec.Command("git", "-C", worktreePath, "add", ".").Run()

	issues := []provider.ReviewIssue{
		{File: "main.go", Line: 10, Severity: "error", Message: "unused variable"},
	}

	ctx := context.Background()
	result := w.runReviewFixLoop(ctx, issues)

	assert.True(t, result, "expected fix loop to succeed")
	assert.True(t, prov.invoked, "expected provider to be invoked")

	// Wait for events to be processed
	waitForEvents(eventBus)

	// Should emit fix_attempt and fix_applied
	var hasAttempt, hasApplied bool
	for _, e := range collected.Get() {
		if e.Type == events.CodeReviewFixAttempt {
			hasAttempt = true
		}
		if e.Type == events.CodeReviewFixApplied {
			hasApplied = true
		}
	}
	assert.True(t, hasAttempt, "expected CodeReviewFixAttempt event")
	assert.True(t, hasApplied, "expected CodeReviewFixApplied event")
}

func TestReviewFixLoop_ProviderError(t *testing.T) {
	prov := &mockProvider{invokeError: errors.New("provider failed")}
	worktreePath := t.TempDir()

	// Initialize git repo for cleanup operations
	exec.Command("git", "init", worktreePath).Run()

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 2,
			Verbose:          false, // Suppress warnings in test output
		},
	}

	issues := []provider.ReviewIssue{
		{Severity: "error", Message: "issue"},
	}

	ctx := context.Background()
	result := w.runReviewFixLoop(ctx, issues)

	assert.False(t, result, "expected fix loop to fail")
	assert.Equal(t, 2, prov.invokeCount, "expected 2 invoke attempts")
}

func TestReviewFixLoop_CommitError(t *testing.T) {
	prov := &mockProvider{} // Succeeds
	worktreePath := t.TempDir()

	// Initialize git repo
	exec.Command("git", "init", worktreePath).Run()
	exec.Command("git", "-C", worktreePath, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", worktreePath, "config", "user.name", "Test").Run()

	// Create initial commit so git works properly
	testFile := filepath.Join(worktreePath, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", worktreePath, "add", ".").Run()
	exec.Command("git", "-C", worktreePath, "commit", "-m", "initial").Run()

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
			Verbose:          false,
		},
	}

	issues := []provider.ReviewIssue{
		{Severity: "error", Message: "issue"},
	}

	ctx := context.Background()
	result := w.runReviewFixLoop(ctx, issues)

	// Should fail because provider doesn't create any changes to commit
	assert.False(t, result, "expected fix loop to fail when no changes made")
}

func TestReviewFixLoop_NoChanges(t *testing.T) {
	prov := &mockProvider{} // Succeeds but makes no changes
	worktreePath := t.TempDir()

	// Initialize git repo
	exec.Command("git", "init", worktreePath).Run()

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
			Verbose:          false,
		},
	}

	issues := []provider.ReviewIssue{
		{Severity: "error", Message: "issue"},
	}

	ctx := context.Background()
	result := w.runReviewFixLoop(ctx, issues)

	assert.False(t, result, "expected fix loop to return false when no changes made")
	assert.True(t, prov.invoked, "expected provider to be invoked")
}

func TestReviewFixLoop_MaxIterations(t *testing.T) {
	prov := &mockProvider{} // Succeeds but makes no changes
	worktreePath := t.TempDir()

	// Initialize git repo
	exec.Command("git", "init", worktreePath).Run()

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 3,
			Verbose:          false,
		},
	}

	issues := []provider.ReviewIssue{
		{Severity: "error", Message: "issue"},
	}

	ctx := context.Background()
	result := w.runReviewFixLoop(ctx, issues)

	assert.False(t, result, "expected fix loop to fail after max iterations")
	assert.Equal(t, 3, prov.invokeCount, "expected exactly 3 invoke attempts")
}

func TestReviewFixLoop_CleanupOnExit(t *testing.T) {
	// Use a provider that creates a file but doesn't succeed (simulating partial work)
	prov := &mockProvider{
		onInvoke: func(workdir string) {
			// Simulate provider creating a file but then failing
			dirtyFile := filepath.Join(workdir, "dirty.txt")
			os.WriteFile(dirtyFile, []byte("uncommitted"), 0644)
		},
		invokeError: errors.New("provider failed after making changes"),
	}
	worktreePath := t.TempDir()

	// Initialize git repo with initial commit
	exec.Command("git", "init", worktreePath).Run()
	exec.Command("git", "-C", worktreePath, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", worktreePath, "config", "user.name", "Test").Run()
	testFile := filepath.Join(worktreePath, "initial.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", worktreePath, "add", ".").Run()
	exec.Command("git", "-C", worktreePath, "commit", "-m", "initial").Run()

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
			Verbose:          false,
		},
	}

	issues := []provider.ReviewIssue{
		{Severity: "error", Message: "issue"},
	}

	ctx := context.Background()
	w.runReviewFixLoop(ctx, issues)

	// Verify the dirty file was cleaned up after provider failure
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	_, err := os.Stat(dirtyFile)
	assert.True(t, os.IsNotExist(err), "expected dirty file to be cleaned up")
}

func TestInvokeProviderForFix_NilProvider(t *testing.T) {
	w := &Worker{
		provider: nil,
	}

	ctx := context.Background()
	err := w.invokeProviderForFix(ctx, "test prompt")

	assert.Error(t, err, "expected error when provider is nil")
	assert.Contains(t, err.Error(), "no provider configured")
}

func TestBuildReviewFixPrompt_SingleIssue(t *testing.T) {
	issues := []provider.ReviewIssue{
		{
			File:       "main.go",
			Line:       42,
			Severity:   "error",
			Message:    "undefined variable: foo",
			Suggestion: "Did you mean 'f00'?",
		},
	}

	prompt := BuildReviewFixPrompt(issues)

	assert.Contains(t, prompt, "## Issue 1: error")
	assert.Contains(t, prompt, "**File**: main.go:42")
	assert.Contains(t, prompt, "**Problem**: undefined variable: foo")
	assert.Contains(t, prompt, "**Suggestion**: Did you mean 'f00'?")
}

func TestBuildReviewFixPrompt_MultipleIssues(t *testing.T) {
	issues := []provider.ReviewIssue{
		{
			File:     "main.go",
			Line:     10,
			Severity: "error",
			Message:  "first issue",
		},
		{
			File:     "util.go",
			Line:     20,
			Severity: "warning",
			Message:  "second issue",
		},
	}

	prompt := BuildReviewFixPrompt(issues)

	assert.Contains(t, prompt, "## Issue 1: error")
	assert.Contains(t, prompt, "## Issue 2: warning")
	assert.Contains(t, prompt, "**File**: main.go:10")
	assert.Contains(t, prompt, "**File**: util.go:20")
	assert.Contains(t, prompt, "first issue")
	assert.Contains(t, prompt, "second issue")
}

func TestBuildReviewFixPrompt_NoFileLocation(t *testing.T) {
	issues := []provider.ReviewIssue{
		{
			Severity: "warning",
			Message:  "general code smell",
			// No File or Line
		},
	}

	prompt := BuildReviewFixPrompt(issues)

	assert.NotContains(t, prompt, "**File**:")
	assert.Contains(t, prompt, "**Problem**: general code smell")
}

func TestBuildReviewFixPrompt_WithSuggestion(t *testing.T) {
	issues := []provider.ReviewIssue{
		{
			File:       "test.go",
			Line:       5,
			Severity:   "error",
			Message:    "syntax error",
			Suggestion: "add semicolon",
		},
	}

	prompt := BuildReviewFixPrompt(issues)

	assert.Contains(t, prompt, "**Suggestion**: add semicolon")
}

// mockProvider for testing
type mockProvider struct {
	invokeError error
	invoked     bool
	invokeCount int
	onInvoke    func(workdir string) // Optional callback to simulate provider work
}

func (m *mockProvider) Invoke(ctx context.Context, prompt, workdir string, stdout, stderr io.Writer) error {
	m.invoked = true
	m.invokeCount++
	if m.onInvoke != nil {
		m.onInvoke(workdir)
	}
	return m.invokeError
}

func (m *mockProvider) Name() provider.ProviderType {
	return "mock"
}

func TestCommitReviewFixes_WithChanges(t *testing.T) {
	// Setup real git repo for this test
	repoDir := setupTestRepo(t)

	// Create a modified file
	testFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	require.NoError(t, err)
	assert.True(t, committed)

	// Verify commit was created
	output, err := w.runner().Exec(ctx, repoDir, "log", "-1", "--pretty=format:%s")
	require.NoError(t, err)
	assert.Equal(t, "fix: address code review feedback", output)
}

func TestCommitReviewFixes_NoChanges(t *testing.T) {
	repoDir := setupTestRepo(t) // Clean repo

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	require.NoError(t, err)
	assert.False(t, committed)
}

func TestCommitReviewFixes_StageError(t *testing.T) {
	// Use fake runner that returns changes on status but fails on add
	fakeRunner := newFakeGitRunner()
	fakeRunner.stub("status --porcelain", "M test.go\n", nil)
	fakeRunner.stub("add -A", "", errors.New("add failed"))

	w := &Worker{
		worktreePath: "/tmp/test",
		gitRunner:    fakeRunner,
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	assert.Error(t, err)
	assert.False(t, committed)
	assert.Contains(t, err.Error(), "staging changes")
}

func TestCommitReviewFixes_CommitError(t *testing.T) {
	// Use fake runner that succeeds on status and add but fails on commit
	fakeRunner := newFakeGitRunner()
	fakeRunner.stub("status --porcelain", "M test.go\n", nil)
	fakeRunner.stub("add -A", "", nil)
	fakeRunner.stub("commit -m fix: address code review feedback --no-verify", "", errors.New("commit failed"))

	w := &Worker{
		worktreePath: "/tmp/test",
		gitRunner:    fakeRunner,
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	assert.Error(t, err)
	assert.False(t, committed)
	assert.Contains(t, err.Error(), "committing changes")
}

func TestCommitReviewFixes_CommitMessage(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create a modified file
	testFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	require.NoError(t, err)
	require.True(t, committed)

	// Verify standardized commit message
	output, err := w.runner().Exec(ctx, repoDir, "log", "-1", "--pretty=format:%s")
	require.NoError(t, err)
	assert.Equal(t, "fix: address code review feedback", output)
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	repoDir := setupTestRepo(t)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.False(t, hasChanges)
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Modify existing file
	testFile := filepath.Join(repoDir, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestHasUncommittedChanges_Untracked(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create an untracked file
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	require.NoError(t, os.WriteFile(untrackedFile, []byte("new file"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestCleanupWorktree_ResetsStaged(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create and stage a file
	testFile := filepath.Join(repoDir, "staged.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("staged"), 0644))

	runner := git.DefaultRunner()
	ctx := context.Background()
	_, err := runner.Exec(ctx, repoDir, "add", "staged.txt")
	require.NoError(t, err)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	// Cleanup should reset staged changes
	w.cleanupWorktree(ctx)

	// Verify no staged changes
	output, err := runner.Exec(ctx, repoDir, "diff", "--cached", "--name-only")
	require.NoError(t, err)
	assert.Empty(t, output)
}

func TestCleanupWorktree_CleansUntracked(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create untracked file
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	require.NoError(t, os.WriteFile(untrackedFile, []byte("untracked"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)

	// Verify untracked file was removed
	_, err := os.Stat(untrackedFile)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupWorktree_RestoresModified(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Modify existing file
	readmePath := filepath.Join(repoDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("modified content"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)

	// Verify file was restored to original state
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	assert.Equal(t, "# Test", string(content))
}

func TestCleanupWorktree_FullCleanup(t *testing.T) {
	repoDir := setupTestRepo(t)

	// Create dirty state: modified file + untracked file + staged file
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "modified.txt"), []byte("mod"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("new"), 0644))

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)

	// Verify worktree is clean
	output, err := w.runner().Exec(ctx, repoDir, "status", "--porcelain")
	require.NoError(t, err)
	assert.Empty(t, output, "worktree should be clean after cleanup")
}

func TestCleanupWorktree_ContinuesOnError(t *testing.T) {
	// Use fake runner that fails on reset but succeeds on clean/checkout
	fakeRunner := newFakeGitRunner()
	fakeRunner.stub("reset HEAD", "", errors.New("reset failed"))
	fakeRunner.stub("clean -fd", "", nil)
	fakeRunner.stub("checkout .", "", nil)

	w := &Worker{
		worktreePath: "/tmp/test",
		gitRunner:    fakeRunner,
		reviewConfig: &config.CodeReviewConfig{Verbose: false}, // Set to false to avoid stderr output in tests
	}

	ctx := context.Background()
	// Should not panic even if reset fails
	w.cleanupWorktree(ctx)

	// All three commands should have been called
	// (verified by not having an "unexpected git call" error)
}

// setupTestRepo creates a git repo for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runner := git.DefaultRunner()
	ctx := context.Background()

	_, err := runner.Exec(ctx, dir, "init")
	require.NoError(t, err)

	_, err = runner.Exec(ctx, dir, "config", "user.email", "test@test.com")
	require.NoError(t, err)

	_, err = runner.Exec(ctx, dir, "config", "user.name", "Test")
	require.NoError(t, err)

	// Create initial commit
	readmePath := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("# Test"), 0644))

	_, err = runner.Exec(ctx, dir, "add", ".")
	require.NoError(t, err)

	_, err = runner.Exec(ctx, dir, "commit", "-m", "Initial commit")
	require.NoError(t, err)

	return dir
}
