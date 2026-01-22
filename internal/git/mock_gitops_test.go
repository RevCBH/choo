// internal/git/mock_gitops_test.go
package git

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// testAuditLogger implements AuditLogger for tests
type testAuditLogger struct {
	entries []AuditEntry
}

func (l *testAuditLogger) Log(entry AuditEntry) {
	l.entries = append(l.entries, entry)
}

func TestMockGitOps_MethodsStatus(t *testing.T) {
	mock := NewMockGitOps("/repo")
	mock.StatusResult = StatusResult{
		Clean:    false,
		Modified: []string{"file1.go", "file2.go"},
	}

	result, err := mock.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Clean {
		t.Error("expected Clean=false")
	}
	if len(result.Modified) != 2 {
		t.Errorf("expected 2 modified files, got %d", len(result.Modified))
	}
}

func TestMockGitOps_MethodsStatusError(t *testing.T) {
	mock := NewMockGitOps("/repo")
	mock.StatusErr = errors.New("git not found")

	_, err := mock.Status(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "git not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMockGitOps_MethodsAdd(t *testing.T) {
	mock := NewMockGitOps("/repo")

	err := mock.Add(context.Background(), "file1.go", "file2.go")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	if mock.Calls[0].Method != "Add" {
		t.Errorf("expected method Add, got %s", mock.Calls[0].Method)
	}
	paths := mock.Calls[0].Args[0].([]string)
	if len(paths) != 2 || paths[0] != "file1.go" {
		t.Errorf("unexpected args: %v", mock.Calls[0].Args)
	}
}

func TestMockGitOps_MethodsCommit(t *testing.T) {
	mock := NewMockGitOps("/repo")
	opts := CommitOpts{NoVerify: true, Author: "Test <test@example.com>"}

	err := mock.Commit(context.Background(), "feat: add feature", opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	call := mock.Calls[0]
	if call.Method != "Commit" {
		t.Errorf("expected method Commit, got %s", call.Method)
	}
	if call.Args[0] != "feat: add feature" {
		t.Errorf("expected message 'feat: add feature', got %v", call.Args[0])
	}
}

func TestMockGitOps_MethodsRecordsCalls(t *testing.T) {
	mock := NewMockGitOps("/repo")
	ctx := context.Background()

	mock.Status(ctx)
	mock.Add(ctx, "file.go")
	mock.Commit(ctx, "msg", CommitOpts{})

	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(mock.Calls))
	}
	if mock.Calls[0].Method != "Status" {
		t.Errorf("expected first call Status, got %s", mock.Calls[0].Method)
	}
	if mock.Calls[1].Method != "Add" {
		t.Errorf("expected second call Add, got %s", mock.Calls[1].Method)
	}
	if mock.Calls[2].Method != "Commit" {
		t.Errorf("expected third call Commit, got %s", mock.Calls[2].Method)
	}
}

func TestMockGitOps_MethodsImplementsInterface(t *testing.T) {
	// Compile-time check that MockGitOps implements GitOps
	var _ GitOps = (*MockGitOps)(nil)
}

// Safety simulation tests

func TestMockGitOps_SafetyDestructiveBlocked(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: false,
	})

	err := mock.ResetHard(context.Background(), "HEAD")

	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].BlockedBy != "AllowDestructive" {
		t.Error("expected call to be blocked by AllowDestructive")
	}
}

func TestMockGitOps_SafetyCleanBlocked(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: false,
	})

	err := mock.Clean(context.Background(), CleanOpts{Force: true})

	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].BlockedBy != "AllowDestructive" {
		t.Error("expected call to be blocked by AllowDestructive")
	}
}

func TestMockGitOps_SafetyCheckoutFilesBlocked(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: false,
	})

	err := mock.CheckoutFiles(context.Background(), "file.go")

	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].BlockedBy != "AllowDestructive" {
		t.Error("expected call to be blocked by AllowDestructive")
	}
}

func TestMockGitOps_SafetyForcePushBlocked(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: false,
	})

	err := mock.Push(context.Background(), "origin", "main", PushOpts{Force: true})

	if !errors.Is(err, ErrDestructiveNotAllowed) {
		t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].BlockedBy != "AllowDestructive" {
		t.Error("expected call to be blocked by AllowDestructive")
	}
}

func TestMockGitOps_SafetyDestructiveAllowed(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: true,
	})

	err := mock.ResetHard(context.Background(), "HEAD")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = mock.Clean(context.Background(), CleanOpts{Force: true})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = mock.CheckoutFiles(context.Background(), "file.go")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(mock.Calls))
	}
	for _, call := range mock.Calls {
		if call.BlockedBy != "" {
			t.Errorf("expected call %s not to be blocked, but was blocked by %s", call.Method, call.BlockedBy)
		}
	}
}

func TestMockGitOps_SafetyBranchGuardProtected(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		BranchGuard: &BranchGuard{
			ProtectedBranches: []string{"main", "master"},
		},
		AllowDestructive: true,
	})
	mock.CurrentBranchResult = "main"

	err := mock.Commit(context.Background(), "msg", CommitOpts{})

	if !errors.Is(err, ErrProtectedBranch) {
		t.Errorf("expected ErrProtectedBranch, got %v", err)
	}
}

func TestMockGitOps_SafetyBranchGuardAllowed(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		BranchGuard: &BranchGuard{
			ProtectedBranches: []string{"main"},
		},
		AllowDestructive: true,
	})
	mock.CurrentBranchResult = "feature/test"

	err := mock.Commit(context.Background(), "msg", CommitOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMockGitOps_SafetyAuditLogged(t *testing.T) {
	logger := &testAuditLogger{}
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AuditLogger:      logger,
		AllowDestructive: true,
	})

	mock.Commit(context.Background(), "msg", CommitOpts{})

	entries := mock.GetAuditEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Operation != "Commit" {
		t.Errorf("expected operation Commit, got %s", entries[0].Operation)
	}
}

// Assertion helper tests

func TestMockGitOps_AssertCalled(t *testing.T) {
	mock := NewMockGitOps("/repo")
	mock.Status(context.Background())

	// This should pass
	mock.AssertCalled(t, "Status")
}

func TestMockGitOps_AssertNotCalled(t *testing.T) {
	mock := NewMockGitOps("/repo")
	mock.Status(context.Background())

	// This should pass
	mock.AssertNotCalled(t, "Commit")
}

func TestMockGitOps_AssertCallCount(t *testing.T) {
	mock := NewMockGitOps("/repo")
	ctx := context.Background()

	mock.Status(ctx)
	mock.Status(ctx)
	mock.Add(ctx, "file.go")

	mock.AssertCallCount(t, "Status", 2)
	mock.AssertCallCount(t, "Add", 1)
	mock.AssertCallCount(t, "Commit", 0)
}

func TestMockGitOps_AssertCalledWith(t *testing.T) {
	mock := NewMockGitOps("/repo")
	mock.Commit(context.Background(), "test message", CommitOpts{NoVerify: true})

	mock.AssertCalledWith(t, "Commit", "test message", CommitOpts{NoVerify: true})
}

func TestMockGitOps_AssertCallOrder(t *testing.T) {
	mock := NewMockGitOps("/repo")
	ctx := context.Background()

	mock.Status(ctx)
	mock.Add(ctx, "file.go")
	mock.Commit(ctx, "msg", CommitOpts{})

	// This should pass
	mock.AssertCallOrder(t, "Status", "Add", "Commit")
}

func TestMockGitOps_GetCallsFor(t *testing.T) {
	mock := NewMockGitOps("/repo")
	ctx := context.Background()

	mock.Add(ctx, "file1.go")
	mock.Status(ctx)
	mock.Add(ctx, "file2.go")

	calls := mock.GetCallsFor("Add")

	if len(calls) != 2 {
		t.Errorf("expected 2 Add calls, got %d", len(calls))
	}
	if calls[0].Args[0].([]string)[0] != "file1.go" {
		t.Error("expected first Add to have file1.go")
	}
	if calls[1].Args[0].([]string)[0] != "file2.go" {
		t.Error("expected second Add to have file2.go")
	}
}

func TestMockGitOps_AssertDestructiveBlocked(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		AllowDestructive: false,
	})

	mock.ResetHard(context.Background(), "HEAD")

	mock.AssertDestructiveBlocked(t, "ResetHard")
}

func TestMockGitOps_AssertBranchGuardTriggered(t *testing.T) {
	mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
		BranchGuard:      &BranchGuard{ProtectedBranches: []string{"main"}},
		AllowDestructive: true,
	})
	mock.CurrentBranchResult = "main"

	mock.Commit(context.Background(), "msg", CommitOpts{})

	mock.AssertBranchGuardTriggered(t, "Commit")
}

func TestMockGitOps_ThreadSafety(t *testing.T) {
	mock := NewMockGitOps("/repo")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mock.Status(context.Background())
		}()
	}

	wg.Wait()

	if mock.CallCount("Status") != 100 {
		t.Errorf("expected 100 Status calls, got %d", mock.CallCount("Status"))
	}
}
