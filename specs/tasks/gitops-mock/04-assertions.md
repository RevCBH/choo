---
task: 4
status: complete
backpressure: "go test ./internal/git/... -run TestMockGitOps_Assert -v"
depends_on: [2]
---

# Assertion Helpers

**Parent spec**: `/specs/GITOPS-MOCK.md`
**Task**: #4 of 4 in implementation plan

## Objective

Implement assertion helpers for common test patterns: AssertCalled, AssertNotCalled, AssertCallCount, AssertCalledWith, AssertCallOrder, and safety-related assertions.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides error types

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: Calls slice, MockCall type)

### Package Dependencies
- Standard library (`errors`, `reflect`, `testing`)

## Deliverables

### Files to Modify

```
internal/git/
├── mock_gitops.go      # MODIFY: Add assertion methods
└── mock_gitops_test.go # MODIFY: Add assertion tests
```

### Functions to Implement

```go
// AssertCalled verifies that a method was called at least once.
func (m *MockGitOps) AssertCalled(t testing.TB, method string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, call := range m.Calls {
        if call.Method == method {
            return
        }
    }
    t.Errorf("expected %s to be called, but it wasn't", method)
}

// AssertNotCalled verifies that a method was never called.
func (m *MockGitOps) AssertNotCalled(t testing.TB, method string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, call := range m.Calls {
        if call.Method == method {
            t.Errorf("expected %s not to be called, but it was", method)
            return
        }
    }
}

// AssertCallCount verifies the exact number of times a method was called.
func (m *MockGitOps) AssertCallCount(t testing.TB, method string, count int) {
    t.Helper()
    actual := m.CallCount(method)
    if actual != count {
        t.Errorf("expected %s to be called %d times, got %d", method, count, actual)
    }
}

// CallCount returns the number of times a method was called.
func (m *MockGitOps) CallCount(method string) int {
    m.mu.RLock()
    defer m.mu.RUnlock()

    count := 0
    for _, call := range m.Calls {
        if call.Method == method {
            count++
        }
    }
    return count
}

// AssertCalledWith verifies a method was called with specific arguments.
func (m *MockGitOps) AssertCalledWith(t testing.TB, method string, args ...any) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, call := range m.Calls {
        if call.Method == method && reflect.DeepEqual(call.Args, args) {
            return
        }
    }
    t.Errorf("expected %s to be called with %v, but it wasn't", method, args)
}

// AssertCallOrder verifies methods were called in a specific sequence.
func (m *MockGitOps) AssertCallOrder(t testing.TB, methods ...string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    if len(methods) == 0 {
        return
    }

    methodIdx := 0
    for _, call := range m.Calls {
        if call.Method == methods[methodIdx] {
            methodIdx++
            if methodIdx == len(methods) {
                return // All methods found in order
            }
        }
    }

    t.Errorf("expected call order %v, but only found %d of %d in order", methods, methodIdx, len(methods))
}

// GetCalls returns all recorded calls for inspection.
func (m *MockGitOps) GetCalls() []MockCall {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return append([]MockCall{}, m.Calls...)
}

// GetCallsFor returns all calls to a specific method.
func (m *MockGitOps) GetCallsFor(method string) []MockCall {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var calls []MockCall
    for _, call := range m.Calls {
        if call.Method == method {
            calls = append(calls, call)
        }
    }
    return calls
}

// Safety-related assertion helpers

// AssertDestructiveBlocked verifies a destructive operation was rejected.
func (m *MockGitOps) AssertDestructiveBlocked(t testing.TB, method string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, call := range m.Calls {
        if call.Method == method && call.BlockedBy == "AllowDestructive" {
            if errors.Is(call.Error, ErrDestructiveNotAllowed) {
                return
            }
        }
    }
    t.Errorf("expected %s to be blocked by AllowDestructive check, but it wasn't", method)
}

// AssertBranchGuardTriggered verifies a branch guard check rejected an operation.
func (m *MockGitOps) AssertBranchGuardTriggered(t testing.TB, method string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, call := range m.Calls {
        if call.Method == method && call.BlockedBy == "BranchGuard" {
            return
        }
    }
    t.Errorf("expected %s to trigger BranchGuard, but it didn't", method)
}

// AssertAuditLogged verifies an audit entry was created for the given operation.
func (m *MockGitOps) AssertAuditLogged(t testing.TB, operation string) {
    t.Helper()
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, entry := range m.AuditEntries {
        if entry.Operation == operation {
            return
        }
    }
    t.Errorf("expected audit entry for %s, but none found", operation)
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestMockGitOps_Assert -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestMockGitOps_AssertCalled` | Passes when method was called |
| `TestMockGitOps_AssertCalled_Fails` | Fails when method wasn't called |
| `TestMockGitOps_AssertNotCalled` | Passes when method wasn't called |
| `TestMockGitOps_AssertCallCount` | Verifies exact call count |
| `TestMockGitOps_AssertCalledWith` | Matches specific arguments |
| `TestMockGitOps_AssertCallOrder` | Verifies call sequence |
| `TestMockGitOps_GetCallsFor` | Returns only matching calls |
| `TestMockGitOps_AssertDestructiveBlocked` | Finds blocked destructive call |
| `TestMockGitOps_AssertBranchGuardTriggered` | Finds branch guard rejection |

### Test Implementation

```go
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
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All assertion methods use `t.Helper()` for proper error line reporting
- Read operations use RLock for concurrent test access
- GetCalls/GetCallsFor return copies to prevent test interference
- AssertCallOrder allows interleaved calls (just checks relative order)

## NOT In Scope

- Argument matchers (Any, Contains, StartsWith) — future enhancement
- Sequence-dependent responses — future enhancement
