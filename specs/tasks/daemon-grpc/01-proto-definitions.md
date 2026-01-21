---
task: 1
status: pending
backpressure: "go build ./pkg/api/v1/..."
depends_on: []
---

# Protocol Buffer Definitions

**Parent spec**: `specs/DAEMON-GRPC.md`
**Task**: #1 of 7 in implementation plan

## Objective

Define the Protocol Buffer service and message types for the DaemonService gRPC interface, and generate Go code.

## Dependencies

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `google.golang.org/protobuf` - Protocol Buffers runtime
- `google.golang.org/grpc` - gRPC framework
- `protoc` - Protocol buffer compiler (build tool)
- `protoc-gen-go` - Go code generator
- `protoc-gen-go-grpc` - gRPC code generator

## Deliverables

### Files to Create/Modify

```
proto/choo/v1/
└── daemon.proto           # CREATE: Service and message definitions

pkg/api/v1/
├── daemon.pb.go           # GENERATED: Message types
└── daemon_grpc.pb.go      # GENERATED: Service stubs

Makefile                   # MODIFY: Add proto generation target
```

### Types to Implement

```protobuf
// proto/choo/v1/daemon.proto

syntax = "proto3";

package choo.v1;

option go_package = "github.com/RevCBH/choo/pkg/api/v1;apiv1";

import "google/protobuf/timestamp.proto";

// DaemonService provides the gRPC interface for daemon communication
service DaemonService {
    // Job lifecycle
    rpc StartJob(StartJobRequest) returns (StartJobResponse);
    rpc StopJob(StopJobRequest) returns (StopJobResponse);
    rpc GetJobStatus(GetJobStatusRequest) returns (GetJobStatusResponse);
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);

    // Event streaming
    rpc WatchJob(WatchJobRequest) returns (stream JobEvent);

    // Daemon lifecycle
    rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
    rpc Health(HealthRequest) returns (HealthResponse);
}

// StartJob creates and starts a new job
message StartJobRequest {
    string tasks_dir = 1;       // Path to directory containing task YAML files
    string target_branch = 2;   // Base branch for PRs (e.g., "main")
    string feature_branch = 3;  // Optional: for feature mode, omit for PR mode
    int32 parallelism = 4;      // Max concurrent units (0 = default from config)
    string repo_path = 5;       // Absolute path to git repository
}

message StartJobResponse {
    string job_id = 1;          // Unique identifier for the created job
    string status = 2;          // Initial status, typically "running"
}

// StopJob gracefully stops a running job
message StopJobRequest {
    string job_id = 1;
    bool force = 2;             // If true, kill immediately without waiting
}

message StopJobResponse {
    bool success = 1;
    string message = 2;         // Human-readable result description
}

// GetJobStatus returns current status of a job
message GetJobStatusRequest {
    string job_id = 1;
}

message GetJobStatusResponse {
    string job_id = 1;
    string status = 2;                          // "pending", "running", "completed", "failed"
    google.protobuf.Timestamp started_at = 3;
    google.protobuf.Timestamp completed_at = 4; // Zero if still running
    string error = 5;                           // Error message if failed
    repeated UnitStatus units = 6;              // Status of each execution unit
}

message UnitStatus {
    string unit_id = 1;
    string status = 2;          // "pending", "running", "completed", "failed"
    int32 tasks_complete = 3;
    int32 tasks_total = 4;
    int32 pr_number = 5;        // GitHub PR number, 0 if not yet created
}

// ListJobs returns all jobs, optionally filtered by status
message ListJobsRequest {
    repeated string status_filter = 1;  // Empty = all statuses
}

message ListJobsResponse {
    repeated JobSummary jobs = 1;
}

message JobSummary {
    string job_id = 1;
    string feature_branch = 2;              // Empty for PR mode jobs
    string status = 3;
    google.protobuf.Timestamp started_at = 4;
    int32 units_complete = 5;
    int32 units_total = 6;
}

// WatchJob streams events for a running job
message WatchJobRequest {
    string job_id = 1;
    int32 from_sequence = 2;    // Resume from sequence number (0 = beginning)
}

message JobEvent {
    int32 sequence = 1;         // Monotonically increasing per job
    string event_type = 2;      // "unit_started", "task_completed", etc.
    string unit_id = 3;         // Which unit this event relates to
    string payload_json = 4;    // Event-specific data as JSON
    google.protobuf.Timestamp timestamp = 5;
}

// Shutdown gracefully shuts down the daemon
message ShutdownRequest {
    bool wait_for_jobs = 1;     // If true, wait for running jobs to complete
    int32 timeout_seconds = 2;  // Max wait time before force shutdown
}

message ShutdownResponse {
    bool success = 1;
    int32 jobs_stopped = 2;     // Number of jobs that were interrupted
}

// Health check
message HealthRequest {}

message HealthResponse {
    bool healthy = 1;
    int32 active_jobs = 2;
    string version = 3;         // Daemon version string
}
```

### Makefile Target

```makefile
.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/choo/v1/daemon.proto
	mv proto/choo/v1/*.go pkg/api/v1/
```

## Backpressure

### Validation Command

```bash
go build ./pkg/api/v1/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Package compiles | No build errors |
| Types accessible | `apiv1.StartJobRequest`, `apiv1.DaemonServiceServer` etc. |
| Generated stubs | `daemon.pb.go` and `daemon_grpc.pb.go` exist |

## NOT In Scope

- Server implementation (task #2-#6)
- Event streaming logic (task #4)
- Socket setup (task #6)
- Tests (task #7)
