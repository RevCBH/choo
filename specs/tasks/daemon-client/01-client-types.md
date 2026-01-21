---
task: 1
status: complete
backpressure: "go build ./internal/client/..."
depends_on: []
---

# Client Types

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #1 of 6 in implementation plan

## Objective

Define the Client struct and all client-side type definitions that decouple CLI code from protobuf details.

## Dependencies

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `google.golang.org/grpc` - gRPC client connection
- `internal/api/v1` - generated protobuf service client (assumed to exist from daemon-grpc)

## Deliverables

### Files to Create/Modify

```
internal/client/
├── client.go    # CREATE: Client struct definition
└── types.go     # CREATE: Client-side type definitions
```

### Types to Implement

```go
// client.go

// Client wraps gRPC connection and service stub for daemon communication
type Client struct {
    conn   *grpc.ClientConn
    daemon apiv1.DaemonServiceClient
}
```

```go
// types.go

// JobConfig contains parameters for starting a new job
type JobConfig struct {
    TasksDir      string    // Directory containing task definitions
    TargetBranch  string    // Base branch for PRs
    FeatureBranch string    // Branch name for work
    Parallelism   int       // Max concurrent units
    RepoPath      string    // Repository root path
}

// JobSummary provides high-level job information for listings
type JobSummary struct {
    JobID         string
    FeatureBranch string
    Status        string
    StartedAt     time.Time
    UnitsComplete int
    UnitsTotal    int
}

// JobStatus provides detailed job state including all units
type JobStatus struct {
    JobID       string
    Status      string
    StartedAt   time.Time
    CompletedAt *time.Time
    Error       string
    Units       []UnitStatus
}

// UnitStatus tracks individual unit progress
type UnitStatus struct {
    UnitID        string
    Status        string
    TasksComplete int
    TasksTotal    int
    PRNumber      int
}

// HealthInfo contains daemon health check response
type HealthInfo struct {
    Healthy    bool
    ActiveJobs int
    Version    string
}
```

### Functions to Implement

None in this task - types only.

## Backpressure

### Validation Command

```bash
go build ./internal/client/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| Types | All types are exported and documented |

## NOT In Scope

- Connection logic (Task #3)
- Protobuf conversion functions (Task #2)
- Any method implementations
