package daemon

import (
	"context"
	"errors"
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
	jobs         map[string]*JobState
	startErr     error
	stopErr      error
	stoppedJobs  map[string]bool
	forceStopped map[string]bool
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
	jobID := "job-" + time.Now().Format("20060102150405")
	now := time.Now()
	m.jobs[jobID] = &JobState{
		ID:        jobID,
		Status:    "running",
		StartedAt: &now,
	}
	return jobID, nil
}

func (m *mockJobManager) Stop(ctx context.Context, jobID string, force bool) error {
	if m.stopErr != nil {
		return m.stopErr
	}
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
	job, ok := m.jobs[jobID]
	if !ok {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (m *mockJobManager) ListJobs(statusFilter []string) ([]*JobSummary, error) {
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
			StartedAt: job.StartedAt,
		})
	}
	return result, nil
}

func (m *mockJobManager) Subscribe(jobID string, fromSeq int) (<-chan Event, func()) {
	ch := make(chan Event)
	return ch, func() { close(ch) }
}

func (m *mockJobManager) ActiveJobCount() int {
	count := 0
	for _, job := range m.jobs {
		if job.Status == "running" {
			count++
		}
	}
	return count
}

func (m *mockJobManager) addJob(id string, status string) {
	now := time.Now()
	m.jobs[id] = &JobState{
		ID:        id,
		Status:    status,
		StartedAt: &now,
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
