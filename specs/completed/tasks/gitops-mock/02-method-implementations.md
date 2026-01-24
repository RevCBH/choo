---
task: 2
status: complete
backpressure: "go test ./internal/git/... -run TestMockGitOps_Methods -v"
depends_on: [1]
---

# Method Implementations

**Parent spec**: `/specs/GITOPS-MOCK.md`
**Task**: #2 of 4 in implementation plan

## Objective

Implement all GitOps interface methods with call recording and stub returns.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides GitOps interface, all method signatures

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: MockGitOps struct, record helper)

### Package Dependencies
- Standard library (`context`)

## Deliverables

### Files to Modify

```
internal/git/
├── mock_gitops.go      # MODIFY: Add interface method implementations
└── mock_gitops_test.go # CREATE: Tests for method implementations
```

### Functions to Implement

```go
// Read operations

func (m *MockGitOps) Status(ctx context.Context) (StatusResult, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Status"})
    return m.StatusResult, m.StatusErr
}

func (m *MockGitOps) RevParse(ctx context.Context, ref string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "RevParse", Args: []any{ref}})
    return m.RevParseResult, m.RevParseErr
}

func (m *MockGitOps) Diff(ctx context.Context, base, head string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Diff", Args: []any{base, head}})
    return m.DiffResult, m.DiffErr
}

func (m *MockGitOps) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Log", Args: []any{opts}})
    return m.LogResult, m.LogErr
}

// Branch operations

func (m *MockGitOps) CurrentBranch(ctx context.Context) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "CurrentBranch"})
    return m.CurrentBranchResult, m.CurrentBranchErr
}

func (m *MockGitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "CheckoutBranch", Args: []any{branch, create}})
    return m.CheckoutBranchErr
}

func (m *MockGitOps) BranchExists(ctx context.Context, branch string) (bool, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "BranchExists", Args: []any{branch}})
    return m.BranchExistsResult, m.BranchExistsErr
}

// Staging operations

func (m *MockGitOps) Add(ctx context.Context, paths ...string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Add", Args: []any{paths}})
    return m.AddErr
}

func (m *MockGitOps) AddAll(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "AddAll"})
    return m.AddAllErr
}

func (m *MockGitOps) Reset(ctx context.Context, paths ...string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Reset", Args: []any{paths}})
    return m.ResetErr
}

// Commit operations

func (m *MockGitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Commit", Args: []any{msg, opts}})
    return m.CommitErr
}

// Working tree operations

func (m *MockGitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "CheckoutFiles", Args: []any{paths}})
    return m.CheckoutFilesErr
}

func (m *MockGitOps) Clean(ctx context.Context, opts CleanOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Clean", Args: []any{opts}})
    return m.CleanErr
}

func (m *MockGitOps) ResetHard(ctx context.Context, ref string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "ResetHard", Args: []any{ref}})
    return m.ResetHardErr
}

// Remote operations

func (m *MockGitOps) Fetch(ctx context.Context, remote, ref string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Fetch", Args: []any{remote, ref}})
    return m.FetchErr
}

func (m *MockGitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Push", Args: []any{remote, branch, opts}})
    return m.PushErr
}

// Merge operations

func (m *MockGitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "Merge", Args: []any{branch, opts}})
    return m.MergeErr
}

func (m *MockGitOps) MergeAbort(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.record(MockCall{Method: "MergeAbort"})
    return m.MergeAbortErr
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestMockGitOps_Methods -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestMockGitOps_MethodsStatus` | Returns stubbed StatusResult |
| `TestMockGitOps_MethodsStatusError` | Returns stubbed error |
| `TestMockGitOps_MethodsAdd` | Records paths in call args |
| `TestMockGitOps_MethodsCommit` | Records message and opts in call args |
| `TestMockGitOps_MethodsRecordsCalls` | All calls appear in Calls slice |
| `TestMockGitOps_MethodsImplementsInterface` | MockGitOps satisfies GitOps interface |

### Test Implementation

```go
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
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All methods acquire lock before recording and returning
- Variadic arguments (paths) are stored as a single []string in Args
- Methods return stub values directly without any logic (safety simulation in Task #3)

## NOT In Scope

- Safety simulation (AllowDestructive, BranchGuard) — Task #3
- Assertion helpers — Task #4
