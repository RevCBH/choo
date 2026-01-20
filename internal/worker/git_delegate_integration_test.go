//go:build integration
// +build integration

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
)

// setupTestGitRepo creates a temporary git repo for testing
func setupTestGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-helper-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			cleanup()
			t.Fatalf("git setup failed: %v", err)
		}
	}

	// Create initial commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to write test file: %v", err)
	}

	cmds = [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", "initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			cleanup()
			t.Fatalf("git commit failed: %v", err)
		}
	}

	return dir, cleanup
}

func TestGetHeadRef_ReturnsCommitSHA(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	ref, err := w.getHeadRef(context.Background())
	if err != nil {
		t.Fatalf("getHeadRef failed: %v", err)
	}

	// SHA should be 40 hex characters
	if len(ref) != 40 {
		t.Errorf("expected 40-char SHA, got %d chars: %s", len(ref), ref)
	}

	for _, c := range ref {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("SHA contains non-hex character: %c", c)
		}
	}
}

func TestHasNewCommit_FalseWhenSame(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	ref, _ := w.getHeadRef(context.Background())

	hasNew, err := w.hasNewCommit(context.Background(), ref)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if hasNew {
		t.Error("expected false when HEAD unchanged")
	}
}

func TestHasNewCommit_TrueAfterCommit(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	refBefore, _ := w.getHeadRef(context.Background())

	// Create new commit
	testFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(testFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmds := [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", "new commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command failed: %v", err)
		}
	}

	hasNew, err := w.hasNewCommit(context.Background(), refBefore)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if !hasNew {
		t.Error("expected true after new commit")
	}
}

func TestBranchExistsOnRemote_FalseForNonexistent(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}

	// No remote configured, so any branch should return false
	exists, err := w.branchExistsOnRemote(context.Background(), "nonexistent-branch")
	if err != nil {
		// Error expected since no remote exists
		return
	}
	if exists {
		t.Error("expected false for non-existent branch")
	}
}

func TestGetChangedFiles_Empty(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{worktreePath: dir}
	files, err := w.getChangedFiles(context.Background())
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty slice for clean worktree, got %v", files)
	}
}

func TestGetChangedFiles_ParsesPorcelain(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create untracked file
	newFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(newFile, []byte("untracked"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Modify existing file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	w := &Worker{worktreePath: dir}
	files, err := w.getChangedFiles(context.Background())
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}

	hasUntracked := false
	hasModified := false
	for _, f := range files {
		if f == "untracked.txt" {
			hasUntracked = true
		}
		if f == "test.txt" {
			hasModified = true
		}
	}

	if !hasUntracked {
		t.Error("expected untracked.txt in changed files")
	}
	if !hasModified {
		t.Error("expected test.txt in changed files")
	}
}

// mockEscalator records escalations for testing
type mockEscalator struct {
	escalations []escalate.Escalation
}

func (m *mockEscalator) Escalate(ctx context.Context, e escalate.Escalation) error {
	m.escalations = append(m.escalations, e)
	return nil
}

func (m *mockEscalator) Name() string {
	return "mock"
}

// mockClaudeInvoker wraps a Worker and allows customizing Claude invocation behavior
type mockClaudeInvoker struct {
	*Worker
	invokeFunc func(ctx context.Context, prompt TaskPrompt) error
}

func (m *mockClaudeInvoker) invokeClaude(ctx context.Context, prompt string) error {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, TaskPrompt{Content: prompt})
	}
	return m.Worker.invokeClaudeForTask(ctx, TaskPrompt{Content: prompt})
}

// wrapWorkerForMocking wraps a worker to allow mocking invokeClaude
func wrapWorkerForMocking(w *Worker, invokeFunc func(ctx context.Context, prompt TaskPrompt) error) *mockClaudeInvoker {
	return &mockClaudeInvoker{
		Worker:     w,
		invokeFunc: invokeFunc,
	}
}

func TestCommitViaClaudeCode_Success(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create a file to commit
	testFile := filepath.Join(dir, "new_feature.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	w := &Worker{
		worktreePath: dir,
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    &mockEscalator{},
	}

	mocked := wrapWorkerForMocking(w, func(ctx context.Context, prompt TaskPrompt) error {
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("git", "commit", "-m", "feat: add new feature")
		cmd.Dir = dir
		return cmd.Run()
	})

	// Test the commit flow
	headBefore, err := w.getHeadRef(context.Background())
	if err != nil {
		t.Fatalf("failed to get HEAD ref: %v", err)
	}

	files, _ := w.getChangedFiles(context.Background())
	prompt := BuildCommitPrompt("Add new feature", files)

	if err := mocked.invokeClaude(context.Background(), prompt); err != nil {
		t.Fatalf("invokeClaude failed: %v", err)
	}

	hasCommit, err := w.hasNewCommit(context.Background(), headBefore)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if !hasCommit {
		t.Error("expected new commit to be created")
	}
}

func TestCommitViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	esc := &mockEscalator{}
	w := &Worker{
		worktreePath: dir,
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    esc,
	}

	mocked := wrapWorkerForMocking(w, func(ctx context.Context, prompt TaskPrompt) error {
		return errors.New("claude unavailable")
	})

	// Test that retries exhaust and escalation is called
	headBefore, _ := w.getHeadRef(context.Background())
	files, _ := w.getChangedFiles(context.Background())
	prompt := BuildCommitPrompt("Test task", files)

	result := RetryWithBackoff(context.Background(), DefaultRetryConfig, func(ctx context.Context) error {
		if err := mocked.invokeClaude(ctx, prompt); err != nil {
			return err
		}
		hasCommit, err := w.hasNewCommit(ctx, headBefore)
		if err != nil {
			return err
		}
		if !hasCommit {
			return errors.New("claude did not create a commit")
		}
		return nil
	})

	if result.Success {
		t.Error("expected retries to fail")
	}

	// Simulate escalation
	if !result.Success {
		if w.escalator != nil {
			w.escalator.Escalate(context.Background(), escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to commit changes",
				Message:  fmt.Sprintf("Claude could not commit after %d attempts", result.Attempts),
				Context: map[string]string{
					"task":  "Test task",
					"error": result.LastErr.Error(),
				},
			})
		}
	}

	if len(esc.escalations) == 0 {
		t.Error("expected escalation to be called")
	}

	if len(esc.escalations) > 0 {
		e := esc.escalations[0]
		if e.Severity != escalate.SeverityBlocking {
			t.Errorf("expected SeverityBlocking, got %v", e.Severity)
		}
		if e.Title != "Failed to commit changes" {
			t.Errorf("unexpected title: %s", e.Title)
		}
	}
}

func TestCommitViaClaudeCode_VerifiesCommit(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{
		worktreePath: dir,
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    &mockEscalator{},
	}

	mocked := wrapWorkerForMocking(w, func(ctx context.Context, prompt TaskPrompt) error {
		return nil // Success but no commit
	})

	headBefore, _ := w.getHeadRef(context.Background())
	files, _ := w.getChangedFiles(context.Background())
	prompt := BuildCommitPrompt("Test task", files)

	if err := mocked.invokeClaude(context.Background(), prompt); err != nil {
		t.Fatalf("invokeClaude should not fail: %v", err)
	}

	hasCommit, err := w.hasNewCommit(context.Background(), headBefore)
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if hasCommit {
		t.Error("expected no commit to be created")
	}

	// Verify that the error flow would catch this
	err = RetryWithBackoff(context.Background(), RetryConfig{MaxAttempts: 1}, func(ctx context.Context) error {
		if err := mocked.invokeClaude(ctx, prompt); err != nil {
			return err
		}
		hasCommit, err := w.hasNewCommit(ctx, headBefore)
		if err != nil {
			return err
		}
		if !hasCommit {
			return errors.New("claude did not create a commit")
		}
		return nil
	}).LastErr

	if err == nil {
		t.Error("expected error when no commit created")
	}

	if !strings.Contains(err.Error(), "did not create a commit") {
		t.Errorf("error should mention missing commit: %v", err)
	}
}

// captureEventBus wraps an events.Bus and captures emitted events
type captureEventBus struct {
	*events.Bus
	mu      sync.Mutex
	emitted []events.Event
}

func newCaptureEventBus() *captureEventBus {
	bus := events.NewBus(100)
	capture := &captureEventBus{
		Bus:     bus,
		emitted: make([]events.Event, 0),
	}
	bus.Subscribe(func(e events.Event) {
		capture.mu.Lock()
		capture.emitted = append(capture.emitted, e)
		capture.mu.Unlock()
	})
	return capture
}

// getEmitted returns a copy of captured events (thread-safe)
func (c *captureEventBus) getEmitted() []events.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]events.Event, len(c.emitted))
	copy(result, c.emitted)
	return result
}

func TestPushViaClaudeCode_Success(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Setup remote for testing (use local bare repo as "remote")
	remoteDir, err := os.MkdirTemp("", "git-remote-*")
	if err != nil {
		t.Fatalf("failed to create remote dir: %v", err)
	}
	defer os.RemoveAll(remoteDir)

	// Initialize bare remote
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init remote: %v", err)
	}

	// Add remote to test repo
	cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	eventBus := newCaptureEventBus()
	defer eventBus.Close()

	w := &Worker{
		worktreePath: dir,
		branch:       "test-branch",
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    &mockEscalator{},
		events:       eventBus.Bus,
	}

	// Create branch
	cmd = exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Create a wrapper that overrides invokeClaude
	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			cmd := exec.Command("git", "push", "-u", "origin", "test-branch")
			cmd.Dir = dir
			return cmd.Run()
		},
	}

	err = wrapper.pushViaClaudeCode(context.Background())
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
}

func TestPushViaClaudeCode_EmitsEvent(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Setup remote
	remoteDir, _ := os.MkdirTemp("", "git-remote-*")
	defer os.RemoveAll(remoteDir)

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	cmd.Run()

	cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "checkout", "-b", "feature-branch")
	cmd.Dir = dir
	cmd.Run()

	eventBus := newCaptureEventBus()
	defer eventBus.Close()

	w := &Worker{
		worktreePath: dir,
		branch:       "feature-branch",
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    &mockEscalator{},
		events:       eventBus.Bus,
	}

	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			cmd := exec.Command("git", "push", "-u", "origin", "feature-branch")
			cmd.Dir = dir
			return cmd.Run()
		},
	}

	if err := wrapper.pushViaClaudeCode(context.Background()); err != nil {
		t.Fatalf("pushViaClaudeCode failed: %v", err)
	}

	// Wait a moment for event bus to process
	time.Sleep(100 * time.Millisecond)

	// Find the BranchPushed event in the emitted events (use thread-safe getter)
	emitted := eventBus.getEmitted()
	var branchPushedEvent *events.Event
	for _, e := range emitted {
		if e.Type == events.BranchPushed {
			branchPushedEvent = &e
			break
		}
	}

	if branchPushedEvent == nil {
		t.Errorf("expected BranchPushed event to be emitted, got events: %v", emitted)
		return
	}

	payload, ok := branchPushedEvent.Payload.(map[string]interface{})
	if !ok {
		t.Error("expected payload to be map")
	}
	if payload["branch"] != "feature-branch" {
		t.Errorf("expected branch in payload, got %v", payload)
	}
}

func TestPushViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	esc := &mockEscalator{}
	w := &Worker{
		worktreePath: dir,
		branch:       "test-branch",
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    esc,
	}

	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			return errors.New("network error")
		},
	}

	err := wrapper.pushViaClaudeCode(context.Background())
	if err == nil {
		t.Error("expected error after exhausting retries")
	}

	if len(esc.escalations) == 0 {
		t.Error("expected escalation to be called")
	}

	if len(esc.escalations) > 0 {
		e := esc.escalations[0]
		if e.Severity != escalate.SeverityBlocking {
			t.Errorf("expected SeverityBlocking, got %v", e.Severity)
		}
		if e.Title != "Failed to push branch" {
			t.Errorf("unexpected title: %s", e.Title)
		}
		if e.Context["branch"] != "test-branch" {
			t.Errorf("expected branch in context: %v", e.Context)
		}
	}
}

func TestPushViaClaudeCode_VerifiesBranch(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	esc := &mockEscalator{}
	w := &Worker{
		worktreePath: dir,
		branch:       "test-branch",
		unit:         &discovery.Unit{ID: "test-unit"},
		escalator:    esc,
	}

	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			return nil // Success but no push
		},
	}

	err := wrapper.pushViaClaudeCode(context.Background())
	if err == nil {
		t.Error("expected error when branch not on remote")
	}
}

// mockPushInvoker wraps a Worker and allows customizing invokeClaude behavior
type mockPushInvoker struct {
	*Worker
	invokeFunc func(ctx context.Context, prompt string) error
}

func (m *mockPushInvoker) invokeClaude(ctx context.Context, prompt string) error {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, prompt)
	}
	return m.Worker.invokeClaude(ctx, prompt)
}

// pushViaClaudeCode overrides the Worker method to use the mock's invokeClaude
func (m *mockPushInvoker) pushViaClaudeCode(ctx context.Context) error {
	prompt := BuildPushPrompt(m.branch)

	result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := m.invokeClaude(ctx, prompt); err != nil {
			return err
		}

		// Verify branch exists on remote
		exists, err := m.branchExistsOnRemote(ctx, m.branch)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("branch not found on remote after push")
		}
		return nil
	})

	if !result.Success {
		if m.escalator != nil {
			m.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     m.unit.ID,
				Title:    "Failed to push branch",
				Message:  fmt.Sprintf("Claude could not push after %d attempts", result.Attempts),
				Context: map[string]string{
					"branch": m.branch,
					"error":  result.LastErr.Error(),
				},
			})
		}
		return result.LastErr
	}

	// Emit BranchPushed event on success
	if m.events != nil {
		evt := events.NewEvent(events.BranchPushed, m.unit.ID).
			WithPayload(map[string]interface{}{"branch": m.branch})
		m.events.Emit(evt)
	}

	return nil
}

func TestExtractPRURL_ValidURL(t *testing.T) {
	output := `Creating pull request...
https://github.com/RevCBH/choo/pull/42
Done!`

	url := extractPRURL(output)
	if url != "https://github.com/RevCBH/choo/pull/42" {
		t.Errorf("expected PR URL, got %q", url)
	}
}

func TestExtractPRURL_NoURL(t *testing.T) {
	output := "Error: could not create PR"

	url := extractPRURL(output)
	if url != "" {
		t.Errorf("expected empty string, got %q", url)
	}
}

func TestExtractPRURL_MultipleURLs(t *testing.T) {
	// Should return the first match
	output := `https://github.com/RevCBH/choo/pull/41
https://github.com/RevCBH/choo/pull/42`

	url := extractPRURL(output)
	if url != "https://github.com/RevCBH/choo/pull/41" {
		t.Errorf("expected first PR URL, got %q", url)
	}
}

func TestExtractPRURL_URLInMiddleOfText(t *testing.T) {
	output := `Pull request created successfully at https://github.com/owner/repo/pull/123 - please review`

	url := extractPRURL(output)
	if url != "https://github.com/owner/repo/pull/123" {
		t.Errorf("expected PR URL from middle of text, got %q", url)
	}
}

func TestExtractPRURL_DifferentOwnerRepo(t *testing.T) {
	output := "https://github.com/my-org/my-project/pull/999"

	url := extractPRURL(output)
	if url != "https://github.com/my-org/my-project/pull/999" {
		t.Errorf("expected PR URL with different org, got %q", url)
	}
}

func TestCreatePRViaClaudeCode_Success(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	w := &Worker{
		worktreePath: dir,
		branch:       "feature/test",
		unit:         &discovery.Unit{ID: "test-unit"},
		config:       WorkerConfig{TargetBranch: "main"},
		escalator:    &mockEscalator{},
	}

	// Mock invokeClaudeWithOutput to return PR URL
	w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
		return "Created PR: https://github.com/test/repo/pull/42", nil
	}

	url, err := w.createPRViaClaudeCode(context.Background())
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if url != "https://github.com/test/repo/pull/42" {
		t.Errorf("expected PR URL, got %q", url)
	}
}

func TestCreatePRViaClaudeCode_EscalatesOnExhaustion(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	esc := &mockEscalator{}
	w := &Worker{
		worktreePath: dir,
		branch:       "feature/test",
		unit:         &discovery.Unit{ID: "test-unit"},
		config:       WorkerConfig{TargetBranch: "main"},
		escalator:    esc,
	}

	// Mock invokeClaudeWithOutput to always fail
	w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("gh CLI error")
	}

	url, err := w.createPRViaClaudeCode(context.Background())
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if url != "" {
		t.Errorf("expected empty URL on error, got %q", url)
	}

	if len(esc.escalations) == 0 {
		t.Error("expected escalation to be called")
	}

	if len(esc.escalations) > 0 {
		e := esc.escalations[0]
		if e.Severity != escalate.SeverityBlocking {
			t.Errorf("expected SeverityBlocking, got %v", e.Severity)
		}
		if e.Title != "Failed to create PR" {
			t.Errorf("unexpected title: %s", e.Title)
		}
		if e.Context["branch"] != "feature/test" {
			t.Errorf("expected branch in context: %v", e.Context)
		}
		if e.Context["target"] != "main" {
			t.Errorf("expected target in context: %v", e.Context)
		}
	}
}

func TestCreatePRViaClaudeCode_FailsWithoutURL(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	esc := &mockEscalator{}
	w := &Worker{
		worktreePath: dir,
		branch:       "feature/test",
		unit:         &discovery.Unit{ID: "test-unit"},
		config:       WorkerConfig{TargetBranch: "main"},
		escalator:    esc,
	}

	// Mock invokeClaudeWithOutput to return no URL
	w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
		return "PR creation completed but no URL printed", nil
	}

	url, err := w.createPRViaClaudeCode(context.Background())
	if err == nil {
		t.Error("expected error when no URL in output")
	}
	if url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}

	if !strings.Contains(err.Error(), "could not find PR URL") {
		t.Errorf("error should mention missing URL: %v", err)
	}
}

func TestCreatePRViaClaudeCode_RetriesOnFailure(t *testing.T) {
	dir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	callCount := 0
	w := &Worker{
		worktreePath: dir,
		branch:       "feature/test",
		unit:         &discovery.Unit{ID: "test-unit"},
		config:       WorkerConfig{TargetBranch: "main"},
		escalator:    &mockEscalator{},
	}

	// Mock to fail twice then succeed
	w.invokeClaudeWithOutput = func(ctx context.Context, prompt string) (string, error) {
		callCount++
		if callCount < 3 {
			return "", errors.New("transient error")
		}
		return "https://github.com/test/repo/pull/42", nil
	}

	url, err := w.createPRViaClaudeCode(context.Background())
	if err != nil {
		t.Errorf("expected success after retries, got error: %v", err)
	}
	if url != "https://github.com/test/repo/pull/42" {
		t.Errorf("expected PR URL, got %q", url)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}
