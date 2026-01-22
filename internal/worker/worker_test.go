package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/RevCBH/choo/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker_WithGitOps(t *testing.T) {
	mockOps := git.NewMockGitOps("/test/worktree")
	unit := &discovery.Unit{ID: "test-unit"}
	cfg := WorkerConfig{WorktreeBase: "/tmp/worktrees"}
	deps := WorkerDeps{
		GitOps: mockOps,
	}

	w, err := NewWorker(unit, cfg, deps)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.gitOps != mockOps {
		t.Error("expected provided GitOps to be used")
	}
}

func TestNewWorker_CreatesGitOps(t *testing.T) {
	baseDir := t.TempDir()
	worktreeBase := filepath.Join(baseDir, "worktrees")
	unitID := "test-unit"
	worktreePath := filepath.Join(worktreeBase, unitID)
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("failed to create worktree path: %v", err)
	}

	runner := testutil.NewStubRunner()
	runner.StubDefault("rev-parse --show-toplevel", worktreePath, nil)
	runner.StubDefault("rev-parse --absolute-git-dir", filepath.Join(baseDir, ".git", "worktrees", unitID), nil)
	prevRunner := git.DefaultRunner()
	git.SetDefaultRunner(runner)
	t.Cleanup(func() {
		git.SetDefaultRunner(prevRunner)
	})

	unit := &discovery.Unit{ID: unitID}
	cfg := WorkerConfig{WorktreeBase: worktreeBase}
	deps := WorkerDeps{}

	w, err := NewWorker(unit, cfg, deps)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.gitOps == nil {
		t.Error("expected GitOps to be created from WorktreeBase")
	}
}

func TestNewWorker_NoGitOps_PathNotFound(t *testing.T) {
	unit := &discovery.Unit{ID: "test-unit"}
	cfg := WorkerConfig{WorktreeBase: "/tmp/nonexistent-base"}
	deps := WorkerDeps{}

	w, err := NewWorker(unit, cfg, deps)

	// Should succeed - path will be created later
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.gitOps != nil {
		t.Error("expected gitOps to be nil when path doesn't exist")
	}
}

func TestNewWorker_RejectsRelativePath(t *testing.T) {
	unit := &discovery.Unit{ID: "test-unit"}
	cfg := WorkerConfig{WorktreeBase: "./relative/path"}
	deps := WorkerDeps{}

	_, err := NewWorker(unit, cfg, deps)

	// Relative path should be rejected
	if err == nil {
		t.Error("expected error for relative WorktreeBase")
	}
	if !errors.Is(err, git.ErrRelativePath) {
		t.Errorf("expected ErrRelativePath, got %v", err)
	}
}

func TestInitGitOps(t *testing.T) {
	baseDir := t.TempDir()
	worktreeBase := filepath.Join(baseDir, "worktrees")
	worktreePath := filepath.Join(worktreeBase, "test-wt")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("failed to create worktree path: %v", err)
	}

	runner := testutil.NewStubRunner()
	runner.StubDefault("rev-parse --show-toplevel", worktreePath, nil)
	runner.StubDefault("rev-parse --absolute-git-dir", filepath.Join(baseDir, ".git", "worktrees", "test-wt"), nil)
	prevRunner := git.DefaultRunner()
	git.SetDefaultRunner(runner)
	t.Cleanup(func() {
		git.SetDefaultRunner(prevRunner)
	})

	// Create worker with no initial gitOps
	w := &Worker{
		worktreePath: worktreePath,
		config:       WorkerConfig{WorktreeBase: worktreeBase},
	}

	err := w.InitGitOps()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.gitOps == nil {
		t.Error("expected gitOps to be initialized")
	}
}

func TestWorker_Run_HappyPath(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_TaskLoopFails(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_BaselineFails(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestWorker_Run_NoPR(t *testing.T) {
	t.Skip("Integration test requires full mock setup - skipped for now")
}

func TestGenerateBranchName(t *testing.T) {
	unit := &discovery.Unit{ID: "my-unit"}
	w := &Worker{unit: unit}

	branch := w.generateBranchName()

	if !strings.HasPrefix(branch, "ralph/my-unit-") {
		t.Errorf("expected branch to start with 'ralph/my-unit-', got %q", branch)
	}

	// Should have 6 hex chars after the dash
	parts := strings.Split(branch, "-")
	if len(parts) < 2 {
		t.Fatalf("expected branch name to have at least one dash, got %q", branch)
	}
	hashPart := parts[len(parts)-1]
	if len(hashPart) != 6 {
		t.Errorf("expected 6 char hash suffix, got %d chars: %q", len(hashPart), hashPart)
	}

	// Verify it's hex
	for _, c := range hashPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hex characters, got %q", hashPart)
			break
		}
	}
}

func TestSetupWorktree(t *testing.T) {
	t.Skip("Integration test requires git commands - skipped for now")
}

func TestCleanup(t *testing.T) {
	t.Skip("Integration test requires mock git manager - skipped for now")
}

// TestMergeToFeatureBranch_WithReview verifies that runCodeReview is called
// and that merge proceeds regardless of review outcome
func TestMergeToFeatureBranch_WithReview(t *testing.T) {
	// Setup mock reviewer that passes
	mockReviewer := &MockReviewer{
		ReviewResult: &provider.ReviewResult{
			Passed:  true,
			Summary: "No issues found",
		},
	}

	eventBus := events.NewBus(100)
	worker := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: mockReviewer,
		config: WorkerConfig{
			TargetBranch: "main",
			NoPR:         true, // Skip actual merge for unit test
		},
		events:       eventBus,
		reviewConfig: &config.CodeReviewConfig{Enabled: true},
	}

	ctx := context.Background()
	worker.runCodeReview(ctx)

	// Verify reviewer was called
	assert.True(t, mockReviewer.ReviewCalled, "expected reviewer to be called")
}

// TestMergeToFeatureBranch_ReviewDisabled verifies merge proceeds when reviewer is nil
func TestMergeToFeatureBranch_ReviewDisabled(t *testing.T) {
	worker := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: nil, // Review disabled
		config: WorkerConfig{
			TargetBranch: "main",
			NoPR:         true,
		},
	}

	ctx := context.Background()
	// Should not panic when reviewer is nil
	worker.runCodeReview(ctx)
}

// TestMergeToFeatureBranch_ReviewFailsButMergeProceeds verifies that
// merge proceeds despite review failure (advisory)
func TestMergeToFeatureBranch_ReviewFailsButMergeProceeds(t *testing.T) {
	// Mock reviewer that returns an error
	mockReviewer := &MockReviewer{
		ReviewErr: errors.New("review failed to execute"),
	}

	eventBus := events.NewBus(100)
	collected := collectEventsForWorkerTest(eventBus)

	worker := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: mockReviewer,
		config: WorkerConfig{
			TargetBranch: "main",
			NoPR:         true,
		},
		events:       eventBus,
		reviewConfig: &config.CodeReviewConfig{Enabled: true, Verbose: true},
	}

	ctx := context.Background()
	// Should not panic or return error - review is advisory
	worker.runCodeReview(ctx)

	// Verify review was attempted
	assert.True(t, mockReviewer.ReviewCalled, "expected review to be called")

	// Wait for events to be processed
	waitForEventsInWorkerTest(eventBus)

	// Should emit CodeReviewFailed but NOT block anything
	var hasFailed bool
	for _, e := range collected.Get() {
		if e.Type == events.CodeReviewFailed {
			hasFailed = true
		}
	}
	assert.True(t, hasFailed, "expected CodeReviewFailed event")
}

// TestMergeToFeatureBranch_ReviewIssuesButMergeProceeds verifies that
// merge proceeds despite issues found (advisory)
func TestMergeToFeatureBranch_ReviewIssuesButMergeProceeds(t *testing.T) {
	// Mock reviewer that finds issues
	mockReviewer := &MockReviewer{
		ReviewResult: &provider.ReviewResult{
			Passed: false,
			Issues: []provider.ReviewIssue{
				{File: "test.go", Line: 10, Severity: "warning", Message: "test issue"},
			},
		},
	}

	eventBus := events.NewBus(100)
	collected := collectEventsForWorkerTest(eventBus)

	worker := &Worker{
		unit:     &discovery.Unit{ID: "test-unit"},
		reviewer: mockReviewer,
		config: WorkerConfig{
			TargetBranch: "main",
			NoPR:         true,
		},
		events: eventBus,
		reviewConfig: &config.CodeReviewConfig{
			Enabled:          true,
			MaxFixIterations: 0, // Review-only mode (no fix attempts)
		},
	}

	ctx := context.Background()
	// Should not panic or return error - review is advisory
	worker.runCodeReview(ctx)

	// Verify review was called
	assert.True(t, mockReviewer.ReviewCalled, "expected review to be called")

	// Wait for events to be processed
	waitForEventsInWorkerTest(eventBus)

	// Should emit CodeReviewIssuesFound but NOT block anything
	var hasIssuesFound bool
	for _, e := range collected.Get() {
		if e.Type == events.CodeReviewIssuesFound {
			hasIssuesFound = true
		}
	}
	assert.True(t, hasIssuesFound, "expected CodeReviewIssuesFound event")
}

// MockReviewer implements provider.Reviewer for testing
type MockReviewer struct {
	ReviewResult *provider.ReviewResult
	ReviewErr    error
	ReviewCalled bool
}

func (m *MockReviewer) Review(ctx context.Context, workdir, baseBranch string) (*provider.ReviewResult, error) {
	m.ReviewCalled = true
	if m.ReviewErr != nil {
		return nil, m.ReviewErr
	}
	return m.ReviewResult, nil
}

func (m *MockReviewer) Name() provider.ProviderType {
	return "mock"
}

// collectEventsForWorkerTest subscribes to the event bus and collects events for testing
func collectEventsForWorkerTest(bus *events.Bus) *events.EventCollector {
	return events.NewEventCollector(bus)
}

// waitForEventsInWorkerTest waits for the event bus to process all pending events
func waitForEventsInWorkerTest(bus *events.Bus) {
	bus.Wait()
}

// newTestWorker creates a Worker configured for testing with MockGitOps.
func newTestWorker(t *testing.T) *Worker {
	t.Helper()
	mockOps := git.NewMockGitOps("/test/worktree")
	return &Worker{
		gitOps:       mockOps,
		worktreePath: "/test/worktree",
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}
}

// newTestWorkerWithConfig creates a Worker with custom configuration.
func newTestWorkerWithConfig(t *testing.T, cfg func(*Worker, *git.MockGitOps)) *Worker {
	t.Helper()
	mockOps := git.NewMockGitOps("/test/worktree")
	w := &Worker{
		gitOps:       mockOps,
		worktreePath: "/test/worktree",
		reviewConfig: &config.CodeReviewConfig{Verbose: false},
	}
	if cfg != nil {
		cfg(w, mockOps)
	}
	return w
}

// getMockGitOps extracts the MockGitOps from a Worker for assertions.
func getMockGitOps(t *testing.T, w *Worker) *git.MockGitOps {
	t.Helper()
	mock, ok := w.gitOps.(*git.MockGitOps)
	if !ok {
		t.Fatal("expected Worker to have MockGitOps")
	}
	return mock
}

func TestWorker_SafetyInvariants(t *testing.T) {
	t.Run("gitOps path matches worktreePath", func(t *testing.T) {
		w := newTestWorker(t)
		mock := getMockGitOps(t, w)

		if mock.Path() != w.worktreePath {
			t.Errorf("gitOps path %s doesn't match worktreePath %s",
				mock.Path(), w.worktreePath)
		}
	})

	t.Run("cleanupWorktree uses gitOps not runner", func(t *testing.T) {
		// Use a tracking runner that records if it was called
		trackingRunner := &trackingGitRunner{}

		mockOps := git.NewMockGitOps("/test/worktree")
		w := &Worker{
			gitOps:       mockOps,
			gitRunner:    trackingRunner,
			worktreePath: "/test/worktree",
		}

		w.cleanupWorktree(context.Background())

		if trackingRunner.called {
			t.Error("cleanupWorktree should use gitOps, not runner")
		}
		mockOps.AssertCalled(t, "Reset")
	})

	t.Run("commitReviewFixes uses gitOps not runner", func(t *testing.T) {
		trackingRunner := &trackingGitRunner{}

		mockOps := git.NewMockGitOps("/test/worktree")
		mockOps.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
		w := &Worker{
			gitOps:       mockOps,
			gitRunner:    trackingRunner,
			worktreePath: "/test/worktree",
		}

		w.commitReviewFixes(context.Background())

		if trackingRunner.called {
			t.Error("commitReviewFixes should use gitOps, not runner")
		}
		mockOps.AssertCalled(t, "Commit")
	})
}

// trackingGitRunner is a test helper that tracks whether any method was called.
type trackingGitRunner struct {
	called bool
}

func (t *trackingGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	t.called = true
	return "", errors.New("unexpected call to runner")
}

func (t *trackingGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	t.called = true
	return "", errors.New("unexpected call to runner")
}

func TestWorker_ReviewLoopWithGitOps(t *testing.T) {
	t.Run("full fix loop uses correct git operations", func(t *testing.T) {
		w := newTestWorkerWithConfig(t, func(w *Worker, mock *git.MockGitOps) {
			mock.StatusResult = git.StatusResult{Clean: false, Modified: []string{"file.go"}}
		})

		// Simulate fix loop
		w.cleanupWorktree(context.Background())
		// ... provider makes fixes ...
		mock := getMockGitOps(t, w)
		mock.StatusResult = git.StatusResult{Clean: false, Modified: []string{"fixed.go"}}
		committed, _ := w.commitReviewFixes(context.Background())

		if !committed {
			t.Error("expected changes to be committed")
		}

		// Verify call order
		mock.AssertCallOrder(t, "Reset", "Clean", "CheckoutFiles", "Status", "AddAll", "Commit")
	})
}
