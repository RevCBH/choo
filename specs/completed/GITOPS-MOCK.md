# GITOPS-MOCK — Mock Implementation of GitOps Interface for Testing

## Overview

The GITOPS-MOCK package provides a testable mock implementation of the GitOps interface defined in the GITOPS spec. It enables unit and integration tests to verify git operation behavior without executing actual git commands or requiring a real repository.

Current tests must stub raw git commands at the Runner level, which requires understanding low-level git command sequences, doesn't test semantic operation behavior, is brittle when implementation details change, and doesn't catch interface misuse. MockGitOps solves these problems by providing a high-level interface mock with configurable responses and call tracking.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Test Code                                      │
│                                                                          │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                        MockGitOps                                 │  │
│   │                                                                   │  │
│   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │  │
│   │  │   Stubbed    │  │    Call      │  │     Assertion        │   │  │
│   │  │   Results    │  │   Tracking   │  │      Helpers         │   │  │
│   │  └──────────────┘  └──────────────┘  └──────────────────────┘   │  │
│   │                                                                   │  │
│   │        StatusResult ───▶ Calls []GitOpsCall ───▶ AssertCalled()  │  │
│   │        StatusErr                                  AssertNotCalled │  │
│   │        BranchResult                               AssertCallCount │  │
│   │        ...                                        ...             │  │
│   │                                                                   │  │
│   │  ┌──────────────────────────────────────────────────────────┐   │  │
│   │  │                 Safety Simulation                         │   │  │
│   │  │  BranchGuard validation ─▶ ErrProtectedBranch            │   │  │
│   │  │  AllowDestructive check ─▶ ErrDestructiveNotAllowed      │   │  │
│   │  │  AuditLogger capture ───▶ GetAuditEntries()              │   │  │
│   │  └──────────────────────────────────────────────────────────┘   │  │
│   │                                                                   │  │
│   └──────────────────────────────────────────────────────────────────┘  │
│                              │                                           │
│                              │ implements                                │
│                              ▼                                           │
│   ┌──────────────────────────────────────────────────────────────────┐  │
│   │                      GitOps Interface                             │  │
│   │                    (from GITOPS spec)                             │  │
│   └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Implement the complete GitOps interface with configurable stub responses
2. Track all method calls with their arguments for test assertions
3. Support per-method error injection to test error handling paths
4. Provide assertion helpers for common test patterns (e.g., AssertCalled, AssertCallCount)
5. Support call ordering verification for tests that require specific operation sequences
6. Reset call history between test cases without recreating the mock
7. Allow dynamic response changes mid-test for multi-step scenarios
8. Thread-safe for concurrent test execution
9. Simulate safety feature behavior (BranchGuard, AllowDestructive, AuditLogger) when configured
10. Provide assertion helpers for safety-related test scenarios

### Performance Requirements

| Metric | Target |
|--------|--------|
| Method call overhead | <1us per call |
| Memory per recorded call | <1KB |
| Concurrent test safety | Support 100+ parallel tests |

### Constraints

- Depends on GITOPS spec for interface definition
- Must maintain interface compatibility as GitOps evolves
- Call recording must not cause memory leaks in long-running test suites
- Safety simulation must match real GitOps behavior for consistent test results

## Design

### Module Structure

```
internal/git/mock/
├── mock.go          # MockGitOps implementation
├── calls.go         # GitOpsCall type and call tracking
├── assertions.go    # Test assertion helpers
├── safety.go        # Safety simulation logic
└── mock_test.go     # Tests for the mock itself
```

### Core Types

```go
// internal/git/mock/mock.go

// MockGitOps provides a testable implementation of the GitOps interface.
// All methods record their calls and return configurable stub values.
// When configured with GitOpsOpts, simulates safety feature behavior.
type MockGitOps struct {
    path string
    opts GitOpsOpts  // Construction options for safety simulation
    mu   sync.RWMutex

    // Stubbed responses for read operations
    StatusResult        StatusResult
    StatusErr           error
    RevParseResult      string
    RevParseErr         error
    DiffResult          string
    DiffErr             error
    LogResult           []Commit
    LogErr              error

    // Stubbed responses for branch operations
    CurrentBranchResult string
    CurrentBranchErr    error
    CheckoutBranchErr   error
    BranchExistsResult  bool
    BranchExistsErr     error

    // Stubbed responses for staging operations
    AddErr      error
    AddAllErr   error
    ResetErr    error

    // Stubbed responses for commit operations
    CommitErr error

    // Stubbed responses for working tree operations
    CheckoutFilesErr error
    CleanErr         error
    ResetHardErr     error

    // Stubbed responses for remote operations
    FetchErr error
    PushErr  error

    // Stubbed responses for merge operations
    MergeErr      error
    MergeAbortErr error

    // Safety-related tracking
    AuditEntries []AuditEntry // Captured audit entries when AuditLogger configured

    // Safety check results (for simulating failures)
    SimulateBranchGuardErr error // Set to simulate BranchGuard failure

    // Call tracking
    Calls []GitOpsCall
}

// StatusResult represents the state of the working tree.
type StatusResult struct {
    Clean      bool
    Staged     []string
    Modified   []string
    Untracked  []string
    Conflicted []string
}

// Commit represents a single commit in the log.
type Commit struct {
    Hash    string
    Author  string
    Date    time.Time
    Subject string
    Body    string
}

// GitOpsOpts configures safety behavior for the mock.
// Mirrors the real GitOps options to enable consistent testing.
type GitOpsOpts struct {
    // Path constraints
    WorktreeBase  string // Required path prefix
    AllowRepoRoot bool   // Allow operating on repo root

    // Branch/remote constraints
    BranchGuard *BranchGuard // Branch and remote validation rules

    // Destructive operation control
    AllowDestructive bool // Must be true for ResetHard, Clean, CheckoutFiles, force push

    // Safety level (alternative to individual flags)
    SafetyLevel SafetyLevel // Strict, Default, or Relaxed (mock ignores, uses individual flags)

    // Audit logging
    AuditLogger AuditLogger // Optional logger for all operations
}

// SafetyLevel controls the aggressiveness of safety checks.
// In the mock, this is ignored in favor of individual flags for explicit control.
type SafetyLevel int

const (
    SafetyStrict  SafetyLevel = iota // All checks enabled, AllowDestructive=false
    SafetyDefault                     // Path validation + runtime checks, no branch guard
    SafetyRelaxed                     // Path validation only (for tests)
)

// BranchGuard enforces branch/remote constraints.
type BranchGuard struct {
    ExpectedBranch        string   // Exact branch name required
    AllowedBranchPrefixes []string // Allowed branch prefixes
    AllowedRemotes        []string // Allowed remote URLs
    ProtectedBranches     []string // Branches that cannot be written to
}

// AuditEntry records a single operation for audit purposes.
type AuditEntry struct {
    Timestamp     time.Time     `json:"ts"`
    Operation     string        `json:"op"`
    RepoPath      string        `json:"repo_path"`
    Branch        string        `json:"branch,omitempty"`
    Remote        string        `json:"remote,omitempty"`
    Args          []string      `json:"args,omitempty"`
    SafetyChecks  []string      `json:"safety_checks"`
    ChecksPassed  bool          `json:"checks_passed"`
    FailureReason string        `json:"failure_reason,omitempty"`
    Duration      time.Duration `json:"duration_ms"`
    Error         string        `json:"error,omitempty"`
}

// AuditLogger interface for capturing audit entries.
type AuditLogger interface {
    Log(entry AuditEntry)
}
```

```go
// internal/git/mock/calls.go

// GitOpsCall records a single method invocation with its arguments.
type GitOpsCall struct {
    Method string
    Args   []any

    // Safety-related call metadata
    BlockedBy string // Set if call was blocked by safety check (e.g., "BranchGuard", "AllowDestructive")
    Error     error  // Error returned, if any
}

// CommitOpts holds options for the Commit method.
type CommitOpts struct {
    NoVerify   bool   // Skip pre-commit hooks
    Author     string // Override author
    AllowEmpty bool   // Allow creating commits with no changes
}

// CleanOpts holds options for the Clean method.
type CleanOpts struct {
    Force       bool // -f
    Directories bool // -d
    IgnoredOnly bool // -X
    IgnoredToo  bool // -x (remove ignored and untracked files)
}

// PushOpts holds options for the Push method.
type PushOpts struct {
    Force          bool
    SetUpstream    bool
    ForceWithLease bool
}

// MergeOpts holds options for the Merge method.
type MergeOpts struct {
    FFOnly   bool
    NoFF     bool
    Message  string
    NoCommit bool // Performs merge but stops before creating commit
}

// LogOpts holds options for the Log method.
type LogOpts struct {
    MaxCount int
    Since    time.Time
    Until    time.Time
    Path     string // Filters commits affecting this path
}
```

```go
// internal/git/mock/assertions.go

// CallMatcher defines criteria for matching recorded calls.
type CallMatcher struct {
    Method string
    Args   []any // nil means match any args
}

// AssertionResult contains the outcome of an assertion.
type AssertionResult struct {
    Passed  bool
    Message string
    Calls   []GitOpsCall // Relevant calls for debugging
}
```

### Safety Error Types

```go
// internal/git/mock/safety.go

var (
    // ErrDestructiveNotAllowed is returned when a destructive operation
    // is attempted without AllowDestructive=true.
    ErrDestructiveNotAllowed = errors.New("git: destructive operation not allowed")

    // ErrProtectedBranch is returned when attempting to write to a protected branch.
    ErrProtectedBranch = errors.New("git: cannot write to protected branch")

    // ErrUnexpectedBranch is returned when current branch doesn't match guard rules.
    ErrUnexpectedBranch = errors.New("git: unexpected branch")
)
```

### API Surface

```go
// internal/git/mock/mock.go

// NewMockGitOps creates a new mock bound to the given path with default safe options.
// Default options: AllowDestructive=true (for test flexibility), no BranchGuard.
func NewMockGitOps(path string) *MockGitOps

// NewMockGitOpsWithOpts creates a new mock with specific options for safety simulation.
// Use this when testing code that depends on safety feature behavior.
func NewMockGitOpsWithOpts(path string, opts GitOpsOpts) *MockGitOps

// Path returns the repository path this mock is bound to.
func (m *MockGitOps) Path() string

// Opts returns the options this mock was configured with.
func (m *MockGitOps) Opts() GitOpsOpts

// Reset clears all recorded calls, preserving stub configuration.
func (m *MockGitOps) Reset()

// ResetAll clears both recorded calls and stub configuration.
func (m *MockGitOps) ResetAll()

// Read operations
func (m *MockGitOps) Status(ctx context.Context) (StatusResult, error)
func (m *MockGitOps) RevParse(ctx context.Context, ref string) (string, error)
func (m *MockGitOps) Diff(ctx context.Context, base, head string) (string, error)
func (m *MockGitOps) Log(ctx context.Context, opts LogOpts) ([]Commit, error)

// Branch operations
func (m *MockGitOps) CurrentBranch(ctx context.Context) (string, error)
func (m *MockGitOps) CheckoutBranch(ctx context.Context, branch string, create bool) error
func (m *MockGitOps) BranchExists(ctx context.Context, branch string) (bool, error)

// Staging operations
func (m *MockGitOps) Add(ctx context.Context, paths ...string) error
func (m *MockGitOps) AddAll(ctx context.Context) error
func (m *MockGitOps) Reset(ctx context.Context, paths ...string) error

// Commit operations (subject to BranchGuard validation)
func (m *MockGitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error

// Working tree operations (subject to AllowDestructive check)
func (m *MockGitOps) CheckoutFiles(ctx context.Context, paths ...string) error
func (m *MockGitOps) Clean(ctx context.Context, opts CleanOpts) error
func (m *MockGitOps) ResetHard(ctx context.Context, ref string) error

// Remote operations (Push with Force subject to AllowDestructive check)
func (m *MockGitOps) Fetch(ctx context.Context, remote, ref string) error
func (m *MockGitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error

// Merge operations
func (m *MockGitOps) Merge(ctx context.Context, branch string, opts MergeOpts) error
func (m *MockGitOps) MergeAbort(ctx context.Context) error
```

```go
// internal/git/mock/assertions.go

// AssertCalled verifies that a method was called at least once.
func (m *MockGitOps) AssertCalled(t testing.TB, method string)

// AssertNotCalled verifies that a method was never called.
func (m *MockGitOps) AssertNotCalled(t testing.TB, method string)

// AssertCallCount verifies the exact number of times a method was called.
func (m *MockGitOps) AssertCallCount(t testing.TB, method string, count int)

// AssertCalledWith verifies a method was called with specific arguments.
func (m *MockGitOps) AssertCalledWith(t testing.TB, method string, args ...any)

// AssertCallOrder verifies methods were called in a specific sequence.
func (m *MockGitOps) AssertCallOrder(t testing.TB, methods ...string)

// GetCalls returns all recorded calls for inspection.
func (m *MockGitOps) GetCalls() []GitOpsCall

// GetCallsFor returns all calls to a specific method.
func (m *MockGitOps) GetCallsFor(method string) []GitOpsCall

// CallCount returns the number of times a method was called.
func (m *MockGitOps) CallCount(method string) int

// Safety-related assertion helpers

// AssertDestructiveBlocked verifies a destructive operation was rejected
// due to AllowDestructive=false.
func (m *MockGitOps) AssertDestructiveBlocked(t testing.TB, method string)

// AssertBranchGuardTriggered verifies a branch guard check rejected an operation.
func (m *MockGitOps) AssertBranchGuardTriggered(t testing.TB, method string)

// AssertAuditLogged verifies an audit entry was created for the given operation.
func (m *MockGitOps) AssertAuditLogged(t testing.TB, operation string)

// GetAuditEntries returns all captured audit entries.
// Returns a copy to prevent test interference.
func (m *MockGitOps) GetAuditEntries() []AuditEntry
```

## Implementation Notes

### Thread Safety

All methods acquire a lock before recording calls or reading/writing stub values. This ensures tests using `t.Parallel()` don't race on shared mock state:

```go
func (m *MockGitOps) Status(ctx context.Context) (StatusResult, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.Calls = append(m.Calls, GitOpsCall{Method: "Status"})
    return m.StatusResult, m.StatusErr
}
```

### Safety Behavior Simulation

The mock simulates safety checks when configured with GitOpsOpts:

```go
// Destructive operation check
func (m *MockGitOps) ResetHard(ctx context.Context, ref string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Simulate AllowDestructive check
    if !m.opts.AllowDestructive {
        call := GitOpsCall{
            Method:    "ResetHard",
            Args:      []any{ref},
            BlockedBy: "AllowDestructive",
            Error:     ErrDestructiveNotAllowed,
        }
        m.Calls = append(m.Calls, call)
        m.captureAudit("ResetHard", false, "ErrDestructiveNotAllowed")
        return fmt.Errorf("%w: ResetHard", ErrDestructiveNotAllowed)
    }

    // Simulate BranchGuard check if configured
    if err := m.simulateBranchGuard(); err != nil {
        call := GitOpsCall{
            Method:    "ResetHard",
            Args:      []any{ref},
            BlockedBy: "BranchGuard",
            Error:     err,
        }
        m.Calls = append(m.Calls, call)
        m.captureAudit("ResetHard", false, err.Error())
        return err
    }

    m.Calls = append(m.Calls, GitOpsCall{Method: "ResetHard", Args: []any{ref}})
    m.captureAudit("ResetHard", true, "")
    return m.ResetHardErr
}

// BranchGuard simulation for write operations
func (m *MockGitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Allow tests to inject BranchGuard failure
    if m.SimulateBranchGuardErr != nil {
        call := GitOpsCall{
            Method:    "Commit",
            Args:      []any{msg, opts},
            BlockedBy: "BranchGuard",
            Error:     m.SimulateBranchGuardErr,
        }
        m.Calls = append(m.Calls, call)
        m.captureAudit("Commit", false, m.SimulateBranchGuardErr.Error())
        return m.SimulateBranchGuardErr
    }

    // Simulate BranchGuard check if configured
    if err := m.simulateBranchGuard(); err != nil {
        call := GitOpsCall{
            Method:    "Commit",
            Args:      []any{msg, opts},
            BlockedBy: "BranchGuard",
            Error:     err,
        }
        m.Calls = append(m.Calls, call)
        m.captureAudit("Commit", false, err.Error())
        return err
    }

    m.Calls = append(m.Calls, GitOpsCall{Method: "Commit", Args: []any{msg, opts}})
    m.captureAudit("Commit", true, "")
    return m.CommitErr
}

// simulateBranchGuard checks BranchGuard rules against CurrentBranchResult
func (m *MockGitOps) simulateBranchGuard() error {
    if m.opts.BranchGuard == nil {
        return nil
    }

    guard := m.opts.BranchGuard
    branch := m.CurrentBranchResult

    // Check exact match requirement
    if guard.ExpectedBranch != "" && branch != guard.ExpectedBranch {
        return fmt.Errorf("%w: expected=%s, actual=%s", ErrUnexpectedBranch, guard.ExpectedBranch, branch)
    }

    // Check prefix match
    if len(guard.AllowedBranchPrefixes) > 0 {
        allowed := false
        for _, prefix := range guard.AllowedBranchPrefixes {
            if strings.HasPrefix(branch, prefix) {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("%w: branch=%s, allowed=%v", ErrUnexpectedBranch, branch, guard.AllowedBranchPrefixes)
        }
    }

    // Check protected branches
    protected := guard.ProtectedBranches
    if len(protected) == 0 {
        protected = []string{"main", "master"}
    }
    for _, p := range protected {
        if branch == p {
            return fmt.Errorf("%w: %s", ErrProtectedBranch, branch)
        }
    }

    return nil
}
```

### Audit Log Capture

When an AuditLogger is configured, the mock captures audit entries for each operation:

```go
func (m *MockGitOps) captureAudit(operation string, passed bool, failureReason string) {
    if m.opts.AuditLogger == nil {
        return
    }

    entry := AuditEntry{
        Timestamp:     time.Now(),
        Operation:     operation,
        RepoPath:      m.path,
        Branch:        m.CurrentBranchResult,
        SafetyChecks:  m.getSafetyChecks(operation),
        ChecksPassed:  passed,
        FailureReason: failureReason,
    }

    m.AuditEntries = append(m.AuditEntries, entry)
    m.opts.AuditLogger.Log(entry)
}

func (m *MockGitOps) getSafetyChecks(operation string) []string {
    checks := []string{"path_valid"}

    if m.opts.BranchGuard != nil {
        checks = append(checks, "branch_allowed")
    }

    if isDestructiveOp(operation) {
        checks = append(checks, "destructive_allowed")
    }

    return checks
}

func isDestructiveOp(operation string) bool {
    switch operation {
    case "ResetHard", "Clean", "CheckoutFiles":
        return true
    default:
        return false
    }
}

// GetAuditEntries returns a copy of captured audit entries
func (m *MockGitOps) GetAuditEntries() []AuditEntry {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return append([]AuditEntry{}, m.AuditEntries...)
}
```

### Argument Recording

Arguments are stored as `[]any` to support heterogeneous types. For variadic methods like `Add(paths ...string)`, the entire slice is stored as a single argument:

```go
func (m *MockGitOps) Add(ctx context.Context, paths ...string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.Calls = append(m.Calls, GitOpsCall{
        Method: "Add",
        Args:   []any{paths}, // Store paths slice as single arg
    })
    return m.AddErr
}
```

### Context Handling

The mock records calls but does not respect context cancellation. Tests that need to verify context behavior should use a real GitOps implementation or wrap the mock with context-checking logic.

### Default Stub Values

A freshly created MockGitOps returns zero values for all stubs:
- `StatusResult`: `StatusResult{Clean: false, ...}`
- All error fields: `nil`
- `CurrentBranchResult`: `""`
- `BranchExistsResult`: `false`

Tests should explicitly set the stub values they depend on to make test intent clear.

### Default Options

`NewMockGitOps(path)` creates a mock with `AllowDestructive=true` for maximum test flexibility. Use `NewMockGitOpsWithOpts` to test safety-constrained scenarios.

### Option Struct Comparison

When asserting calls with option structs (CommitOpts, CleanOpts, etc.), use `reflect.DeepEqual` for comparison. The assertion helpers handle this automatically:

```go
func (m *MockGitOps) AssertCalledWith(t testing.TB, method string, args ...any) {
    t.Helper()
    for _, call := range m.Calls {
        if call.Method == method && reflect.DeepEqual(call.Args, args) {
            return
        }
    }
    t.Errorf("expected %s to be called with %v, but it wasn't", method, args)
}
```

### Safety Assertion Implementation

```go
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

## Testing Strategy

### Unit Tests

```go
// internal/git/mock/mock_test.go

func TestMockGitOps_Status(t *testing.T) {
    mock := NewMockGitOps("/repo")
    mock.StatusResult = StatusResult{
        Clean:    false,
        Modified: []string{"file1.go", "file2.go"},
        Staged:   []string{"new.go"},
    }

    result, err := mock.Status(context.Background())

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Clean {
        t.Error("expected Clean to be false")
    }
    if len(result.Modified) != 2 {
        t.Errorf("expected 2 modified files, got %d", len(result.Modified))
    }
    mock.AssertCalled(t, "Status")
    mock.AssertCallCount(t, "Status", 1)
}

func TestMockGitOps_StatusError(t *testing.T) {
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

func TestMockGitOps_Add(t *testing.T) {
    mock := NewMockGitOps("/repo")

    err := mock.Add(context.Background(), "file1.go", "file2.go")

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    mock.AssertCalledWith(t, "Add", []string{"file1.go", "file2.go"})
}

func TestMockGitOps_Commit(t *testing.T) {
    mock := NewMockGitOps("/repo")
    opts := CommitOpts{NoVerify: true, Author: "Test <test@example.com>"}

    err := mock.Commit(context.Background(), "feat: add feature", opts)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    mock.AssertCalledWith(t, "Commit", "feat: add feature", opts)
}

func TestMockGitOps_CheckoutBranch(t *testing.T) {
    mock := NewMockGitOps("/repo")

    err := mock.CheckoutBranch(context.Background(), "feature/new", true)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    mock.AssertCalledWith(t, "CheckoutBranch", "feature/new", true)
}

func TestMockGitOps_Reset(t *testing.T) {
    mock := NewMockGitOps("/repo")

    mock.Status(context.Background())
    mock.Add(context.Background(), "file.go")

    if mock.CallCount("Status") != 1 {
        t.Error("expected Status call count to be 1")
    }

    mock.Reset()

    if mock.CallCount("Status") != 0 {
        t.Error("expected Status call count to be 0 after Reset")
    }
    // Stub values should be preserved
    if mock.StatusErr != nil {
        t.Error("expected StatusErr to remain nil after Reset")
    }
}
```

### Safety Feature Tests

```go
// internal/git/mock/safety_test.go

func TestMockGitOps_DestructiveBlocked(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AllowDestructive: false,
    })

    err := mock.ResetHard(context.Background(), "HEAD")

    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
    mock.AssertDestructiveBlocked(t, "ResetHard")
}

func TestMockGitOps_CleanBlockedWithoutAllowDestructive(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AllowDestructive: false,
    })

    err := mock.Clean(context.Background(), CleanOpts{Force: true})

    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
    mock.AssertDestructiveBlocked(t, "Clean")
}

func TestMockGitOps_CheckoutFilesBlockedWithoutAllowDestructive(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AllowDestructive: false,
    })

    err := mock.CheckoutFiles(context.Background(), "file.go")

    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
    mock.AssertDestructiveBlocked(t, "CheckoutFiles")
}

func TestMockGitOps_ForcePushBlockedWithoutAllowDestructive(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AllowDestructive: false,
    })

    err := mock.Push(context.Background(), "origin", "main", PushOpts{Force: true})

    if !errors.Is(err, ErrDestructiveNotAllowed) {
        t.Errorf("expected ErrDestructiveNotAllowed, got %v", err)
    }
    mock.AssertDestructiveBlocked(t, "Push")
}

func TestMockGitOps_DestructiveAllowedWhenEnabled(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AllowDestructive: true,
    })

    err := mock.ResetHard(context.Background(), "HEAD")

    if err != nil {
        t.Errorf("expected no error, got %v", err)
    }
}

func TestMockGitOps_BranchGuardBlocksProtectedBranch(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        BranchGuard: &BranchGuard{
            ProtectedBranches: []string{"main", "master"},
        },
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "main" // Simulate being on main

    err := mock.Commit(context.Background(), "msg", CommitOpts{})

    if !errors.Is(err, ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch, got %v", err)
    }
    mock.AssertBranchGuardTriggered(t, "Commit")
}

func TestMockGitOps_BranchGuardDefaultsProtectMainMaster(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        BranchGuard:      &BranchGuard{}, // Empty guard uses defaults
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "master"

    err := mock.Commit(context.Background(), "msg", CommitOpts{})

    if !errors.Is(err, ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch for master, got %v", err)
    }
}

func TestMockGitOps_BranchGuardAllowsFeatureBranch(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        BranchGuard: &BranchGuard{
            ProtectedBranches: []string{"main"},
        },
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "feature/my-feature"

    err := mock.Commit(context.Background(), "msg", CommitOpts{})

    if err != nil {
        t.Errorf("expected no error for feature branch, got %v", err)
    }
}

func TestMockGitOps_BranchGuardExpectedBranch(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        BranchGuard: &BranchGuard{
            ExpectedBranch: "feature/specific",
        },
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "feature/other"

    err := mock.Commit(context.Background(), "msg", CommitOpts{})

    if !errors.Is(err, ErrUnexpectedBranch) {
        t.Errorf("expected ErrUnexpectedBranch, got %v", err)
    }
}

func TestMockGitOps_BranchGuardAllowedPrefixes(t *testing.T) {
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        BranchGuard: &BranchGuard{
            AllowedBranchPrefixes: []string{"feature/", "fix/"},
        },
        AllowDestructive: true,
    })

    tests := []struct {
        branch    string
        wantError bool
    }{
        {"feature/new", false},
        {"fix/bug", false},
        {"hotfix/urgent", true}, // Not in allowed prefixes
        {"main", true},
    }

    for _, tt := range tests {
        mock.CurrentBranchResult = tt.branch
        mock.Reset()

        err := mock.Commit(context.Background(), "msg", CommitOpts{})

        if tt.wantError && err == nil {
            t.Errorf("branch %s: expected error, got nil", tt.branch)
        }
        if !tt.wantError && err != nil {
            t.Errorf("branch %s: expected no error, got %v", tt.branch, err)
        }
    }
}

func TestMockGitOps_SimulateBranchGuardErr(t *testing.T) {
    mock := NewMockGitOps("/repo")
    mock.SimulateBranchGuardErr = fmt.Errorf("%w: test branch", ErrProtectedBranch)

    err := mock.Commit(context.Background(), "msg", CommitOpts{})

    if !errors.Is(err, ErrProtectedBranch) {
        t.Errorf("expected ErrProtectedBranch, got %v", err)
    }
}
```

### Audit Logging Tests

```go
// internal/git/mock/audit_test.go

type testLogger struct {
    entries []AuditEntry
    mu      sync.Mutex
}

func (l *testLogger) Log(entry AuditEntry) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.entries = append(l.entries, entry)
}

func TestMockGitOps_AuditLogCaptured(t *testing.T) {
    logger := &testLogger{}
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AuditLogger:      logger,
        AllowDestructive: true,
    })

    mock.Status(context.Background())
    mock.Commit(context.Background(), "test commit", CommitOpts{})

    entries := mock.GetAuditEntries()
    if len(entries) != 2 {
        t.Errorf("expected 2 audit entries, got %d", len(entries))
    }

    mock.AssertAuditLogged(t, "Status")
    mock.AssertAuditLogged(t, "Commit")
}

func TestMockGitOps_AuditLogContainsRequiredFields(t *testing.T) {
    logger := &testLogger{}
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AuditLogger:      logger,
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "feature/test"

    mock.Commit(context.Background(), "msg", CommitOpts{})

    entries := mock.GetAuditEntries()
    if len(entries) != 1 {
        t.Fatalf("expected 1 audit entry, got %d", len(entries))
    }

    entry := entries[0]
    if entry.Operation != "Commit" {
        t.Errorf("expected Operation=Commit, got %s", entry.Operation)
    }
    if entry.RepoPath != "/repo" {
        t.Errorf("expected RepoPath=/repo, got %s", entry.RepoPath)
    }
    if entry.Branch != "feature/test" {
        t.Errorf("expected Branch=feature/test, got %s", entry.Branch)
    }
    if !entry.ChecksPassed {
        t.Error("expected ChecksPassed=true")
    }
    if entry.Timestamp.IsZero() {
        t.Error("expected non-zero Timestamp")
    }
}

func TestMockGitOps_AuditLogOnSafetyFailure(t *testing.T) {
    logger := &testLogger{}
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AuditLogger: logger,
        BranchGuard: &BranchGuard{
            ProtectedBranches: []string{"main"},
        },
        AllowDestructive: true,
    })
    mock.CurrentBranchResult = "main"

    mock.Commit(context.Background(), "bad commit", CommitOpts{})

    entries := mock.GetAuditEntries()
    if len(entries) != 1 {
        t.Fatalf("expected 1 audit entry, got %d", len(entries))
    }

    entry := entries[0]
    if entry.ChecksPassed {
        t.Error("expected ChecksPassed=false for blocked operation")
    }
    if entry.FailureReason == "" {
        t.Error("expected FailureReason to be set")
    }
    if !strings.Contains(entry.FailureReason, "ErrProtectedBranch") {
        t.Errorf("expected FailureReason to contain ErrProtectedBranch, got %s", entry.FailureReason)
    }
}

func TestMockGitOps_AuditLogDestructiveBlocked(t *testing.T) {
    logger := &testLogger{}
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AuditLogger:      logger,
        AllowDestructive: false,
    })

    mock.ResetHard(context.Background(), "HEAD")

    entries := mock.GetAuditEntries()
    if len(entries) != 1 {
        t.Fatalf("expected 1 audit entry, got %d", len(entries))
    }

    entry := entries[0]
    if entry.ChecksPassed {
        t.Error("expected ChecksPassed=false")
    }
    if !strings.Contains(entry.FailureReason, "ErrDestructiveNotAllowed") {
        t.Errorf("expected FailureReason to contain ErrDestructiveNotAllowed, got %s", entry.FailureReason)
    }
}

func TestMockGitOps_GetAuditEntriesReturnsCopy(t *testing.T) {
    logger := &testLogger{}
    mock := NewMockGitOpsWithOpts("/repo", GitOpsOpts{
        AuditLogger:      logger,
        AllowDestructive: true,
    })

    mock.Status(context.Background())

    entries1 := mock.GetAuditEntries()
    entries1[0].Operation = "Modified" // Modify the copy

    entries2 := mock.GetAuditEntries()
    if entries2[0].Operation != "Status" {
        t.Error("GetAuditEntries should return a copy, not a reference")
    }
}
```

### Assertion Tests

```go
// internal/git/mock/assertions_test.go

func TestMockGitOps_AssertCallOrder(t *testing.T) {
    mock := NewMockGitOps("/repo")

    mock.Status(context.Background())
    mock.Add(context.Background(), "file.go")
    mock.Commit(context.Background(), "msg", CommitOpts{})

    // This should pass
    mock.AssertCallOrder(t, "Status", "Add", "Commit")
}

func TestMockGitOps_AssertNotCalled(t *testing.T) {
    mock := NewMockGitOps("/repo")

    mock.Status(context.Background())

    // This should pass
    mock.AssertNotCalled(t, "Commit")
}

func TestMockGitOps_GetCallsFor(t *testing.T) {
    mock := NewMockGitOps("/repo")

    mock.Add(context.Background(), "file1.go")
    mock.Status(context.Background())
    mock.Add(context.Background(), "file2.go")

    calls := mock.GetCallsFor("Add")

    if len(calls) != 2 {
        t.Errorf("expected 2 Add calls, got %d", len(calls))
    }
    if calls[0].Args[0].([]string)[0] != "file1.go" {
        t.Error("expected first Add call to have file1.go")
    }
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

### Integration Tests

| Scenario | Description |
|----------|-------------|
| Worker with MockGitOps | Verify worker uses GitOps correctly for a complete task cycle |
| Error propagation | Inject errors and verify calling code handles them properly |
| Concurrent workers | Multiple workers sharing a mock verify thread safety |
| Full workflow mock | Mock entire git workflow from clone to push |
| Safety constraint testing | Verify code correctly handles blocked operations |
| Audit log verification | Verify audit entries match expected operations |

### Manual Testing

- [ ] Create mock, set stub values, verify methods return expected values
- [ ] Verify assertion helpers fail tests correctly on mismatch
- [ ] Run parallel tests using mock to verify thread safety
- [ ] Verify Reset() clears calls but preserves stubs
- [ ] Verify ResetAll() clears both calls and stubs
- [ ] Verify destructive operations blocked when AllowDestructive=false
- [ ] Verify BranchGuard blocks protected branches
- [ ] Verify audit entries captured when AuditLogger configured

## Design Decisions

### Why a Concrete Mock Instead of Gomock/Mockery?

Generated mocks from tools like gomock or mockery add external dependencies and generate verbose setup code. A hand-written mock:
- Provides domain-specific assertion helpers (AssertCallOrder, GetCallsFor)
- Enables cleaner test code without EXPECT() chains
- Avoids generated code maintenance burden
- Allows stub values to be set directly on struct fields
- Enables safety feature simulation with consistent behavior

Trade-offs considered: Generated mocks auto-update with interface changes. Manual mocks require explicit updates when GitOps evolves.

### Why Store Arguments as []any?

Using `[]any` for arguments provides flexibility to record any argument type without defining method-specific call types. The alternative (typed call structs per method) would require 20+ structs and make assertion helpers more complex.

Trade-off: Type safety at assertion time relies on runtime checks rather than compile-time verification.

### Why Provide Reset() and ResetAll() Separately?

Common test patterns need to reset call history while preserving stub configuration:
- Subtests that verify different scenarios with the same mock configuration
- Setup functions that pre-configure stubs once for multiple tests

ResetAll() exists for tests that need complete isolation.

### Why Not Verify Context Cancellation?

Testing context cancellation behavior belongs in integration tests with real GitOps implementations. The mock focuses on verifying method call sequences and argument passing. Adding context verification would complicate the mock and conflate unit and integration testing concerns.

### Why Simulate Safety Features in the Mock?

The mock simulates safety features (BranchGuard, AllowDestructive, audit logging) because:
- Tests can verify code correctly handles safety errors without a real git repo
- Consistent behavior between mock and real implementation prevents test/production divergence
- Enables testing error handling paths that are difficult to trigger with real git
- Audit log capture allows verifying logging behavior in unit tests

Trade-off: Mock safety simulation must stay synchronized with real GitOps implementation.

### Why Default AllowDestructive=true in NewMockGitOps?

Most tests don't need safety constraints and want maximum flexibility. The default constructor optimizes for the common case. Tests that specifically need to verify safety behavior use `NewMockGitOpsWithOpts`.

## Future Enhancements

1. Call expectation mode: Pre-declare expected calls and fail if they don't happen
2. Sequence-dependent responses: Return different values on successive calls
3. Argument matchers: Support patterns like `Any()`, `Contains()`, `StartsWith()`
4. Call recording export: Serialize calls to JSON for debugging complex test failures
5. Automatic interface compliance checking at test time
6. Safety level presets: `WithStrictSafety()`, `WithRelaxedSafety()` helper methods
7. Remote URL validation simulation for Push operations

## References

- GITOPS spec (for interface definition)
- safe-git-operations PRD (for safety feature requirements)
- [testing.TB interface](https://pkg.go.dev/testing#TB)
- [sync.RWMutex](https://pkg.go.dev/sync#RWMutex)
