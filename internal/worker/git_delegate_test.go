package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/events"
)

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

func (m *mockPushInvoker) pushViaClaudeCode(ctx context.Context) error {
	prompt := BuildPushPrompt(m.branch)

	result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := m.invokeClaude(ctx, prompt); err != nil {
			return err
		}

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
		return result.LastErr
	}

	if m.events != nil {
		evt := events.NewEvent(events.BranchPushed, m.unit.ID).
			WithPayload(map[string]any{"branch": m.branch})
		m.events.Emit(evt)
	}

	return nil
}

func useFastRetry(t *testing.T) {
	t.Helper()
	prev := DefaultRetryConfig
	DefaultRetryConfig = RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1,
	}
	t.Cleanup(func() {
		DefaultRetryConfig = prev
	})
}

func TestGetHeadRef_ReturnsCommitSHA(t *testing.T) {
	runner := newFakeGitRunner()
	runner.stub("rev-parse HEAD", "abc123\n", nil)

	w := &Worker{worktreePath: "/tmp/worktree", gitRunner: runner}
	ref, err := w.getHeadRef(context.Background())
	if err != nil {
		t.Fatalf("getHeadRef failed: %v", err)
	}
	if ref != "abc123" {
		t.Errorf("expected abc123, got %q", ref)
	}
}

func TestHasNewCommit_DetectsChange(t *testing.T) {
	runner := newFakeGitRunner()
	runner.stub("rev-parse HEAD", "newsha", nil)

	w := &Worker{worktreePath: "/tmp/worktree", gitRunner: runner}
	changed, err := w.hasNewCommit(context.Background(), "oldsha")
	if err != nil {
		t.Fatalf("hasNewCommit failed: %v", err)
	}
	if !changed {
		t.Error("expected change to be detected")
	}
}

func TestBranchExistsOnRemote(t *testing.T) {
	runner := newFakeGitRunner()
	runner.stub("ls-remote --heads origin feature", "hash\trefs/heads/feature\n", nil)

	w := &Worker{worktreePath: "/tmp/worktree", gitRunner: runner}
	exists, err := w.branchExistsOnRemote(context.Background(), "feature")
	if err != nil {
		t.Fatalf("branchExistsOnRemote failed: %v", err)
	}
	if !exists {
		t.Error("expected branch to exist on remote")
	}
}

func TestGetChangedFiles_ParsesPorcelain(t *testing.T) {
	runner := newFakeGitRunner()
	runner.stub("status --porcelain", " M file1.txt\nA  file2.txt\n", nil)

	w := &Worker{worktreePath: "/tmp/worktree", gitRunner: runner}
	files, err := w.getChangedFiles(context.Background())
	if err != nil {
		t.Fatalf("getChangedFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestPushViaClaudeCode_EmitsEvent(t *testing.T) {
	useFastRetry(t)

	runner := newFakeGitRunner()
	runner.stub("ls-remote --heads origin feature", "hash\trefs/heads/feature\n", nil)

	bus := events.NewBus(10)
	defer bus.Close()

	eventCh := make(chan events.Event, 1)
	bus.Subscribe(func(e events.Event) {
		eventCh <- e
	})

	w := &Worker{
		worktreePath: "/tmp/worktree",
		branch:       "feature",
		unit:         &discovery.Unit{ID: "unit-1"},
		events:       bus,
		gitRunner:    runner,
	}

	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			return nil
		},
	}

	if err := wrapper.pushViaClaudeCode(context.Background()); err != nil {
		t.Fatalf("pushViaClaudeCode failed: %v", err)
	}

	select {
	case evt := <-eventCh:
		if evt.Type != events.BranchPushed {
			t.Errorf("expected BranchPushed, got %s", evt.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected BranchPushed event")
	}
}

func TestPushViaClaudeCode_FailsWhenBranchMissing(t *testing.T) {
	useFastRetry(t)

	runner := newFakeGitRunner()
	runner.stub("ls-remote --heads origin missing", "", nil)

	w := &Worker{
		worktreePath: "/tmp/worktree",
		branch:       "missing",
		unit:         &discovery.Unit{ID: "unit-1"},
		gitRunner:    runner,
	}

	wrapper := &mockPushInvoker{
		Worker: w,
		invokeFunc: func(ctx context.Context, prompt string) error {
			return nil
		},
	}

	if err := wrapper.pushViaClaudeCode(context.Background()); err == nil {
		t.Fatal("expected error when branch missing on remote")
	}
}
