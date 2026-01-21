package client

import (
	"context"
	"errors"
	"testing"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockDaemonClient implements apiv1.DaemonServiceClient for testing
type mockDaemonClient struct {
	apiv1.DaemonServiceClient
	startJobFn     func(context.Context, *apiv1.StartJobRequest, ...grpc.CallOption) (*apiv1.StartJobResponse, error)
	stopJobFn      func(context.Context, *apiv1.StopJobRequest, ...grpc.CallOption) (*apiv1.StopJobResponse, error)
	listJobsFn     func(context.Context, *apiv1.ListJobsRequest, ...grpc.CallOption) (*apiv1.ListJobsResponse, error)
	getJobStatusFn func(context.Context, *apiv1.GetJobStatusRequest, ...grpc.CallOption) (*apiv1.GetJobStatusResponse, error)
}

func (m *mockDaemonClient) StartJob(ctx context.Context, req *apiv1.StartJobRequest, opts ...grpc.CallOption) (*apiv1.StartJobResponse, error) {
	if m.startJobFn != nil {
		return m.startJobFn(ctx, req, opts...)
	}
	return nil, errors.New("startJobFn not set")
}

func (m *mockDaemonClient) StopJob(ctx context.Context, req *apiv1.StopJobRequest, opts ...grpc.CallOption) (*apiv1.StopJobResponse, error) {
	if m.stopJobFn != nil {
		return m.stopJobFn(ctx, req, opts...)
	}
	return nil, errors.New("stopJobFn not set")
}

func (m *mockDaemonClient) ListJobs(ctx context.Context, req *apiv1.ListJobsRequest, opts ...grpc.CallOption) (*apiv1.ListJobsResponse, error) {
	if m.listJobsFn != nil {
		return m.listJobsFn(ctx, req, opts...)
	}
	return nil, errors.New("listJobsFn not set")
}

func (m *mockDaemonClient) GetJobStatus(ctx context.Context, req *apiv1.GetJobStatusRequest, opts ...grpc.CallOption) (*apiv1.GetJobStatusResponse, error) {
	if m.getJobStatusFn != nil {
		return m.getJobStatusFn(ctx, req, opts...)
	}
	return nil, errors.New("getJobStatusFn not set")
}

func TestStartJob_Success(t *testing.T) {
	expectedJobID := "job-123"
	mock := &mockDaemonClient{
		startJobFn: func(ctx context.Context, req *apiv1.StartJobRequest, opts ...grpc.CallOption) (*apiv1.StartJobResponse, error) {
			return &apiv1.StartJobResponse{
				JobId:  expectedJobID,
				Status: "running",
			}, nil
		},
	}

	client := &Client{daemon: mock}
	cfg := JobConfig{
		TasksDir:      "/path/to/tasks",
		TargetBranch:  "main",
		FeatureBranch: "feature",
		Parallelism:   4,
		RepoPath:      "/path/to/repo",
	}

	jobID, err := client.StartJob(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}
	if jobID != expectedJobID {
		t.Errorf("Expected job ID %s, got %s", expectedJobID, jobID)
	}
}

func TestStartJob_Error(t *testing.T) {
	expectedErr := errors.New("daemon unavailable")
	mock := &mockDaemonClient{
		startJobFn: func(ctx context.Context, req *apiv1.StartJobRequest, opts ...grpc.CallOption) (*apiv1.StartJobResponse, error) {
			return nil, expectedErr
		},
	}

	client := &Client{daemon: mock}
	cfg := JobConfig{
		TasksDir: "/path/to/tasks",
	}

	_, err := client.StartJob(context.Background(), cfg)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestStopJob_Force(t *testing.T) {
	var capturedForce bool
	mock := &mockDaemonClient{
		stopJobFn: func(ctx context.Context, req *apiv1.StopJobRequest, opts ...grpc.CallOption) (*apiv1.StopJobResponse, error) {
			capturedForce = req.GetForce()
			return &apiv1.StopJobResponse{
				Success: true,
				Message: "Job stopped",
			}, nil
		},
	}

	client := &Client{daemon: mock}

	err := client.StopJob(context.Background(), "job-123", true)
	if err != nil {
		t.Fatalf("StopJob failed: %v", err)
	}
	if !capturedForce {
		t.Error("Expected force flag to be true")
	}
}

func TestListJobs_WithFilter(t *testing.T) {
	var capturedFilter []string
	mock := &mockDaemonClient{
		listJobsFn: func(ctx context.Context, req *apiv1.ListJobsRequest, opts ...grpc.CallOption) (*apiv1.ListJobsResponse, error) {
			capturedFilter = req.GetStatusFilter()
			return &apiv1.ListJobsResponse{
				Jobs: []*apiv1.JobSummary{
					{
						JobId:         "job-1",
						FeatureBranch: "feature-1",
						Status:        "running",
						StartedAt:     timestamppb.Now(),
						UnitsComplete: 1,
						UnitsTotal:    5,
					},
				},
			}, nil
		},
	}

	client := &Client{daemon: mock}
	filter := []string{"running", "pending"}

	jobs, err := client.ListJobs(context.Background(), filter)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
	if len(capturedFilter) != 2 || capturedFilter[0] != "running" || capturedFilter[1] != "pending" {
		t.Errorf("Expected filter [running pending], got %v", capturedFilter)
	}
}

func TestListJobs_Empty(t *testing.T) {
	mock := &mockDaemonClient{
		listJobsFn: func(ctx context.Context, req *apiv1.ListJobsRequest, opts ...grpc.CallOption) (*apiv1.ListJobsResponse, error) {
			return &apiv1.ListJobsResponse{
				Jobs: []*apiv1.JobSummary{},
			}, nil
		},
	}

	client := &Client{daemon: mock}

	jobs, err := client.ListJobs(context.Background(), []string{})
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if jobs == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(jobs) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(jobs))
	}
}

func TestGetJobStatus_NotFound(t *testing.T) {
	expectedErr := errors.New("job not found")
	mock := &mockDaemonClient{
		getJobStatusFn: func(ctx context.Context, req *apiv1.GetJobStatusRequest, opts ...grpc.CallOption) (*apiv1.GetJobStatusResponse, error) {
			return nil, expectedErr
		},
	}

	client := &Client{daemon: mock}

	_, err := client.GetJobStatus(context.Background(), "nonexistent-job")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
