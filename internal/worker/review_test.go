package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "M test.go\n", nil)
	runner.stub("add -A", "", nil)
	runner.stub("commit -m fix: address code review feedback --no-verify", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       eventBus,
		worktreePath: worktreePath,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 1,
			Verbose:          true,
		},
	}

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
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    runner,
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
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    runner,
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
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    runner,
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
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    runner,
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
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		unit:         &discovery.Unit{ID: "test-unit"},
		provider:     prov,
		events:       events.NewBus(100),
		worktreePath: worktreePath,
		gitRunner:    runner,
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

	assert.Equal(t, 2, runner.callsFor("reset", "HEAD"))
	assert.Equal(t, 2, runner.callsFor("clean", "-fd"))
	assert.Equal(t, 2, runner.callsFor("checkout", "."))
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
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "M test.txt\n", nil)
	runner.stub("add -A", "", nil)
	runner.stub("commit -m fix: address code review feedback --no-verify", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	require.NoError(t, err)
	assert.True(t, committed)
}

func TestCommitReviewFixes_NoChanges(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
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
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "M test.txt\n", nil)
	runner.stub("add -A", "", nil)
	runner.stub("commit -m fix: address code review feedback --no-verify", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	ctx := context.Background()
	committed, err := w.commitReviewFixes(ctx)

	require.NoError(t, err)
	require.True(t, committed)
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.False(t, hasChanges)
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", " M README.md\n", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestHasUncommittedChanges_Untracked(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "?? untracked.txt\n", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	ctx := context.Background()
	hasChanges, err := w.hasUncommittedChanges(ctx)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestCleanupWorktree_ResetsStaged(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	// Cleanup should reset staged changes
	ctx := context.Background()
	w.cleanupWorktree(ctx)
	assert.Equal(t, 1, runner.callsFor("reset", "HEAD"))
	assert.Equal(t, 1, runner.callsFor("clean", "-fd"))
	assert.Equal(t, 1, runner.callsFor("checkout", "."))
}

func TestCleanupWorktree_CleansUntracked(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)

	assert.Equal(t, 1, runner.callsFor("reset", "HEAD"))
	assert.Equal(t, 1, runner.callsFor("clean", "-fd"))
	assert.Equal(t, 1, runner.callsFor("checkout", "."))
}

func TestCleanupWorktree_RestoresModified(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)
	assert.Equal(t, 1, runner.callsFor("reset", "HEAD"))
	assert.Equal(t, 1, runner.callsFor("clean", "-fd"))
	assert.Equal(t, 1, runner.callsFor("checkout", "."))
}

func TestCleanupWorktree_FullCleanup(t *testing.T) {
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("reset HEAD", "", nil)
	runner.stub("clean -fd", "", nil)
	runner.stub("checkout .", "", nil)

	w := &Worker{
		worktreePath: repoDir,
		gitRunner:    runner,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	ctx := context.Background()
	w.cleanupWorktree(ctx)

	assert.Equal(t, 1, runner.callsFor("reset", "HEAD"))
	assert.Equal(t, 1, runner.callsFor("clean", "-fd"))
	assert.Equal(t, 1, runner.callsFor("checkout", "."))
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

// === GitOps-based cleanupWorktree tests ===

func TestCleanupWorktree_UsesGitOps(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	w.cleanupWorktree(context.Background())

	mockOps.AssertCalled(t, "Reset")
	mockOps.AssertCalled(t, "Clean")
	mockOps.AssertCalled(t, "CheckoutFiles")

	// Verify Clean was called with correct options
	cleanCalls := mockOps.GetCallsFor("Clean")
	if len(cleanCalls) != 1 {
		t.Fatalf("expected 1 Clean call, got %d", len(cleanCalls))
	}
	opts := cleanCalls[0].Args[0].(git.CleanOpts)
	if !opts.Force {
		t.Error("expected Force=true for Clean")
	}
	if !opts.Directories {
		t.Error("expected Directories=true for Clean")
	}
}

func TestCleanupWorktree_NilGitOps_NoOp(t *testing.T) {
	w := &Worker{
		gitOps:       nil,
		worktreePath: "", // Prevents legacy fallback from doing anything
	}

	// Should not panic
	w.cleanupWorktree(context.Background())
}

func TestCleanupWorktree_ResetError_Continues(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.ResetErr = errors.New("reset failed")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}

	w.cleanupWorktree(context.Background())

	// Should still call Clean and CheckoutFiles despite Reset error
	mockOps.AssertCalled(t, "Clean")
	mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CleanError_Continues(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.CleanErr = errors.New("clean failed")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}

	w.cleanupWorktree(context.Background())

	// Should still call CheckoutFiles despite Clean error
	mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CleanOpts(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{gitOps: mockOps}

	w.cleanupWorktree(context.Background())

	cleanCalls := mockOps.GetCallsFor("Clean")
	if len(cleanCalls) != 1 {
		t.Fatalf("expected 1 Clean call, got %d", len(cleanCalls))
	}
	opts := cleanCalls[0].Args[0].(git.CleanOpts)
	if !opts.Force {
		t.Error("expected Force=true for Clean")
	}
	if !opts.Directories {
		t.Error("expected Directories=true for Clean")
	}
}

func TestCleanupWorktree_CheckoutFiles_Dot(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{gitOps: mockOps}

	w.cleanupWorktree(context.Background())

	checkoutCalls := mockOps.GetCallsFor("CheckoutFiles")
	if len(checkoutCalls) != 1 {
		t.Fatalf("expected 1 CheckoutFiles call, got %d", len(checkoutCalls))
	}
	paths := checkoutCalls[0].Args[0].([]string)
	if len(paths) != 1 || paths[0] != "." {
		t.Errorf("expected CheckoutFiles to be called with [\".\"], got %v", paths)
	}
}

// === GitOps-based commitReviewFixes tests ===

func TestCommitReviewFixes_GitOps_NoChanges(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: true}

	w := &Worker{gitOps: mockOps}

	committed, err := w.commitReviewFixes(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if committed {
		t.Error("expected committed=false when no changes")
	}
	mockOps.AssertCalled(t, "Status")
	mockOps.AssertNotCalled(t, "AddAll")
	mockOps.AssertNotCalled(t, "Commit")
}

func TestCommitReviewFixes_GitOps_WithChanges(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}

	committed, err := w.commitReviewFixes(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !committed {
		t.Error("expected committed=true when changes exist")
	}
	mockOps.AssertCalled(t, "AddAll")
	mockOps.AssertCalled(t, "Commit")
}

func TestCommitReviewFixes_GitOps_ProtectedBranch(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
	mockOps.CommitErr = fmt.Errorf("%w: main", git.ErrProtectedBranch)

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error for protected branch")
	}
	if !errors.Is(err, git.ErrProtectedBranch) {
		t.Errorf("expected error to wrap ErrProtectedBranch, got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_StatusError(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusErr = errors.New("status failed")

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error from Status")
	}
	if !strings.Contains(err.Error(), "checking for changes") {
		t.Errorf("expected error to contain 'checking for changes', got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_AddAllError(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
	mockOps.AddAllErr = errors.New("add failed")

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error from AddAll")
	}
	if !strings.Contains(err.Error(), "staging changes") {
		t.Errorf("expected error to contain 'staging changes', got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_CommitMessage(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}
	w.commitReviewFixes(context.Background())

	commitCalls := mockOps.GetCallsFor("Commit")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
	}
	msg := commitCalls[0].Args[0].(string)
	if msg != "fix: address code review feedback" {
		t.Errorf("expected message 'fix: address code review feedback', got %s", msg)
	}
}

func TestCommitReviewFixes_GitOps_NoVerify(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}
	w.commitReviewFixes(context.Background())

	commitCalls := mockOps.GetCallsFor("Commit")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
	}
	opts := commitCalls[0].Args[1].(git.CommitOpts)
	if !opts.NoVerify {
		t.Error("expected NoVerify=true")
	}
}

// === GitOps-based hasUncommittedChanges tests ===

func TestHasUncommittedChanges_GitOps_Clean(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: true}

	w := &Worker{gitOps: mockOps}

	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hasChanges {
		t.Error("expected hasChanges=false when Clean=true")
	}
}

func TestHasUncommittedChanges_GitOps_Modified(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}

	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !hasChanges {
		t.Error("expected hasChanges=true when files modified")
	}
}

func TestHasUncommittedChanges_GitOps_NilGitOps(t *testing.T) {
	// Test that nil gitOps falls back to legacy behavior
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "", nil)

	w := &Worker{
		gitOps:       nil, // No GitOps
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	// Clean repo should return false
	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hasChanges {
		t.Error("expected hasChanges=false for clean repo")
	}
}

func TestCommitReviewFixes_GitOps_NilGitOps(t *testing.T) {
	// Test that nil gitOps falls back to legacy behavior
	repoDir := t.TempDir()
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", "M test.txt\n", nil)
	runner.stub("add -A", "", nil)
	runner.stub("commit -m fix: address code review feedback --no-verify", "", nil)

	w := &Worker{
		gitOps:       nil, // No GitOps
		worktreePath: repoDir,
		gitRunner:    runner,
	}

	committed, err := w.commitReviewFixes(context.Background())

	require.NoError(t, err)
	assert.True(t, committed)
}

// === GitOps-based cleanupWorktree tests ===

func TestCleanupWorktree_UsesGitOps(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: true},
	}

	w.cleanupWorktree(context.Background())

	mockOps.AssertCalled(t, "Reset")
	mockOps.AssertCalled(t, "Clean")
	mockOps.AssertCalled(t, "CheckoutFiles")

	// Verify Clean was called with correct options
	cleanCalls := mockOps.GetCallsFor("Clean")
	if len(cleanCalls) != 1 {
		t.Fatalf("expected 1 Clean call, got %d", len(cleanCalls))
	}
	opts := cleanCalls[0].Args[0].(git.CleanOpts)
	if !opts.Force {
		t.Error("expected Force=true for Clean")
	}
	if !opts.Directories {
		t.Error("expected Directories=true for Clean")
	}
}

func TestCleanupWorktree_NilGitOps_NoOp(t *testing.T) {
	w := &Worker{
		gitOps:       nil,
		worktreePath: "", // Prevents legacy fallback from doing anything
	}

	// Should not panic
	w.cleanupWorktree(context.Background())
}

func TestCleanupWorktree_ResetError_Continues(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.ResetErr = errors.New("reset failed")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}

	w.cleanupWorktree(context.Background())

	// Should still call Clean and CheckoutFiles despite Reset error
	mockOps.AssertCalled(t, "Clean")
	mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CleanError_Continues(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.CleanErr = errors.New("clean failed")
	w := &Worker{
		gitOps:       mockOps,
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}

	w.cleanupWorktree(context.Background())

	// Should still call CheckoutFiles despite Clean error
	mockOps.AssertCalled(t, "CheckoutFiles")
}

func TestCleanupWorktree_CleanOpts(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{gitOps: mockOps}

	w.cleanupWorktree(context.Background())

	cleanCalls := mockOps.GetCallsFor("Clean")
	if len(cleanCalls) != 1 {
		t.Fatalf("expected 1 Clean call, got %d", len(cleanCalls))
	}
	opts := cleanCalls[0].Args[0].(git.CleanOpts)
	if !opts.Force {
		t.Error("expected Force=true for Clean")
	}
	if !opts.Directories {
		t.Error("expected Directories=true for Clean")
	}
}

func TestCleanupWorktree_CheckoutFiles_Dot(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{gitOps: mockOps}

	w.cleanupWorktree(context.Background())

	checkoutCalls := mockOps.GetCallsFor("CheckoutFiles")
	if len(checkoutCalls) != 1 {
		t.Fatalf("expected 1 CheckoutFiles call, got %d", len(checkoutCalls))
	}
	paths := checkoutCalls[0].Args[0].([]string)
	if len(paths) != 1 || paths[0] != "." {
		t.Errorf("expected CheckoutFiles to be called with [\".\"], got %v", paths)
	}
}

// === GitOps-based commitReviewFixes tests ===

func TestCommitReviewFixes_GitOps_NoChanges(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: true}

	w := &Worker{gitOps: mockOps}

	committed, err := w.commitReviewFixes(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if committed {
		t.Error("expected committed=false when no changes")
	}
	mockOps.AssertCalled(t, "Status")
	mockOps.AssertNotCalled(t, "AddAll")
	mockOps.AssertNotCalled(t, "Commit")
}

func TestCommitReviewFixes_GitOps_WithChanges(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}

	committed, err := w.commitReviewFixes(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !committed {
		t.Error("expected committed=true when changes exist")
	}
	mockOps.AssertCalled(t, "AddAll")
	mockOps.AssertCalled(t, "Commit")
}

func TestCommitReviewFixes_GitOps_ProtectedBranch(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
	mockOps.CommitErr = fmt.Errorf("%w: main", git.ErrProtectedBranch)

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error for protected branch")
	}
	if !errors.Is(err, git.ErrProtectedBranch) {
		t.Errorf("expected error to wrap ErrProtectedBranch, got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_StatusError(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusErr = errors.New("status failed")

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error from Status")
	}
	if !strings.Contains(err.Error(), "checking for changes") {
		t.Errorf("expected error to contain 'checking for changes', got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_AddAllError(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
	mockOps.AddAllErr = errors.New("add failed")

	w := &Worker{gitOps: mockOps}

	_, err := w.commitReviewFixes(context.Background())

	if err == nil {
		t.Error("expected error from AddAll")
	}
	if !strings.Contains(err.Error(), "staging changes") {
		t.Errorf("expected error to contain 'staging changes', got %v", err)
	}
}

func TestCommitReviewFixes_GitOps_CommitMessage(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}
	w.commitReviewFixes(context.Background())

	commitCalls := mockOps.GetCallsFor("Commit")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
	}
	msg := commitCalls[0].Args[0].(string)
	if msg != "fix: address code review feedback" {
		t.Errorf("expected message 'fix: address code review feedback', got %s", msg)
	}
}

func TestCommitReviewFixes_GitOps_NoVerify(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}
	w.commitReviewFixes(context.Background())

	commitCalls := mockOps.GetCallsFor("Commit")
	if len(commitCalls) != 1 {
		t.Fatalf("expected 1 Commit call, got %d", len(commitCalls))
	}
	opts := commitCalls[0].Args[1].(git.CommitOpts)
	if !opts.NoVerify {
		t.Error("expected NoVerify=true")
	}
}

// === GitOps-based hasUncommittedChanges tests ===

func TestHasUncommittedChanges_GitOps_Clean(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: true}

	w := &Worker{gitOps: mockOps}

	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hasChanges {
		t.Error("expected hasChanges=false when Clean=true")
	}
}

func TestHasUncommittedChanges_GitOps_Modified(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}

	w := &Worker{gitOps: mockOps}

	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !hasChanges {
		t.Error("expected hasChanges=true when files modified")
	}
}

func TestHasUncommittedChanges_GitOps_NilGitOps(t *testing.T) {
	// Test that nil gitOps falls back to legacy behavior
	repoDir := setupTestRepo(t)

	w := &Worker{
		gitOps:       nil, // No GitOps
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	// Clean repo should return false
	hasChanges, err := w.hasUncommittedChanges(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hasChanges {
		t.Error("expected hasChanges=false for clean repo")
	}
}

func TestCommitReviewFixes_GitOps_NilGitOps(t *testing.T) {
	// Test that nil gitOps falls back to legacy behavior
	repoDir := setupTestRepo(t)

	// Create a file to commit
	testFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

	w := &Worker{
		gitOps:       nil, // No GitOps
		worktreePath: repoDir,
		gitRunner:    git.DefaultRunner(),
	}

	committed, err := w.commitReviewFixes(context.Background())

	require.NoError(t, err)
	assert.True(t, committed)

	// Verify commit was created
	output, err := w.runner().Exec(context.Background(), repoDir, "log", "-1", "--pretty=format:%s")
	require.NoError(t, err)
	assert.Equal(t, "fix: address code review feedback", output)
}
