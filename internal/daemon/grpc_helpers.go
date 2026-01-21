package daemon

import (
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// timeToProto converts a Go time.Time pointer to protobuf Timestamp
func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// unitStateToProto converts internal UnitState to protobuf UnitStatus
func unitStateToProto(u *UnitState) *apiv1.UnitStatus {
	return &apiv1.UnitStatus{
		UnitId:        u.UnitID,
		Status:        u.Status,
		TasksComplete: int32(u.TasksComplete),
		TasksTotal:    int32(u.TasksTotal),
		PrNumber:      int32(u.PRNumber),
	}
}

// jobStateToProto converts internal JobState to protobuf GetJobStatusResponse
func jobStateToProto(j *JobState) *apiv1.GetJobStatusResponse {
	resp := &apiv1.GetJobStatusResponse{
		JobId:       j.ID,
		Status:      j.Status,
		StartedAt:   timeToProto(j.StartedAt),
		CompletedAt: timeToProto(j.CompletedAt),
	}
	if j.Error != nil {
		resp.Error = *j.Error
	}
	for _, u := range j.Units {
		resp.Units = append(resp.Units, unitStateToProto(u))
	}
	return resp
}

// jobSummaryToProto converts internal JobSummary to protobuf JobSummary
func jobSummaryToProto(j *JobSummary) *apiv1.JobSummary {
	return &apiv1.JobSummary{
		JobId:         j.JobID,
		FeatureBranch: j.FeatureBranch,
		Status:        j.Status,
		StartedAt:     timeToProto(j.StartedAt),
		UnitsComplete: int32(j.UnitsComplete),
		UnitsTotal:    int32(j.UnitsTotal),
	}
}

// eventToProto converts internal Event to protobuf JobEvent
func eventToProto(e Event) *apiv1.JobEvent {
	return &apiv1.JobEvent{
		Sequence:    int32(e.Sequence),
		EventType:   e.EventType,
		UnitId:      e.UnitID,
		PayloadJson: e.PayloadJSON,
		Timestamp:   timestamppb.New(e.Timestamp),
	}
}
