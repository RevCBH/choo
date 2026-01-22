---
task: 1
status: pending
backpressure: "go build ./internal/container/..."
depends_on: []
---

# Core Container Types

**Parent spec**: `/specs/CONTAINER-MANAGER.md`
**Task**: #1 of 3 in implementation plan

## Objective

Define the ContainerID type, ContainerConfig struct, and Manager interface for container lifecycle management.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None

### Package Dependencies
- None (standard library only)

## Deliverables

### Files to Create/Modify

```
internal/
└── container/
    ├── container.go    # CREATE: ContainerID and ContainerConfig
    └── manager.go      # CREATE: Manager interface
```

### Types to Implement

#### container.go

```go
package container

// ContainerID is a unique identifier for a container.
// This is the full container ID returned by `docker create`, not the short form.
type ContainerID string

// ContainerConfig specifies container creation parameters.
type ContainerConfig struct {
    // Image is the container image (e.g., "choo:latest")
    Image string

    // Name is the container name (e.g., "choo-job-abc123")
    Name string

    // Env contains environment variables to set in the container
    Env map[string]string

    // Cmd is the command and arguments to run
    Cmd []string

    // WorkDir is the working directory inside the container
    WorkDir string
}
```

#### manager.go

```go
package container

import (
    "context"
    "io"
    "time"
)

// Manager provides container lifecycle management.
// Implementations must be safe for concurrent use.
type Manager interface {
    // Create creates a new container but does not start it.
    // Returns the container ID on success.
    Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error)

    // Start starts a previously created container.
    Start(ctx context.Context, id ContainerID) error

    // Wait blocks until the container exits and returns the exit code.
    // Returns an error if the container doesn't exist or wait fails.
    Wait(ctx context.Context, id ContainerID) (exitCode int, err error)

    // Logs returns a stream of container logs (stdout and stderr combined).
    // The caller must close the returned ReadCloser.
    Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error)

    // Stop stops a running container. Sends SIGTERM, waits for timeout,
    // then sends SIGKILL if still running.
    Stop(ctx context.Context, id ContainerID, timeout time.Duration) error

    // Remove removes a container. The container must be stopped first.
    Remove(ctx context.Context, id ContainerID) error
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/container/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build | Package compiles without errors |
| Interface completeness | Manager interface has all 6 methods |
| Type usage | ContainerConfig fields are accessible |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- ContainerID is a type alias for string to provide type safety
- Manager interface is designed for both Docker and Podman CLI implementations
- ContainerConfig uses simple types (no pointers) since all fields are optional or have zero values
- The Env map uses string keys and values matching environment variable semantics

## NOT In Scope

- DetectRuntime function (Task #2)
- CLIManager implementation (Task #3)
- Any CLI execution or subprocess handling
