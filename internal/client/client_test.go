package client

import (
	"context"
	"errors"
	"io"
	"testing"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/RevCBH/choo/internal/events"
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
	healthFn       func(context.Context, *apiv1.HealthRequest, ...grpc.CallOption) (*apiv1.HealthResponse, error)
	shutdownFn     func(context.Context, *apiv1.ShutdownRequest, ...grpc.CallOption) (*apiv1.ShutdownResponse, error)
	watchJobFn     func(context.Context, *apiv1.WatchJobRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error)
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

func (m *mockDaemonClient) Health(ctx context.Context, req *apiv1.HealthRequest, opts ...grpc.CallOption) (*apiv1.HealthResponse, error) {
	if m.healthFn != nil {
		return m.healthFn(ctx, req, opts...)
	}
	return nil, errors.New("healthFn not set")
}

func (m *mockDaemonClient) Shutdown(ctx context.Context, req *apiv1.ShutdownRequest, opts ...grpc.CallOption) (*apiv1.ShutdownResponse, error) {
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx, req, opts...)
	}
	return nil, errors.New("shutdownFn not set")
}

func (m *mockDaemonClient) WatchJob(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
	if m.watchJobFn != nil {
		return m.watchJobFn(ctx, req, opts...)
	}
	return nil, errors.New("watchJobFn not set")
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

func TestHealth_Success(t *testing.T) {
	mock := &mockDaemonClient{
		healthFn: func(ctx context.Context, req *apiv1.HealthRequest, opts ...grpc.CallOption) (*apiv1.HealthResponse, error) {
			return &apiv1.HealthResponse{
				Healthy:    true,
				ActiveJobs: 3,
				Version:    "1.0.0",
			}, nil
		},
	}

	client := &Client{daemon: mock}

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if !health.Healthy {
		t.Error("Expected healthy to be true")
	}
	if health.ActiveJobs != 3 {
		t.Errorf("Expected 3 active jobs, got %d", health.ActiveJobs)
	}
	if health.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", health.Version)
	}
}

func TestHealth_Unavailable(t *testing.T) {
	expectedErr := errors.New("daemon unavailable")
	mock := &mockDaemonClient{
		healthFn: func(ctx context.Context, req *apiv1.HealthRequest, opts ...grpc.CallOption) (*apiv1.HealthResponse, error) {
			return nil, expectedErr
		},
	}

	client := &Client{daemon: mock}

	_, err := client.Health(context.Background())
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestShutdown_WaitForJobs(t *testing.T) {
	var capturedWaitForJobs bool
	var capturedTimeout int32
	mock := &mockDaemonClient{
		shutdownFn: func(ctx context.Context, req *apiv1.ShutdownRequest, opts ...grpc.CallOption) (*apiv1.ShutdownResponse, error) {
			capturedWaitForJobs = req.GetWaitForJobs()
			capturedTimeout = req.GetTimeoutSeconds()
			return &apiv1.ShutdownResponse{
				Success: true,
			}, nil
		},
	}

	client := &Client{daemon: mock}

	err := client.Shutdown(context.Background(), true, 30)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if !capturedWaitForJobs {
		t.Error("Expected waitForJobs to be true")
	}
	if capturedTimeout != 30 {
		t.Errorf("Expected timeout 30, got %d", capturedTimeout)
	}
}

func TestShutdown_Immediate(t *testing.T) {
	var capturedWaitForJobs bool
	var capturedTimeout int32
	mock := &mockDaemonClient{
		shutdownFn: func(ctx context.Context, req *apiv1.ShutdownRequest, opts ...grpc.CallOption) (*apiv1.ShutdownResponse, error) {
			capturedWaitForJobs = req.GetWaitForJobs()
			capturedTimeout = req.GetTimeoutSeconds()
			return &apiv1.ShutdownResponse{
				Success: true,
			}, nil
		},
	}

	client := &Client{daemon: mock}

	err := client.Shutdown(context.Background(), false, 0)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if capturedWaitForJobs {
		t.Error("Expected waitForJobs to be false")
	}
	if capturedTimeout != 0 {
		t.Errorf("Expected timeout 0, got %d", capturedTimeout)
	}
}

func TestShutdown_Timeout(t *testing.T) {
	var capturedTimeout int32
	mock := &mockDaemonClient{
		shutdownFn: func(ctx context.Context, req *apiv1.ShutdownRequest, opts ...grpc.CallOption) (*apiv1.ShutdownResponse, error) {
			capturedTimeout = req.GetTimeoutSeconds()
			return &apiv1.ShutdownResponse{
				Success: true,
			}, nil
		},
	}

	client := &Client{daemon: mock}

	err := client.Shutdown(context.Background(), true, 60)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if capturedTimeout != 60 {
		t.Errorf("Expected timeout 60, got %d", capturedTimeout)
	}
}

// mockJobEventStream implements grpc.ServerStreamingClient[apiv1.JobEvent] for testing
type mockJobEventStream struct {
	grpc.ServerStreamingClient[apiv1.JobEvent]
	events []*apiv1.JobEvent
	index  int
	err    error
}

func (m *mockJobEventStream) Recv() (*apiv1.JobEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.index >= len(m.events) {
		return nil, io.EOF
	}
	event := m.events[m.index]
	m.index++
	return event, nil
}

func TestWatchJob_ReceivesEvents(t *testing.T) {
	now := timestamppb.Now()
	jobEvents := []*apiv1.JobEvent{
		{
			Sequence:    1,
			EventType:   "unit.started",
			UnitId:      "unit-1",
			PayloadJson: `{"foo":"bar"}`,
			Timestamp:   now,
		},
		{
			Sequence:    2,
			EventType:   "task.completed",
			UnitId:      "unit-1",
			PayloadJson: `{"task":1}`,
			Timestamp:   now,
		},
	}

	mock := &mockDaemonClient{
		watchJobFn: func(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
			return &mockJobEventStream{events: jobEvents}, nil
		},
	}

	client := &Client{daemon: mock}

	var receivedEvents []events.Event
	handler := func(e events.Event) {
		receivedEvents = append(receivedEvents, e)
	}

	err := client.WatchJob(context.Background(), "job-123", 0, handler)
	if err != nil {
		t.Fatalf("WatchJob failed: %v", err)
	}

	if len(receivedEvents) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(receivedEvents))
	}

	if receivedEvents[0].Type != "unit.started" {
		t.Errorf("Expected first event type 'unit.started', got %s", receivedEvents[0].Type)
	}
	if receivedEvents[0].Unit != "unit-1" {
		t.Errorf("Expected unit 'unit-1', got %s", receivedEvents[0].Unit)
	}

	if receivedEvents[1].Type != "task.completed" {
		t.Errorf("Expected second event type 'task.completed', got %s", receivedEvents[1].Type)
	}
}

func TestWatchJob_EOF(t *testing.T) {
	mock := &mockDaemonClient{
		watchJobFn: func(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
			return &mockJobEventStream{events: []*apiv1.JobEvent{}}, nil
		},
	}

	client := &Client{daemon: mock}

	var receivedEvents []events.Event
	handler := func(e events.Event) {
		receivedEvents = append(receivedEvents, e)
	}

	err := client.WatchJob(context.Background(), "job-123", 0, handler)
	if err != nil {
		t.Fatalf("Expected nil on EOF, got %v", err)
	}

	if len(receivedEvents) != 0 {
		t.Errorf("Expected 0 events, got %d", len(receivedEvents))
	}
}

func TestWatchJob_Error(t *testing.T) {
	expectedErr := errors.New("stream error")
	mock := &mockDaemonClient{
		watchJobFn: func(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
			return &mockJobEventStream{err: expectedErr}, nil
		},
	}

	client := &Client{daemon: mock}

	var receivedEvents []events.Event
	handler := func(e events.Event) {
		receivedEvents = append(receivedEvents, e)
	}

	err := client.WatchJob(context.Background(), "job-123", 0, handler)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if len(receivedEvents) != 0 {
		t.Errorf("Expected 0 events before error, got %d", len(receivedEvents))
	}
}

func TestWatchJob_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a stream that would block forever
	jobEvents := []*apiv1.JobEvent{
		{
			Sequence:  1,
			EventType: "unit.started",
			UnitId:    "unit-1",
			Timestamp: timestamppb.Now(),
		},
	}

	mock := &mockDaemonClient{
		watchJobFn: func(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
			return &mockJobEventStream{events: jobEvents, err: context.Canceled}, nil
		},
	}

	client := &Client{daemon: mock}

	var receivedEvents []events.Event
	handler := func(e events.Event) {
		receivedEvents = append(receivedEvents, e)
		cancel() // Cancel after first event
	}

	err := client.WatchJob(ctx, "job-123", 0, handler)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestWatchJob_FromSequence(t *testing.T) {
	var capturedFromSeq int32
	mock := &mockDaemonClient{
		watchJobFn: func(ctx context.Context, req *apiv1.WatchJobRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[apiv1.JobEvent], error) {
			capturedFromSeq = req.GetFromSequence()
			return &mockJobEventStream{events: []*apiv1.JobEvent{}}, nil
		},
	}

	client := &Client{daemon: mock}

	err := client.WatchJob(context.Background(), "job-123", 42, func(e events.Event) {})
	if err != nil {
		t.Fatalf("WatchJob failed: %v", err)
	}

	if capturedFromSeq != 42 {
		t.Errorf("Expected fromSequence 42, got %d", capturedFromSeq)
	}
}

func TestProtoToEvent(t *testing.T) {
	now := timestamppb.Now()
	proto := &apiv1.JobEvent{
		Sequence:    5,
		EventType:   "unit.completed",
		UnitId:      "unit-42",
		PayloadJson: `{"result":"success","pr_number":123}`,
		Timestamp:   now,
	}

	event := protoToEvent(proto)

	if event.Type != "unit.completed" {
		t.Errorf("Expected type 'unit.completed', got %s", event.Type)
	}
	if event.Unit != "unit-42" {
		t.Errorf("Expected unit 'unit-42', got %s", event.Unit)
	}
	if event.Time != now.AsTime() {
		t.Errorf("Expected time %v, got %v", now.AsTime(), event.Time)
	}

	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}
	if payload["result"] != "success" {
		t.Errorf("Expected result 'success', got %v", payload["result"])
	}
	if payload["pr_number"] != float64(123) {
		t.Errorf("Expected pr_number 123, got %v", payload["pr_number"])
	}
}
