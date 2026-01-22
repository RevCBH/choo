---
task: 1
status: complete
backpressure: "go build ./internal/daemon/..."
depends_on: []
---

# Container Config Types

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #1 of 6 in implementation plan

## Objective

Extend the daemon Config struct with container-specific fields and define ContainerJobConfig, ContainerJobState, and ContainerStatus types.

## Dependencies

### External Specs (must be implemented)
- CONTAINER-MANAGER - provides `container.ContainerID` type

### Task Dependencies (within this unit)
- None

### Package Dependencies
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── config.go          # MODIFY: Add container mode fields to Config
└── container_types.go # CREATE: Container job types
```

### Types to Implement

```go
// internal/daemon/config.go - Add to existing Config struct

// Config holds daemon configuration with sensible defaults.
type Config struct {
    // Existing fields
    SocketPath    string
    PIDFile       string
    DBPath        string
    MaxJobs       int
    WebAddr       string
    WebSocketPath string

    // Container mode configuration (new fields)
    ContainerMode    bool   // Enable container isolation for job execution
    ContainerImage   string // Container image to use, e.g., "choo:latest"
    ContainerRuntime string // "auto", "docker", or "podman"
}
```

```go
// internal/daemon/container_types.go

package daemon

import "time"

// ContainerJobConfig extends job configuration with container-specific settings.
type ContainerJobConfig struct {
    // JobID is the unique identifier for this job
    JobID string

    // RepoPath is the local repository path (for reference)
    RepoPath string

    // GitURL is the repository URL to clone inside the container
    GitURL string

    // TasksDir is the path to specs/tasks/ relative to repo root
    TasksDir string

    // TargetBranch is the branch PRs will target
    TargetBranch string

    // Unit is the specific unit to run (optional)
    Unit string
}

// ContainerJobState tracks the container lifecycle for a job.
type ContainerJobState struct {
    // ContainerID is the container identifier from the runtime
    ContainerID string

    // ContainerName is the human-readable name: choo-<job-id>
    ContainerName string

    // Status is the current container lifecycle state
    Status ContainerStatus

    // ExitCode is the container exit code when stopped (nil if still running)
    ExitCode *int

    // StartedAt is when the container started running
    StartedAt *time.Time

    // StoppedAt is when the container stopped
    StoppedAt *time.Time

    // Error is the error message if the container failed
    Error string
}

// ContainerStatus represents container lifecycle states.
type ContainerStatus string

const (
    // ContainerStatusCreating indicates the container is being created
    ContainerStatusCreating ContainerStatus = "creating"

    // ContainerStatusRunning indicates the container is executing
    ContainerStatusRunning ContainerStatus = "running"

    // ContainerStatusStopped indicates the container exited normally
    ContainerStatusStopped ContainerStatus = "stopped"

    // ContainerStatusFailed indicates the container failed
    ContainerStatusFailed ContainerStatus = "failed"
)

// IsTerminal returns true if this status is a terminal state.
func (s ContainerStatus) IsTerminal() bool {
    return s == ContainerStatusStopped || s == ContainerStatusFailed
}

// String returns the string representation of the status.
func (s ContainerStatus) String() string {
    return string(s)
}
```

### Functions to Implement

```go
// internal/daemon/config.go - Add validation for container fields

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
    // ... existing validation ...

    // Container mode validation
    if c.ContainerMode {
        if c.ContainerImage == "" {
            return fmt.Errorf("ContainerImage is required when ContainerMode is enabled")
        }
        if c.ContainerRuntime != "" && c.ContainerRuntime != "auto" &&
           c.ContainerRuntime != "docker" && c.ContainerRuntime != "podman" {
            return fmt.Errorf("ContainerRuntime must be 'auto', 'docker', or 'podman', got %s", c.ContainerRuntime)
        }
    }

    return nil
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/daemon/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| Type completeness | ContainerStatus constants are defined |
| Validation | ContainerMode validation is enforced |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ContainerRuntime defaults to "auto" if not specified
- ContainerImage has no default - must be explicitly provided when container mode is enabled
- ContainerStatus uses string constants for easy JSON serialization
- ContainerJobState uses pointers for optional time fields

## NOT In Scope

- Container creation logic (Task #3)
- Log streaming (Task #2)
- CLI flag parsing (Task #5)
- DefaultConfig changes for container fields (defaults are zero values)
