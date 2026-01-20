package feature

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/RevCBH/choo/internal/git"
	"github.com/stretchr/testify/assert"
)

// mockGitRunner is a test double for git.Runner
type mockGitRunner struct {
	mu        sync.Mutex
	responses map[string][]mockResponse
	calls     []mockCall
}

type mockResponse struct {
	out string
	err error
}

type mockCall struct {
	dir  string
	args []string
}

func newMockGitRunner() *mockGitRunner {
	return &mockGitRunner{
		responses: make(map[string][]mockResponse),
	}
}

func (m *mockGitRunner) stub(args string, out string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[args] = append(m.responses[args], mockResponse{out: out, err: err})
}

func (m *mockGitRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	m.mu.Lock()
	m.calls = append(m.calls, mockCall{dir: dir, args: append([]string(nil), args...)})
	queue := m.responses[key]
	if len(queue) == 0 {
		m.mu.Unlock()
		return "", fmt.Errorf("unexpected git call: %s", key)
	}
	resp := queue[0]
	m.responses[key] = queue[1:]
	m.mu.Unlock()
	return resp.out, resp.err
}

func (m *mockGitRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	return m.Exec(ctx, dir, args...)
}

func TestNewBranchManager_DefaultPrefix(t *testing.T) {
	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "")

	assert.Equal(t, "feature/", manager.GetPrefix())
}

func TestNewBranchManager_CustomPrefix(t *testing.T) {
	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "custom/")

	assert.Equal(t, "custom/", manager.GetPrefix())
}

func TestBranchManager_GetBranchName(t *testing.T) {
	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	branchName := manager.GetBranchName("streaming-events")
	assert.Equal(t, "feature/streaming-events", branchName)
}

func TestBranchManager_Create(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub responses: branch doesn't exist (both checks fail), then creation succeeds
	mockRunner.stub("rev-parse --verify feature/new-feature", "", fmt.Errorf("not found"))
	mockRunner.stub("rev-parse --verify origin/feature/new-feature", "", fmt.Errorf("not found"))
	mockRunner.stub("branch feature/new-feature main", "", nil)

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	err := manager.Create(ctx, "new-feature", "main")
	assert.NoError(t, err)
}

func TestBranchManager_Create_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub response: branch exists locally
	mockRunner.stub("rev-parse --verify feature/existing-feature", "abc123", nil)

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	err := manager.Create(ctx, "existing-feature", "main")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature branch feature/existing-feature already exists")
}

func TestBranchManager_Exists(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub response: branch exists locally
	mockRunner.stub("rev-parse --verify feature/test-feature", "abc123", nil)

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	exists, err := manager.Exists(ctx, "test-feature")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestBranchManager_Checkout(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub responses: branch exists, then checkout succeeds
	mockRunner.stub("rev-parse --verify feature/test-feature", "abc123", nil)
	mockRunner.stub("checkout feature/test-feature", "", nil)

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	err := manager.Checkout(ctx, "test-feature")
	assert.NoError(t, err)
}

func TestBranchManager_Checkout_NotExists(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub response: branch doesn't exist
	mockRunner.stub("rev-parse --verify feature/missing-feature", "", fmt.Errorf("not found"))
	mockRunner.stub("rev-parse --verify origin/feature/missing-feature", "", fmt.Errorf("not found"))

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	err := manager.Checkout(ctx, "missing-feature")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "feature branch feature/missing-feature does not exist")
}

func TestBranchManager_Delete(t *testing.T) {
	ctx := context.Background()
	mockRunner := newMockGitRunner()
	git.SetDefaultRunner(mockRunner)
	defer git.SetDefaultRunner(nil)

	// Stub response: delete succeeds
	mockRunner.stub("branch -D feature/old-feature", "", nil)

	client := git.NewClient("/test/repo")
	manager := NewBranchManager(client, "feature/")

	err := manager.Delete(ctx, "old-feature")
	assert.NoError(t, err)
}
