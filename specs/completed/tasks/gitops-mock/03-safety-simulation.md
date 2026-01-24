---
task: 3
status: complete
backpressure: "go test ./internal/git/... -run TestMockGitOps_Safety -v"
depends_on: [2]
---

# Safety Simulation

**Parent spec**: `/specs/GITOPS-MOCK.md`
**Task**: #3 of 4 in implementation plan

## Objective

Add safety feature simulation: AllowDestructive checks, BranchGuard validation, and audit log capture.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides ErrDestructiveNotAllowed, ErrProtectedBranch, ErrUnexpectedBranch

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: method implementations to modify)

### Package Dependencies
- Standard library (`fmt`, `strings`, `time`)

## Deliverables

### Files to Modify

```
internal/git/
├── mock_gitops.go      # MODIFY: Add safety checks to destructive operations
└── mock_gitops_test.go # MODIFY: Add safety simulation tests
```

### Functions to Implement

```go
// simulateBranchGuard checks BranchGuard rules against CurrentBranchResult
func (m *MockGitOps) simulateBranchGuard() error {
    // Allow direct error injection
    if m.SimulateBranchGuardErr != nil {
        return m.SimulateBranchGuardErr
    }

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

// captureAudit records an audit entry
func (m *MockGitOps) captureAudit(operation string, passed bool, failureReason string) {
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

    if m.opts.AuditLogger != nil {
        m.opts.AuditLogger.Log(entry)
    }
}

func (m *MockGitOps) getSafetyChecks(operation string) []string {
    checks := []string{"path_valid"}
    if m.opts.BranchGuard != nil {
        checks = append(checks, "branch_guard")
    }
    if isDestructiveOperation(operation) {
        checks = append(checks, "destructive_allowed")
    }
    return checks
}

func isDestructiveOperation(operation string) bool {
    switch operation {
    case "ResetHard", "Clean", "CheckoutFiles":
        return true
    case "Push": // Force push is destructive
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

// Updated destructive operations with safety checks

func (m *MockGitOps) CheckoutFiles(ctx context.Context, paths ...string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !m.opts.AllowDestructive {
        call := MockCall{
            Method:    "CheckoutFiles",
            Args:      []any{paths},
            BlockedBy: "AllowDestructive",
            Error:     ErrDestructiveNotAllowed,
        }
        m.record(call)
        m.captureAudit("CheckoutFiles", false, ErrDestructiveNotAllowed.Error())
        return fmt.Errorf("%w: CheckoutFiles", ErrDestructiveNotAllowed)
    }

    m.record(MockCall{Method: "CheckoutFiles", Args: []any{paths}})
    m.captureAudit("CheckoutFiles", true, "")
    return m.CheckoutFilesErr
}

func (m *MockGitOps) Clean(ctx context.Context, opts CleanOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !m.opts.AllowDestructive {
        call := MockCall{
            Method:    "Clean",
            Args:      []any{opts},
            BlockedBy: "AllowDestructive",
            Error:     ErrDestructiveNotAllowed,
        }
        m.record(call)
        m.captureAudit("Clean", false, ErrDestructiveNotAllowed.Error())
        return fmt.Errorf("%w: Clean", ErrDestructiveNotAllowed)
    }

    m.record(MockCall{Method: "Clean", Args: []any{opts}})
    m.captureAudit("Clean", true, "")
    return m.CleanErr
}

func (m *MockGitOps) ResetHard(ctx context.Context, ref string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !m.opts.AllowDestructive {
        call := MockCall{
            Method:    "ResetHard",
            Args:      []any{ref},
            BlockedBy: "AllowDestructive",
            Error:     ErrDestructiveNotAllowed,
        }
        m.record(call)
        m.captureAudit("ResetHard", false, ErrDestructiveNotAllowed.Error())
        return fmt.Errorf("%w: ResetHard", ErrDestructiveNotAllowed)
    }

    if err := m.simulateBranchGuard(); err != nil {
        call := MockCall{
            Method:    "ResetHard",
            Args:      []any{ref},
            BlockedBy: "BranchGuard",
            Error:     err,
        }
        m.record(call)
        m.captureAudit("ResetHard", false, err.Error())
        return err
    }

    m.record(MockCall{Method: "ResetHard", Args: []any{ref}})
    m.captureAudit("ResetHard", true, "")
    return m.ResetHardErr
}

func (m *MockGitOps) Commit(ctx context.Context, msg string, opts CommitOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if err := m.simulateBranchGuard(); err != nil {
        call := MockCall{
            Method:    "Commit",
            Args:      []any{msg, opts},
            BlockedBy: "BranchGuard",
            Error:     err,
        }
        m.record(call)
        m.captureAudit("Commit", false, err.Error())
        return err
    }

    m.record(MockCall{Method: "Commit", Args: []any{msg, opts}})
    m.captureAudit("Commit", true, "")
    return m.CommitErr
}

func (m *MockGitOps) Push(ctx context.Context, remote, branch string, opts PushOpts) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if (opts.Force || opts.ForceWithLease) && !m.opts.AllowDestructive {
        call := MockCall{
            Method:    "Push",
            Args:      []any{remote, branch, opts},
            BlockedBy: "AllowDestructive",
            Error:     ErrDestructiveNotAllowed,
        }
        m.record(call)
        m.captureAudit("Push", false, ErrDestructiveNotAllowed.Error())
        return fmt.Errorf("%w: Push --force", ErrDestructiveNotAllowed)
    }

    if err := m.simulateBranchGuard(); err != nil {
        call := MockCall{
            Method:    "Push",
            Args:      []any{remote, branch, opts},
            BlockedBy: "BranchGuard",
            Error:     err,
        }
        m.record(call)
        m.captureAudit("Push", false, err.Error())
        return err
    }

    m.record(MockCall{Method: "Push", Args: []any{remote, branch, opts}})
    m.captureAudit("Push", true, "")
    return m.PushErr
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestMockGitOps_Safety -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestMockGitOps_SafetyDestructiveBlocked` | ResetHard returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestMockGitOps_SafetyCleanBlocked` | Clean returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestMockGitOps_SafetyCheckoutFilesBlocked` | CheckoutFiles returns ErrDestructiveNotAllowed when AllowDestructive=false |
| `TestMockGitOps_SafetyForcePushBlocked` | Push with Force returns ErrDestructiveNotAllowed |
| `TestMockGitOps_SafetyDestructiveAllowed` | Operations succeed when AllowDestructive=true |
| `TestMockGitOps_SafetyBranchGuardProtected` | Commit returns ErrProtectedBranch on main |
| `TestMockGitOps_SafetyBranchGuardAllowed` | Commit succeeds on feature branch |
| `TestMockGitOps_SafetyAuditLogged` | Audit entries captured for operations |

### Test Implementation

```go
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

type testAuditLogger struct {
    entries []AuditEntry
}

func (l *testAuditLogger) Log(entry AuditEntry) {
    l.entries = append(l.entries, entry)
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Safety checks run BEFORE recording the call for blocked operations
- BlockedBy field in MockCall indicates which safety check rejected the call
- GetAuditEntries returns a copy to prevent test interference
- Default protected branches are main/master when ProtectedBranches is empty

## NOT In Scope

- Assertion helpers — Task #4
