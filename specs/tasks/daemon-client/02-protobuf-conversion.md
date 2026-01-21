---
task: 2
status: complete
backpressure: "go test ./internal/client/... -run TestProto"
depends_on: [1]
---

# Protobuf Conversion

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #2 of 6 in implementation plan

## Objective

Implement bidirectional conversion functions between client-side types and protobuf messages.

## Dependencies

### Task Dependencies (within this unit)
- Task #1 must be complete (defines client-side types)

### Package Dependencies
- `internal/api/v1` - generated protobuf types
- `google.golang.org/protobuf/types/known/timestamppb` - timestamp conversion

## Deliverables

### Files to Create/Modify

```
internal/client/
└── convert.go       # CREATE: Protobuf conversion utilities
    convert_test.go  # CREATE: Unit tests for conversion functions
```

### Types to Implement

None (uses types from Task #1).

### Functions to Implement

```go
// convert.go

// jobConfigToProto converts client JobConfig to protobuf StartJobRequest
func jobConfigToProto(cfg JobConfig) *apiv1.StartJobRequest {
    // Map all fields from JobConfig to StartJobRequest
}

// protoToJobSummary converts a single protobuf JobSummary to client type
func protoToJobSummary(p *apiv1.JobSummary) *JobSummary {
    // Handle time conversion via AsTime()
}

// protoToJobSummaries converts a slice of protobuf JobSummary to client types
func protoToJobSummaries(protos []*apiv1.JobSummary) []*JobSummary {
    // Iterate and convert each summary
}

// protoToJobStatus converts GetJobStatusResponse to client JobStatus
func protoToJobStatus(resp *apiv1.GetJobStatusResponse) *JobStatus {
    // Handle optional CompletedAt field (may be nil)
    // Convert Units slice
}

// protoToUnitStatus converts a single protobuf UnitStatus to client type
func protoToUnitStatus(u *apiv1.UnitStatus) UnitStatus {
    // Map all unit status fields
}

// protoToHealthInfo converts HealthResponse to client HealthInfo
func protoToHealthInfo(resp *apiv1.HealthResponse) *HealthInfo {
    // Map health check fields
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/client/... -run TestProto
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestProtoToJobSummary` | All fields correctly mapped including time conversion |
| `TestProtoToJobSummaries` | Empty slice returns empty slice, not nil |
| `TestProtoToJobStatus_NilCompletedAt` | Nil CompletedAt handled correctly |
| `TestProtoToJobStatus_WithCompletedAt` | Non-nil CompletedAt converted correctly |
| `TestProtoToUnitStatus` | All unit fields mapped correctly |
| `TestJobConfigToProto` | All config fields mapped to request |

## NOT In Scope

- Event conversion (handled in Task #5 with streaming)
- Error type conversion (gRPC errors propagate directly)
- Any method implementations that use these converters
