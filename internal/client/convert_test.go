package client

import (
	"testing"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestJobConfigToProto(t *testing.T) {
	cfg := JobConfig{
		TasksDir:      "/path/to/tasks",
		TargetBranch:  "main",
		FeatureBranch: "feature-branch",
		Parallelism:   4,
		RepoPath:      "/path/to/repo",
	}

	result := jobConfigToProto(cfg)

	if result.TasksDir != cfg.TasksDir {
		t.Errorf("TasksDir: got %s, want %s", result.TasksDir, cfg.TasksDir)
	}
	if result.TargetBranch != cfg.TargetBranch {
		t.Errorf("TargetBranch: got %s, want %s", result.TargetBranch, cfg.TargetBranch)
	}
	if result.FeatureBranch != cfg.FeatureBranch {
		t.Errorf("FeatureBranch: got %s, want %s", result.FeatureBranch, cfg.FeatureBranch)
	}
	if result.Parallelism != int32(cfg.Parallelism) {
		t.Errorf("Parallelism: got %d, want %d", result.Parallelism, cfg.Parallelism)
	}
	if result.RepoPath != cfg.RepoPath {
		t.Errorf("RepoPath: got %s, want %s", result.RepoPath, cfg.RepoPath)
	}
}

func TestProtoToJobSummary(t *testing.T) {
	now := time.Now()
	proto := &apiv1.JobSummary{
		JobId:         "job-123",
		FeatureBranch: "feature-test",
		Status:        "running",
		StartedAt:     timestamppb.New(now),
		UnitsComplete: 3,
		UnitsTotal:    10,
	}

	result := protoToJobSummary(proto)

	if result.JobID != proto.JobId {
		t.Errorf("JobID: got %s, want %s", result.JobID, proto.JobId)
	}
	if result.FeatureBranch != proto.FeatureBranch {
		t.Errorf("FeatureBranch: got %s, want %s", result.FeatureBranch, proto.FeatureBranch)
	}
	if result.Status != proto.Status {
		t.Errorf("Status: got %s, want %s", result.Status, proto.Status)
	}
	if !result.StartedAt.Equal(now) {
		t.Errorf("StartedAt: got %v, want %v", result.StartedAt, now)
	}
	if result.UnitsComplete != int(proto.UnitsComplete) {
		t.Errorf("UnitsComplete: got %d, want %d", result.UnitsComplete, proto.UnitsComplete)
	}
	if result.UnitsTotal != int(proto.UnitsTotal) {
		t.Errorf("UnitsTotal: got %d, want %d", result.UnitsTotal, proto.UnitsTotal)
	}
}

func TestProtoToJobSummaries(t *testing.T) {
	t.Run("empty slice returns empty slice", func(t *testing.T) {
		result := protoToJobSummaries([]*apiv1.JobSummary{})
		if result == nil {
			t.Error("Expected empty slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("Expected empty slice, got length %d", len(result))
		}
	})

	t.Run("converts multiple summaries", func(t *testing.T) {
		now := time.Now()
		protos := []*apiv1.JobSummary{
			{
				JobId:         "job-1",
				FeatureBranch: "feature-1",
				Status:        "running",
				StartedAt:     timestamppb.New(now),
				UnitsComplete: 1,
				UnitsTotal:    5,
			},
			{
				JobId:         "job-2",
				FeatureBranch: "feature-2",
				Status:        "completed",
				StartedAt:     timestamppb.New(now),
				UnitsComplete: 5,
				UnitsTotal:    5,
			},
		}

		result := protoToJobSummaries(protos)

		if len(result) != 2 {
			t.Fatalf("Expected 2 summaries, got %d", len(result))
		}
		if result[0].JobID != "job-1" {
			t.Errorf("First job ID: got %s, want job-1", result[0].JobID)
		}
		if result[1].JobID != "job-2" {
			t.Errorf("Second job ID: got %s, want job-2", result[1].JobID)
		}
	})
}

func TestProtoToJobStatus_NilCompletedAt(t *testing.T) {
	now := time.Now()
	resp := &apiv1.GetJobStatusResponse{
		JobId:       "job-456",
		Status:      "running",
		StartedAt:   timestamppb.New(now),
		CompletedAt: nil,
		Error:       "",
		Units: []*apiv1.UnitStatus{
			{
				UnitId:        "unit-1",
				Status:        "running",
				TasksComplete: 2,
				TasksTotal:    5,
				PrNumber:      0,
			},
		},
	}

	result := protoToJobStatus(resp)

	if result.JobID != resp.JobId {
		t.Errorf("JobID: got %s, want %s", result.JobID, resp.JobId)
	}
	if result.Status != resp.Status {
		t.Errorf("Status: got %s, want %s", result.Status, resp.Status)
	}
	if !result.StartedAt.Equal(now) {
		t.Errorf("StartedAt: got %v, want %v", result.StartedAt, now)
	}
	if result.CompletedAt != nil {
		t.Errorf("CompletedAt: got %v, want nil", result.CompletedAt)
	}
	if result.Error != resp.Error {
		t.Errorf("Error: got %s, want %s", result.Error, resp.Error)
	}
	if len(result.Units) != 1 {
		t.Fatalf("Expected 1 unit, got %d", len(result.Units))
	}
	if result.Units[0].UnitID != "unit-1" {
		t.Errorf("Unit ID: got %s, want unit-1", result.Units[0].UnitID)
	}
}

func TestProtoToJobStatus_WithCompletedAt(t *testing.T) {
	now := time.Now()
	completedTime := now.Add(5 * time.Minute)
	resp := &apiv1.GetJobStatusResponse{
		JobId:       "job-789",
		Status:      "completed",
		StartedAt:   timestamppb.New(now),
		CompletedAt: timestamppb.New(completedTime),
		Error:       "",
		Units: []*apiv1.UnitStatus{
			{
				UnitId:        "unit-1",
				Status:        "completed",
				TasksComplete: 5,
				TasksTotal:    5,
				PrNumber:      123,
			},
		},
	}

	result := protoToJobStatus(resp)

	if result.CompletedAt == nil {
		t.Fatal("CompletedAt: got nil, expected non-nil")
	}
	if !result.CompletedAt.Equal(completedTime) {
		t.Errorf("CompletedAt: got %v, want %v", *result.CompletedAt, completedTime)
	}
}

func TestProtoToUnitStatus(t *testing.T) {
	proto := &apiv1.UnitStatus{
		UnitId:        "unit-xyz",
		Status:        "completed",
		TasksComplete: 8,
		TasksTotal:    8,
		PrNumber:      456,
	}

	result := protoToUnitStatus(proto)

	if result.UnitID != proto.UnitId {
		t.Errorf("UnitID: got %s, want %s", result.UnitID, proto.UnitId)
	}
	if result.Status != proto.Status {
		t.Errorf("Status: got %s, want %s", result.Status, proto.Status)
	}
	if result.TasksComplete != int(proto.TasksComplete) {
		t.Errorf("TasksComplete: got %d, want %d", result.TasksComplete, proto.TasksComplete)
	}
	if result.TasksTotal != int(proto.TasksTotal) {
		t.Errorf("TasksTotal: got %d, want %d", result.TasksTotal, proto.TasksTotal)
	}
	if result.PRNumber != int(proto.PrNumber) {
		t.Errorf("PRNumber: got %d, want %d", result.PRNumber, proto.PrNumber)
	}
}

func TestProtoToHealthInfo(t *testing.T) {
	resp := &apiv1.HealthResponse{
		Healthy:    true,
		ActiveJobs: 3,
		Version:    "1.0.0",
	}

	result := protoToHealthInfo(resp)

	if result.Healthy != resp.Healthy {
		t.Errorf("Healthy: got %v, want %v", result.Healthy, resp.Healthy)
	}
	if result.ActiveJobs != int(resp.ActiveJobs) {
		t.Errorf("ActiveJobs: got %d, want %d", result.ActiveJobs, resp.ActiveJobs)
	}
	if result.Version != resp.Version {
		t.Errorf("Version: got %s, want %s", result.Version, resp.Version)
	}
}
