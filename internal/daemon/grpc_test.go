package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockJobManager implements JobManager for testing
type mockJobManager struct {
	mu            sync.RWMutex
	jobs          map[string]*JobState
	startErr      error
	stopErr       error
	stoppedJobs   map[string]bool
	forceStopped  map[string]bool
	subscribeFunc func(jobID string, fromSeq int) (<-chan Event, func())
}

func newMockJobManager() *mockJobManager {
	return &mockJobManager{
		jobs:         make(map[string]*JobState),
		stoppedJobs:  make(map[string]bool),
		forceStopped: make(map[string]bool),
	}
}

func (m *mockJobManager) Start(ctx context.Context, cfg JobConfig) (string, error) {
	if m.startErr != nil {
		return "", m.startErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	jobID := "job-" + time.Now().Format("20060102150405")
	m.jobs[jobID] = &JobState{
		ID:        jobID,
		Status:    "running",
		StartedAt: time.Now(),
	}
	return jobID, nil
}

func (m *mockJobManager) Stop(ctx context.Context, jobID string, force bool) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stoppedJobs[jobID] = true
	if force {
		m.forceStopped[jobID] = true
	}
	if job, ok := m.jobs[jobID]; ok {
		job.Status = "cancelled"
	}
	return nil
}

func (m *mockJobManager) GetJob(jobID string) (*JobState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[jobID]
	if !ok {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (m *mockJobManager) ListJobs(statusFilter []string) ([]*JobSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*JobSummary
	for _, job := range m.jobs {
		if len(statusFilter) > 0 {
			found := false
			for _, s := range statusFilter {
				if job.Status == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, &JobSummary{
			JobID:     job.ID,
			Status:    job.Status,
			StartedAt: &job.StartedAt,
		})
	}
	return result, nil
}

func (m *mockJobManager) Subscribe(jobID string, fromSeq int) (<-chan Event, func()) {
	if m.subscribeFunc != nil {
		return m.subscribeFunc(jobID, fromSeq)
	}
	ch := make(chan Event)
	return ch, func() { close(ch) }
}

func (m *mockJobManager) ActiveJobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, job := range m.jobs {
		if job.Status == "running" {
			count++
		}
	}
	return count
}

func (m *mockJobManager) addJob(id string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[id] = &JobState{
		ID:        id,
		Status:    status,
		StartedAt: time.Now(),
	}
}

func (m *mockJobManager) setJobStatus(id string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, ok := m.jobs[id]; ok {
		job.Status = status
	}
}

func TestGRPC_JobStartJob_ValidatesRequiredFields(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")

	tests := []struct {
		name    string
		req     *apiv1.StartJobRequest
		wantErr codes.Code
	}{
		{
			name:    "missing tasks_dir",
			req:     &apiv1.StartJobRequest{TargetBranch: "main", RepoPath: "/repo"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:    "missing target_branch",
			req:     &apiv1.StartJobRequest{TasksDir: "/tasks", RepoPath: "/repo"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:    "missing repo_path",
			req:     &apiv1.StartJobRequest{TasksDir: "/tasks", TargetBranch: "main"},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "valid request",
			req: &apiv1.StartJobRequest{
				TasksDir:     "/tasks",
				TargetBranch: "main",
				RepoPath:     "/repo",
			},
			wantErr: codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.StartJob(context.Background(), tt.req)
			if tt.wantErr == codes.OK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, status.Code(err))
			}
		})
	}
}

func TestGRPC_JobStartJob_RejectsWhenShuttingDown(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")
	server.setShuttingDown()

	req := &apiv1.StartJobRequest{
		TasksDir:     "/tasks",
		TargetBranch: "main",
		RepoPath:     "/repo",
	}

	_, err := server.StartJob(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestGRPC_JobStopJob_GracefulStop(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-123", "running")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
		JobId: "job-123",
		Force: false,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.True(t, jm.stoppedJobs["job-123"])
	assert.False(t, jm.forceStopped["job-123"])
}

func TestGRPC_JobStopJob_ForceStop(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-456", "running")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
		JobId: "job-456",
		Force: true,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.True(t, jm.forceStopped["job-456"])
}

func TestGRPC_JobStopJob_NotFound(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")

	_, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
		JobId: "nonexistent",
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGRPC_JobStopJob_AlreadyStopped(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-done", "completed")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	_, err := server.StopJob(context.Background(), &apiv1.StopJobRequest{
		JobId: "job-done",
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGRPC_JobGetJobStatus(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-status", "running")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.GetJobStatus(context.Background(), &apiv1.GetJobStatusRequest{
		JobId: "job-status",
	})

	require.NoError(t, err)
	assert.Equal(t, "job-status", resp.JobId)
	assert.Equal(t, "running", resp.Status)
}

func TestGRPC_JobListJobs_NoFilter(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-1", "running")
	jm.addJob("job-2", "completed")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.ListJobs(context.Background(), &apiv1.ListJobsRequest{})

	require.NoError(t, err)
	assert.Len(t, resp.Jobs, 2)
}

func TestGRPC_JobListJobs_WithFilter(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-1", "running")
	jm.addJob("job-2", "completed")
	jm.addJob("job-3", "running")
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.ListJobs(context.Background(), &apiv1.ListJobsRequest{
		StatusFilter: []string{"running"},
	})

	require.NoError(t, err)
	assert.Len(t, resp.Jobs, 2)
	for _, job := range resp.Jobs {
		assert.Equal(t, "running", job.Status)
	}
}

// mockWatchStream implements DaemonService_WatchJobServer for testing
type mockWatchStream struct {
	apiv1.DaemonService_WatchJobServer
	events  []*apiv1.JobEvent
	ctx     context.Context
	cancel  context.CancelFunc
	sendErr error
	mu      sync.Mutex
}

func newMockWatchStream() *mockWatchStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockWatchStream{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *mockWatchStream) Send(event *apiv1.JobEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.events = append(m.events, event)
	return nil
}

func (m *mockWatchStream) Context() context.Context {
	return m.ctx
}

func (m *mockWatchStream) getEvents() []*apiv1.JobEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*apiv1.JobEvent, len(m.events))
	copy(result, m.events)
	return result
}

func TestGRPC_WatchJob_ValidatesJobID(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId: "",
	}, stream)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGRPC_WatchJob_JobNotFound(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId: "nonexistent",
	}, stream)

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGRPC_WatchJob_StreamsEvents(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-stream", "running")

	// Setup event channel that will close after sending events
	eventsCh := make(chan Event, 3)
	eventsCh <- Event{Sequence: 1, EventType: "unit_started", UnitID: "unit-1"}
	eventsCh <- Event{Sequence: 2, EventType: "task_completed", UnitID: "unit-1"}
	eventsCh <- Event{Sequence: 3, EventType: "unit_completed", UnitID: "unit-1"}
	close(eventsCh)

	jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
		return eventsCh, func() {}
	}

	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId:        "job-stream",
		FromSequence: 0,
	}, stream)

	require.NoError(t, err)
	events := stream.getEvents()
	assert.Len(t, events, 3)
	assert.Equal(t, int32(1), events[0].Sequence)
	assert.Equal(t, "unit_started", events[0].EventType)
	assert.Equal(t, int32(2), events[1].Sequence)
	assert.Equal(t, int32(3), events[2].Sequence)
}

func TestGRPC_WatchJob_ReplayFromSequence(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-replay", "running")

	// Create channel with events starting from sequence 3
	eventsCh := make(chan Event, 2)
	eventsCh <- Event{Sequence: 3, EventType: "task_completed"}
	eventsCh <- Event{Sequence: 4, EventType: "unit_completed"}
	close(eventsCh)

	var capturedFromSeq int
	jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
		capturedFromSeq = fromSeq
		return eventsCh, func() {}
	}

	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId:        "job-replay",
		FromSequence: 2,
	}, stream)

	require.NoError(t, err)
	assert.Equal(t, 2, capturedFromSeq)
	events := stream.getEvents()
	assert.Len(t, events, 2)
	assert.Equal(t, int32(3), events[0].Sequence)
}

func TestGRPC_WatchJob_ClientDisconnect(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-disconnect", "running")

	// Create a channel that blocks
	blockingCh := make(chan Event)
	jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
		return blockingCh, func() { close(blockingCh) }
	}

	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	// Start watching in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.WatchJob(&apiv1.WatchJobRequest{
			JobId: "job-disconnect",
		}, stream)
	}()

	// Simulate client disconnect
	time.Sleep(10 * time.Millisecond)
	stream.cancel()

	// Should return context error
	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("WatchJob did not return after client disconnect")
	}
}

func TestGRPC_WatchJob_ServerShutdown(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-shutdown", "running")

	blockingCh := make(chan Event)
	jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
		return blockingCh, func() { close(blockingCh) }
	}

	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.WatchJob(&apiv1.WatchJobRequest{
			JobId: "job-shutdown",
		}, stream)
	}()

	// Trigger shutdown
	time.Sleep(10 * time.Millisecond)
	server.setShuttingDown()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.Equal(t, codes.Unavailable, status.Code(err))
	case <-time.After(time.Second):
		t.Fatal("WatchJob did not return after server shutdown")
	}
}

func TestGRPC_WatchJob_SendError(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-send-err", "running")

	eventsCh := make(chan Event, 1)
	eventsCh <- Event{Sequence: 1, EventType: "test"}

	jm.subscribeFunc = func(jobID string, fromSeq int) (<-chan Event, func()) {
		return eventsCh, func() { close(eventsCh) }
	}

	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()
	stream.sendErr = errors.New("connection reset")

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId: "job-send-err",
	}, stream)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

func TestGRPC_WatchJob_CompletedJobNoReplay(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-done", "completed")
	server := NewGRPCServer(nil, jm, "v1.0.0")
	stream := newMockWatchStream()

	err := server.WatchJob(&apiv1.WatchJobRequest{
		JobId:        "job-done",
		FromSequence: 0,
	}, stream)

	// Should return immediately with no events
	require.NoError(t, err)
	assert.Empty(t, stream.getEvents())
}

func TestGRPC_LifecycleHealth_ReturnsStatus(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-1", "running")
	jm.addJob("job-2", "running")
	jm.addJob("job-3", "completed")

	server := NewGRPCServer(nil, jm, "v1.2.3")

	resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, int32(2), resp.ActiveJobs) // Only running jobs
	assert.Equal(t, "v1.2.3", resp.Version)
}

func TestGRPC_LifecycleHealth_UnhealthyDuringShutdown(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")
	server.setShuttingDown()

	resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

	require.NoError(t, err)
	assert.False(t, resp.Healthy)
}

func TestGRPC_LifecycleShutdown_NoJobs(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
		WaitForJobs: false,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, int32(0), resp.JobsStopped)
	assert.True(t, server.isShuttingDown())
}

func TestGRPC_LifecycleShutdown_ForceStopJobs(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-1", "running")
	jm.addJob("job-2", "running")

	server := NewGRPCServer(nil, jm, "v1.0.0")

	// Track the jobs so they show in activeJobs
	server.trackJob("job-1", func() {})
	server.trackJob("job-2", func() {})

	resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
		WaitForJobs: false,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, int32(2), resp.JobsStopped)
}

func TestGRPC_LifecycleShutdown_WaitForJobs(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-wait", "running")

	server := NewGRPCServer(nil, jm, "v1.0.0")
	server.trackJob("job-wait", func() {})

	// Simulate job completing during wait
	go func() {
		time.Sleep(50 * time.Millisecond)
		jm.setJobStatus("job-wait", "completed")
	}()

	resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
		WaitForJobs:    true,
		TimeoutSeconds: 5,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, int32(0), resp.JobsStopped) // Completed naturally
}

func TestGRPC_LifecycleShutdown_WaitTimeout(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-slow", "running")

	server := NewGRPCServer(nil, jm, "v1.0.0")
	server.trackJob("job-slow", func() {})

	// Job never completes, timeout will trigger
	resp, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
		WaitForJobs:    true,
		TimeoutSeconds: 1, // Short timeout
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, int32(1), resp.JobsStopped) // Force stopped after timeout
}

func TestGRPC_LifecycleShutdown_AlreadyShuttingDown(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")

	// First shutdown
	_, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{})
	require.NoError(t, err)

	// Second shutdown should fail
	_, err = server.Shutdown(context.Background(), &apiv1.ShutdownRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGRPC_LifecycleShutdown_CancelsJobContexts(t *testing.T) {
	jm := newMockJobManager()
	jm.addJob("job-ctx", "running")

	server := NewGRPCServer(nil, jm, "v1.0.0")

	// Track job with a cancel function we can verify
	cancelled := false
	server.trackJob("job-ctx", func() { cancelled = true })

	_, err := server.Shutdown(context.Background(), &apiv1.ShutdownRequest{
		WaitForJobs: false,
	})

	require.NoError(t, err)
	assert.True(t, cancelled, "job context should be cancelled")
}

func TestGRPC_LifecycleHealth_ZeroActiveJobs(t *testing.T) {
	jm := newMockJobManager()
	server := NewGRPCServer(nil, jm, "v1.0.0")

	resp, err := server.Health(context.Background(), &apiv1.HealthRequest{})

	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, int32(0), resp.ActiveJobs)
}
