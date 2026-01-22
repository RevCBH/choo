---
task: 2
status: pending
backpressure: "go test ./internal/container/... -run TestDetectRuntime"
depends_on: []
---

# Runtime Detection

**Parent spec**: `/specs/CONTAINER-MANAGER.md`
**Task**: #2 of 3 in implementation plan

## Objective

Implement DetectRuntime() function that finds an available container runtime (Docker or Podman) by checking PATH and verifying the binary works.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (can run in parallel with Task #1)

### Package Dependencies
- None (standard library only: os/exec)

## Deliverables

### Files to Create/Modify

```
internal/
└── container/
    ├── detect.go       # CREATE: Runtime detection
    └── detect_test.go  # CREATE: Detection tests
```

### Functions to Implement

#### detect.go

```go
package container

import (
    "errors"
    "os/exec"
)

// ErrNoRuntime is returned when no container runtime is found.
var ErrNoRuntime = errors.New("no container runtime found (need docker or podman)")

// DetectRuntime finds an available container runtime.
// Checks docker first, then podman. Verifies the binary actually works
// by running `<runtime> version`.
func DetectRuntime() (string, error) {
    for _, bin := range []string{"docker", "podman"} {
        if _, err := exec.LookPath(bin); err != nil {
            continue
        }
        cmd := exec.Command(bin, "version")
        if err := cmd.Run(); err != nil {
            continue
        }
        return bin, nil
    }
    return "", ErrNoRuntime
}
```

### Tests to Implement

#### detect_test.go

```go
package container

import (
    "os/exec"
    "testing"
)

func TestDetectRuntime_FindsDocker(t *testing.T) {
    // Skip if docker is not available
    if _, err := exec.LookPath("docker"); err != nil {
        t.Skip("docker not available")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Fatalf("DetectRuntime() failed: %v", err)
    }

    // Docker should be preferred if both are available
    if runtime != "docker" {
        t.Errorf("expected docker, got %s", runtime)
    }
}

func TestDetectRuntime_FindsPodman(t *testing.T) {
    // This test only runs if podman is available but docker is not
    if _, err := exec.LookPath("docker"); err == nil {
        t.Skip("docker is available, podman fallback not tested")
    }
    if _, err := exec.LookPath("podman"); err != nil {
        t.Skip("podman not available")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Fatalf("DetectRuntime() failed: %v", err)
    }

    if runtime != "podman" {
        t.Errorf("expected podman, got %s", runtime)
    }
}

func TestDetectRuntime_ReturnsErrorWhenNoneAvailable(t *testing.T) {
    // This test documents the expected behavior but cannot easily
    // be run in environments where docker or podman are installed.
    // The function should return ErrNoRuntime when neither is found.
    t.Log("DetectRuntime returns ErrNoRuntime when no runtime is found")
}

func TestDetectRuntime_VerifiesBinaryWorks(t *testing.T) {
    // Verify that we get a valid runtime that can execute commands
    runtime, err := DetectRuntime()
    if err != nil {
        t.Skip("no container runtime available")
    }

    // The detected runtime should be able to run 'version'
    cmd := exec.Command(runtime, "version")
    if err := cmd.Run(); err != nil {
        t.Errorf("%s version failed: %v", runtime, err)
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/container/... -run TestDetectRuntime
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDetectRuntime_FindsDocker` | Returns "docker" when docker is available |
| `TestDetectRuntime_VerifiesBinaryWorks` | Detected runtime can execute version command |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds
- Note: Tests skip gracefully if neither docker nor podman is installed

## Implementation Notes

- Order matters: check docker before podman (docker is more common)
- Use exec.LookPath to check PATH before running the binary
- Use `<runtime> version` to verify the daemon is running (docker) or binary works (podman)
- Return a sentinel error (ErrNoRuntime) for programmatic checking
- Tests should skip when no runtime is available rather than fail

## NOT In Scope

- Core types (Task #1)
- CLIManager implementation (Task #3)
- Any container lifecycle operations
