// internal/git/mock_gitops_test.go
package git

import (
	"context"
	"errors"
	"testing"
)

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
