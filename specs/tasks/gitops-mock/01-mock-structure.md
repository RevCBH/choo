---
task: 1
status: pending
backpressure: "go build ./internal/git/..."
depends_on: []
---

# Mock Structure and Constructors

**Parent spec**: `/specs/GITOPS-MOCK.md`
**Task**: #1 of 4 in implementation plan

## Objective

Define MockGitOps struct with stub fields, call tracking, and constructors.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides GitOps interface, GitOpsOpts, all types

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- Standard library (`sync`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── mock_gitops.go    # CREATE: MockGitOps struct and constructors
```

### Types to Implement

```go
package git

import (
    "context"
    "sync"
    "time"
)

// MockGitOps provides a testable implementation of the GitOps interface.
// All methods record their calls and return configurable stub values.
// When configured with GitOpsOpts, simulates safety feature behavior.
type MockGitOps struct {
    path string
    opts GitOpsOpts
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
    AddErr    error
    AddAllErr error
    ResetErr  error

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
    AuditEntries []AuditEntry

    // Safety check simulation
    SimulateBranchGuardErr error // Set to simulate BranchGuard failure

    // Call tracking
    Calls []MockCall
}

// MockCall records a single method invocation with its arguments.
type MockCall struct {
    Method    string
    Args      []any
    BlockedBy string // Set if call was blocked by safety check
    Error     error  // Error returned, if any
}

// NewMockGitOps creates a new mock bound to the given path with default safe options.
// Default options: AllowDestructive=true (for test flexibility), no BranchGuard.
func NewMockGitOps(path string) *MockGitOps {
    return &MockGitOps{
        path: path,
        opts: GitOpsOpts{
            AllowDestructive: true, // Default for test flexibility
        },
        Calls: make([]MockCall, 0),
    }
}

// NewMockGitOpsWithOpts creates a new mock with specific options for safety simulation.
// Use this when testing code that depends on safety feature behavior.
func NewMockGitOpsWithOpts(path string, opts GitOpsOpts) *MockGitOps {
    return &MockGitOps{
        path:  path,
        opts:  opts,
        Calls: make([]MockCall, 0),
    }
}

// Path returns the repository path this mock is bound to.
func (m *MockGitOps) Path() string {
    return m.path
}

// Opts returns the options this mock was configured with.
func (m *MockGitOps) Opts() GitOpsOpts {
    return m.opts
}

// Reset clears all recorded calls, preserving stub configuration.
func (m *MockGitOps) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Calls = make([]MockCall, 0)
    m.AuditEntries = nil
}

// ResetAll clears both recorded calls and stub configuration.
func (m *MockGitOps) ResetAll() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Calls = make([]MockCall, 0)
    m.AuditEntries = nil
    m.StatusResult = StatusResult{}
    m.StatusErr = nil
    m.RevParseResult = ""
    m.RevParseErr = nil
    m.DiffResult = ""
    m.DiffErr = nil
    m.LogResult = nil
    m.LogErr = nil
    m.CurrentBranchResult = ""
    m.CurrentBranchErr = nil
    m.CheckoutBranchErr = nil
    m.BranchExistsResult = false
    m.BranchExistsErr = nil
    m.AddErr = nil
    m.AddAllErr = nil
    m.ResetErr = nil
    m.CommitErr = nil
    m.CheckoutFilesErr = nil
    m.CleanErr = nil
    m.ResetHardErr = nil
    m.FetchErr = nil
    m.PushErr = nil
    m.MergeErr = nil
    m.MergeAbortErr = nil
    m.SimulateBranchGuardErr = nil
}

// record adds a call to the call log (must hold lock)
func (m *MockGitOps) record(call MockCall) {
    m.Calls = append(m.Calls, call)
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/git/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| MockGitOps compiles | `git.NewMockGitOps("/path")` works |
| NewMockGitOpsWithOpts compiles | `git.NewMockGitOpsWithOpts("/path", opts)` works |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Uses RWMutex for thread-safe access to call log and stubs
- `record()` is internal and assumes caller holds lock
- Default AllowDestructive=true for maximum test flexibility
- Stub fields are public for direct test configuration

## NOT In Scope

- Interface method implementations (Task #2)
- Safety simulation logic (Task #3)
- Assertion helpers (Task #4)
