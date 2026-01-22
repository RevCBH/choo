package git

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
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
	StatusResult   StatusResult
	StatusErr      error
	RevParseResult string
	RevParseErr    error
	DiffResult     string
	DiffErr        error
	LogResult      []CommitRecord
	LogErr         error

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

// ResetCalls clears all recorded calls, preserving stub configuration.
func (m *MockGitOps) ResetCalls() {
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

func (m *MockGitOps) Log(ctx context.Context, opts LogOpts) ([]CommitRecord, error) {
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

// Working tree operations

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

// Assertion helpers

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
