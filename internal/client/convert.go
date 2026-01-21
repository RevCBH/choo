package client

import (
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
)

// jobConfigToProto converts client JobConfig to protobuf StartJobRequest
func jobConfigToProto(cfg JobConfig) *apiv1.StartJobRequest {
	return &apiv1.StartJobRequest{
		TasksDir:      cfg.TasksDir,
		TargetBranch:  cfg.TargetBranch,
		FeatureBranch: cfg.FeatureBranch,
		Parallelism:   int32(cfg.Parallelism),
		RepoPath:      cfg.RepoPath,
	}
}

// protoToJobSummary converts a single protobuf JobSummary to client type
func protoToJobSummary(p *apiv1.JobSummary) *JobSummary {
	return &JobSummary{
		JobID:         p.GetJobId(),
		FeatureBranch: p.GetFeatureBranch(),
		Status:        p.GetStatus(),
		StartedAt:     p.GetStartedAt().AsTime(),
		UnitsComplete: int(p.GetUnitsComplete()),
		UnitsTotal:    int(p.GetUnitsTotal()),
	}
}

// protoToJobSummaries converts a slice of protobuf JobSummary to client types
func protoToJobSummaries(protos []*apiv1.JobSummary) []*JobSummary {
	if len(protos) == 0 {
		return []*JobSummary{}
	}
	result := make([]*JobSummary, len(protos))
	for i, proto := range protos {
		result[i] = protoToJobSummary(proto)
	}
	return result
}

// protoToJobStatus converts GetJobStatusResponse to client JobStatus
func protoToJobStatus(resp *apiv1.GetJobStatusResponse) *JobStatus {
	var completedAt *time.Time
	if resp.GetCompletedAt() != nil {
		t := resp.GetCompletedAt().AsTime()
		completedAt = &t
	}

	units := make([]UnitStatus, len(resp.GetUnits()))
	for i, u := range resp.GetUnits() {
		units[i] = protoToUnitStatus(u)
	}

	return &JobStatus{
		JobID:       resp.GetJobId(),
		Status:      resp.GetStatus(),
		StartedAt:   resp.GetStartedAt().AsTime(),
		CompletedAt: completedAt,
		Error:       resp.GetError(),
		Units:       units,
	}
}

// protoToUnitStatus converts a single protobuf UnitStatus to client type
func protoToUnitStatus(u *apiv1.UnitStatus) UnitStatus {
	return UnitStatus{
		UnitID:        u.GetUnitId(),
		Status:        u.GetStatus(),
		TasksComplete: int(u.GetTasksComplete()),
		TasksTotal:    int(u.GetTasksTotal()),
		PRNumber:      int(u.GetPrNumber()),
	}
}

// protoToHealthInfo converts HealthResponse to client HealthInfo
func protoToHealthInfo(resp *apiv1.HealthResponse) *HealthInfo {
	return &HealthInfo{
		Healthy:    resp.GetHealthy(),
		ActiveJobs: int(resp.GetActiveJobs()),
		Version:    resp.GetVersion(),
	}
}
