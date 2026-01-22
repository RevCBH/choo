package git

import (
	"context"
	"sync"
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
