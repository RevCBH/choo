package daemon

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RevCBH/choo/internal/container"
	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/RevCBH/choo/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockContainerRuntime struct {
	mu        sync.Mutex
	createCfg container.ContainerConfig

	createFunc func(ctx context.Context, cfg container.ContainerConfig) (container.ContainerID, error)
	startFunc  func(ctx context.Context, id container.ContainerID) error
	waitFunc   func(ctx context.Context, id container.ContainerID) (int, error)
	logsFunc   func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error)
	removeFunc func(ctx context.Context, id container.ContainerID) error
}

var _ container.Manager = (*mockContainerRuntime)(nil)

func (m *mockContainerRuntime) Create(ctx context.Context, cfg container.ContainerConfig) (container.ContainerID, error) {
	m.mu.Lock()
	m.createCfg = cfg
	m.mu.Unlock()
	if m.createFunc != nil {
		return m.createFunc(ctx, cfg)
	}
	return "mock-container", nil
}

func (m *mockContainerRuntime) Start(ctx context.Context, id container.ContainerID) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, id)
	}
	return nil
}

func (m *mockContainerRuntime) Wait(ctx context.Context, id container.ContainerID) (int, error) {
	if m.waitFunc != nil {
		return m.waitFunc(ctx, id)
	}
	return 0, nil
}

func (m *mockContainerRuntime) Logs(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
	if m.logsFunc != nil {
		return m.logsFunc(ctx, id)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockContainerRuntime) Stop(ctx context.Context, id container.ContainerID, timeout time.Duration) error {
	return nil
}

func (m *mockContainerRuntime) Remove(ctx context.Context, id container.ContainerID) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, id)
	}
	return nil
}

func setupContainerJobManager(t *testing.T, runtime container.Manager) *jobManagerImpl {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = database.Close()
	})

	jm := NewJobManager(database, 10)
	jm.runtime = runtime
	jm.cfg.ContainerImage = "choo:latest"
	return jm
}

func TestStartContainerJob_CreatesContainer(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}
	jm := setupContainerJobManager(t, mockRuntime)

	cfg := ContainerJobConfig{
		JobID:        "job-123",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
		Unit:         "unit-1",
	}

	jobID, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "job-123", jobID)

	mockRuntime.mu.Lock()
	created := mockRuntime.createCfg
	mockRuntime.mu.Unlock()

	assert.Equal(t, "choo:latest", created.Image)
	assert.Equal(t, "choo-job-123", created.Name)
	assert.Equal(t, cfg.GitURL, created.Env["GIT_URL"])
	assert.Equal(t, []string{
		"choo", "run",
		"--clone-url", cfg.GitURL,
		"--tasks", cfg.TasksDir,
		"--json-events",
		"--unit", cfg.Unit,
	}, created.Cmd)
}

func TestStartContainerJob_PassesCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-gh-token")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	mockRuntime := &mockContainerRuntime{}
	jm := setupContainerJobManager(t, mockRuntime)

	cfg := ContainerJobConfig{
		JobID:        "job-cred",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	_, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	mockRuntime.mu.Lock()
	created := mockRuntime.createCfg
	mockRuntime.mu.Unlock()

	assert.Equal(t, "test-gh-token", created.Env["GITHUB_TOKEN"])
	assert.Equal(t, "test-anthropic-key", created.Env["ANTHROPIC_API_KEY"])
}

func TestStartContainerJob_MissingCredential(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	mockRuntime := &mockContainerRuntime{}
	jm := setupContainerJobManager(t, mockRuntime)

	cfg := ContainerJobConfig{
		JobID:        "job-missing-cred",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	_, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	mockRuntime.mu.Lock()
	created := mockRuntime.createCfg
	mockRuntime.mu.Unlock()

	_, hasGitHub := created.Env["GITHUB_TOKEN"]
	_, hasAnthropic := created.Env["ANTHROPIC_API_KEY"]
	assert.False(t, hasGitHub)
	assert.False(t, hasAnthropic)
}

func TestStartContainerJob_StartsLogStreamer(t *testing.T) {
	logsCalled := make(chan struct{})

	mockRuntime := &mockContainerRuntime{
		logsFunc: func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
			select {
			case <-logsCalled:
				// already closed
			default:
				close(logsCalled)
			}
			return io.NopCloser(strings.NewReader("")), nil
		},
	}

	jm := setupContainerJobManager(t, mockRuntime)
	cfg := ContainerJobConfig{
		JobID:        "job-logs",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	_, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	select {
	case <-logsCalled:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("log streamer did not start")
	}
}

func TestStartContainerJob_RecordsInDB(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}
	jm := setupContainerJobManager(t, mockRuntime)

	cfg := ContainerJobConfig{
		JobID:        "job-db",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	run, err := jm.db.GetRun(jobID)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, db.RunStatusRunning, run.Status)
	assert.Equal(t, cfg.RepoPath, run.RepoPath)
	assert.Equal(t, cfg.TasksDir, run.TasksDir)
}

func TestContainerJob_SuccessfulCompletion(t *testing.T) {
	waitCalled := make(chan struct{})

	mockRuntime := &mockContainerRuntime{
		waitFunc: func(ctx context.Context, id container.ContainerID) (int, error) {
			close(waitCalled)
			return 0, nil
		},
	}

	jm := setupContainerJobManager(t, mockRuntime)
	cfg := ContainerJobConfig{
		JobID:        "job-success",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	select {
	case <-waitCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("container wait not invoked")
	}

	require.Eventually(t, func() bool {
		run, err := jm.db.GetRun(jobID)
		if err != nil || run == nil {
			return false
		}
		return run.Status == db.RunStatusCompleted
	}, 2*time.Second, 20*time.Millisecond)
}

func TestContainerJob_FailureCapture(t *testing.T) {
	logPayload := "line-1\nline-2\nerror-line\n"
	callCount := 0

	mockRuntime := &mockContainerRuntime{
		logsFunc: func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
			callCount++
			if callCount == 1 {
				return io.NopCloser(strings.NewReader("")), nil
			}
			return io.NopCloser(strings.NewReader(logPayload)), nil
		},
		waitFunc: func(ctx context.Context, id container.ContainerID) (int, error) {
			return 2, nil
		},
	}

	jm := setupContainerJobManager(t, mockRuntime)
	collector := events.NewEventCollector(jm.eventBus)

	cfg := ContainerJobConfig{
		JobID:        "job-fail",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	_, err := jm.StartContainerJob(context.Background(), cfg)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		jm.eventBus.Wait()
		return collector.Len() > 0
	}, 2*time.Second, 20*time.Millisecond)

	eventsCaptured := collector.Get()
	require.NotEmpty(t, eventsCaptured)
	assert.Equal(t, events.OrchFailed, eventsCaptured[0].Type)

	payload, ok := eventsCaptured[0].Payload.(map[string]string)
	require.True(t, ok)
	assert.Contains(t, payload["details"], "error-line")
}

func TestGetContainerState_ReturnsState(t *testing.T) {
	waitCh := make(chan struct{})
	pr, pw := io.Pipe()

	mockRuntime := &mockContainerRuntime{
		logsFunc: func(ctx context.Context, id container.ContainerID) (io.ReadCloser, error) {
			return pr, nil
		},
		waitFunc: func(ctx context.Context, id container.ContainerID) (int, error) {
			<-waitCh
			return 0, nil
		},
	}

	jm := setupContainerJobManager(t, mockRuntime)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := ContainerJobConfig{
		JobID:        "job-state",
		RepoPath:     "/repo",
		GitURL:       "https://github.com/org/repo.git",
		TasksDir:     "specs/tasks",
		TargetBranch: "main",
	}

	jobID, err := jm.StartContainerJob(ctx, cfg)
	require.NoError(t, err)

	state, err := jm.GetContainerState(jobID)
	require.NoError(t, err)
	assert.Equal(t, ContainerStatusRunning, state.Status)
	assert.Equal(t, "choo-job-state", state.ContainerName)
	assert.Equal(t, "mock-container", state.ContainerID)

	_ = pw.Close()
	close(waitCh)
}

func TestGetContainerState_NotFound(t *testing.T) {
	mockRuntime := &mockContainerRuntime{}
	jm := setupContainerJobManager(t, mockRuntime)

	_, err := jm.GetContainerState("missing")
	assert.Error(t, err)
}

func TestBuildContainerEnv_IncludesGitURL(t *testing.T) {
	cfg := ContainerJobConfig{
		GitURL: "https://github.com/org/repo.git",
	}

	env := buildContainerEnv(cfg)

	if env["GIT_URL"] != cfg.GitURL {
		t.Errorf("GIT_URL = %q, want %q", env["GIT_URL"], cfg.GitURL)
	}
}

func TestBuildContainerEnv_IncludesCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-gh-token")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	cfg := ContainerJobConfig{
		GitURL: "https://github.com/org/repo.git",
	}

	env := buildContainerEnv(cfg)

	if env["GITHUB_TOKEN"] != "test-gh-token" {
		t.Error("GITHUB_TOKEN not passed through")
	}
	if env["ANTHROPIC_API_KEY"] != "test-anthropic-key" {
		t.Error("ANTHROPIC_API_KEY not passed through")
	}
}

func TestBuildContainerEnv_MissingCredentialsOmitted(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg := ContainerJobConfig{
		GitURL: "https://github.com/org/repo.git",
	}

	env := buildContainerEnv(cfg)

	if _, ok := env["GITHUB_TOKEN"]; ok {
		t.Error("GITHUB_TOKEN should not be present when unset")
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be present when unset")
	}
}

func TestTailLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"fewer than n", "a\nb\nc", 5, "a\nb\nc"},
		{"exactly n", "a\nb\nc", 3, "a\nb\nc"},
		{"more than n", "a\nb\nc\nd\ne", 3, "c\nd\ne"},
		{"empty", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tailLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("tailLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}
