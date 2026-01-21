package worker

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewFailed, (*collected)[1].Type)
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

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewPassed, (*collected)[1].Type)
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

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, (*collected)[1].Type)
	// Note: we can't verify fix loop was called without refactoring,
	// but the config check ensures it would be called if implemented
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

	require.Len(t, *collected, 2)
	assert.Equal(t, events.CodeReviewStarted, (*collected)[0].Type)
	assert.Equal(t, events.CodeReviewIssuesFound, (*collected)[1].Type)
	// The logic ensures fix loop is not called when MaxFixIterations=0
}

// collectEvents subscribes to the event bus and collects events for testing.
// Returns a pointer to a slice that will be populated with events as they occur.
func collectEvents(bus *events.Bus) *[]events.Event {
	collected := &[]events.Event{}
	var mu sync.Mutex
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		*collected = append(*collected, e)
		mu.Unlock()
	})
	return collected
}

// waitForEvents waits for the event bus to process all pending events
func waitForEvents(bus *events.Bus) {
	// Wait a small amount of time for events to be processed by the goroutine
	time.Sleep(10 * time.Millisecond)
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
	for _, e := range *collected {
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
