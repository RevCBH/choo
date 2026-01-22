---
task: 3
status: complete
backpressure: "go test ./internal/container/... -run TestCLIManager"
depends_on: [1, 2]
---

# CLI Manager Implementation

**Parent spec**: `/specs/CONTAINER-MANAGER.md`
**Task**: #3 of 3 in implementation plan

## Objective

Implement CLIManager struct with all Manager interface methods (Create, Start, Wait, Logs, Stop, Remove) using docker/podman CLI commands.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: ContainerID, ContainerConfig, Manager interface)
- Task #2 must be complete (provides: DetectRuntime for tests)

### Package Dependencies
- None (standard library only: context, errors, fmt, io, os/exec, strconv, strings, time)

## Deliverables

### Files to Create/Modify

```
internal/
└── container/
    ├── cli.go          # CREATE: CLIManager implementation
    └── cli_test.go     # CREATE: CLIManager tests
```

### Types and Functions to Implement

#### cli.go

```go
package container

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

// CLIManager implements Manager using docker/podman CLI.
type CLIManager struct {
    runtime string // "docker" or "podman"
}

// NewCLIManager creates a Manager using the specified runtime.
// Use DetectRuntime() to find an available runtime first.
func NewCLIManager(runtime string) *CLIManager {
    return &CLIManager{runtime: runtime}
}

// Create creates a new container but does not start it.
func (m *CLIManager) Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error) {
    args := []string{"create", "--name", cfg.Name}

    // Add environment variables
    for k, v := range cfg.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }

    // Set working directory if specified
    if cfg.WorkDir != "" {
        args = append(args, "-w", cfg.WorkDir)
    }

    // Image and command come last
    args = append(args, cfg.Image)
    args = append(args, cfg.Cmd...)

    cmd := exec.CommandContext(ctx, m.runtime, args...)
    output, err := cmd.Output()
    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return "", fmt.Errorf("failed to create container: %s", exitErr.Stderr)
        }
        return "", fmt.Errorf("failed to create container: %w", err)
    }

    return ContainerID(strings.TrimSpace(string(output))), nil
}

// Start starts a previously created container.
func (m *CLIManager) Start(ctx context.Context, id ContainerID) error {
    cmd := exec.CommandContext(ctx, m.runtime, "start", string(id))

    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to start container: %s", output)
    }

    return nil
}

// Wait blocks until the container exits and returns the exit code.
func (m *CLIManager) Wait(ctx context.Context, id ContainerID) (int, error) {
    cmd := exec.CommandContext(ctx, m.runtime, "wait", string(id))
    output, err := cmd.Output()
    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return -1, fmt.Errorf("failed to wait for container: %s", exitErr.Stderr)
        }
        return -1, fmt.Errorf("failed to wait for container: %w", err)
    }

    exitCode, err := strconv.Atoi(strings.TrimSpace(string(output)))
    if err != nil {
        return -1, fmt.Errorf("failed to parse exit code: %w", err)
    }

    return exitCode, nil
}

// Logs returns a stream of container logs (stdout and stderr combined).
func (m *CLIManager) Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error) {
    // -f follows the log output until container exits
    cmd := exec.CommandContext(ctx, m.runtime, "logs", "-f", string(id))

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start log streaming: %w", err)
    }

    // Return the pipe; caller is responsible for closing
    // When ctx is canceled, the command will be killed and pipe will close
    return stdout, nil
}

// Stop stops a running container with the specified timeout.
func (m *CLIManager) Stop(ctx context.Context, id ContainerID, timeout time.Duration) error {
    timeoutSecs := int(timeout.Seconds())
    cmd := exec.CommandContext(ctx, m.runtime, "stop", "-t", strconv.Itoa(timeoutSecs), string(id))

    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to stop container: %s", output)
    }

    return nil
}

// Remove removes a stopped container.
func (m *CLIManager) Remove(ctx context.Context, id ContainerID) error {
    cmd := exec.CommandContext(ctx, m.runtime, "rm", string(id))

    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to remove container: %s", output)
    }

    return nil
}

// Verify CLIManager implements Manager interface
var _ Manager = (*CLIManager)(nil)
```

### Tests to Implement

#### cli_test.go

```go
package container

import (
    "context"
    "fmt"
    "io"
    "strings"
    "testing"
    "time"
)

func TestCLIManager_ImplementsManagerInterface(t *testing.T) {
    var _ Manager = (*CLIManager)(nil)
}

func TestCLIManager_NewCLIManager(t *testing.T) {
    mgr := NewCLIManager("docker")
    if mgr == nil {
        t.Fatal("NewCLIManager returned nil")
    }
    if mgr.runtime != "docker" {
        t.Errorf("expected runtime docker, got %s", mgr.runtime)
    }
}

func TestCLIManager_FullLifecycle(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Skip("no container runtime available")
    }

    mgr := NewCLIManager(runtime)
    ctx := context.Background()

    cfg := ContainerConfig{
        Image: "alpine:latest",
        Name:  fmt.Sprintf("test-%d", time.Now().UnixNano()),
        Cmd:   []string{"sh", "-c", "echo hello && exit 42"},
    }

    // Create
    id, err := mgr.Create(ctx, cfg)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }
    t.Cleanup(func() {
        mgr.Remove(context.Background(), id)
    })

    if id == "" {
        t.Error("Create returned empty container ID")
    }

    // Start
    if err := mgr.Start(ctx, id); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Wait
    exitCode, err := mgr.Wait(ctx, id)
    if err != nil {
        t.Fatalf("Wait failed: %v", err)
    }
    if exitCode != 42 {
        t.Errorf("expected exit code 42, got %d", exitCode)
    }
}

func TestCLIManager_LogStreaming(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Skip("no container runtime available")
    }

    mgr := NewCLIManager(runtime)
    ctx := context.Background()

    cfg := ContainerConfig{
        Image: "alpine:latest",
        Name:  fmt.Sprintf("test-logs-%d", time.Now().UnixNano()),
        Cmd:   []string{"sh", "-c", "echo line1 && echo line2 && echo line3"},
    }

    id, err := mgr.Create(ctx, cfg)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }
    t.Cleanup(func() {
        mgr.Remove(context.Background(), id)
    })

    if err := mgr.Start(ctx, id); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Wait for container to finish first
    mgr.Wait(ctx, id)

    // Now get logs
    logs, err := mgr.Logs(ctx, id)
    if err != nil {
        t.Fatalf("Logs failed: %v", err)
    }
    defer logs.Close()

    output, err := io.ReadAll(logs)
    if err != nil {
        t.Fatalf("failed to read logs: %v", err)
    }

    if !strings.Contains(string(output), "line1") {
        t.Error("logs missing expected output 'line1'")
    }
    if !strings.Contains(string(output), "line2") {
        t.Error("logs missing expected output 'line2'")
    }
    if !strings.Contains(string(output), "line3") {
        t.Error("logs missing expected output 'line3'")
    }
}

func TestCLIManager_CreateWithEnvAndWorkDir(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Skip("no container runtime available")
    }

    mgr := NewCLIManager(runtime)
    ctx := context.Background()

    cfg := ContainerConfig{
        Image:   "alpine:latest",
        Name:    fmt.Sprintf("test-env-%d", time.Now().UnixNano()),
        Env:     map[string]string{"TEST_VAR": "test_value"},
        WorkDir: "/tmp",
        Cmd:     []string{"sh", "-c", "echo $TEST_VAR && pwd"},
    }

    id, err := mgr.Create(ctx, cfg)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }
    t.Cleanup(func() {
        mgr.Remove(context.Background(), id)
    })

    if err := mgr.Start(ctx, id); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    mgr.Wait(ctx, id)

    logs, err := mgr.Logs(ctx, id)
    if err != nil {
        t.Fatalf("Logs failed: %v", err)
    }
    defer logs.Close()

    output, _ := io.ReadAll(logs)
    outputStr := string(output)

    if !strings.Contains(outputStr, "test_value") {
        t.Error("environment variable not set correctly")
    }
    if !strings.Contains(outputStr, "/tmp") {
        t.Error("working directory not set correctly")
    }
}

func TestCLIManager_StopContainer(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Skip("no container runtime available")
    }

    mgr := NewCLIManager(runtime)
    ctx := context.Background()

    cfg := ContainerConfig{
        Image: "alpine:latest",
        Name:  fmt.Sprintf("test-stop-%d", time.Now().UnixNano()),
        Cmd:   []string{"sleep", "300"},
    }

    id, err := mgr.Create(ctx, cfg)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }
    t.Cleanup(func() {
        mgr.Remove(context.Background(), id)
    })

    if err := mgr.Start(ctx, id); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Stop with short timeout
    if err := mgr.Stop(ctx, id, 1*time.Second); err != nil {
        t.Fatalf("Stop failed: %v", err)
    }

    // Container should now be stopped, Remove should work
    if err := mgr.Remove(ctx, id); err != nil {
        t.Errorf("Remove after stop failed: %v", err)
    }
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/container/... -run TestCLIManager
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestCLIManager_ImplementsManagerInterface` | CLIManager satisfies Manager interface |
| `TestCLIManager_NewCLIManager` | Constructor returns valid manager with runtime set |
| `TestCLIManager_FullLifecycle` | Create, Start, Wait, Remove work; exit code captured |
| `TestCLIManager_LogStreaming` | Logs returns readable output containing expected lines |
| `TestCLIManager_CreateWithEnvAndWorkDir` | Environment variables and working directory are set |
| `TestCLIManager_StopContainer` | Stop terminates running container, Remove succeeds after |

### Test Fixtures

None required - tests use alpine:latest image.

### CI Compatibility

- [x] No external API keys required
- [ ] Network access required (to pull alpine:latest if not cached)
- [x] Runs in <60 seconds
- Note: Tests skip gracefully if neither docker nor podman is installed
- Note: Tests skip in short mode (`go test -short`)

## Implementation Notes

- Always capture stderr from failed commands using exec.ExitError
- Use CombinedOutput for commands where we only care about success/failure
- Use Output (stdout only) when we need to parse the result (Create, Wait)
- The interface compile check `var _ Manager = (*CLIManager)(nil)` ensures completeness
- Map iteration order is non-deterministic for Env, but this is fine for env vars
- Context cancellation automatically kills running commands via CommandContext

## NOT In Scope

- Core types definition (Task #1)
- Runtime detection (Task #2)
- Volume mounts, resource limits, network isolation (future enhancements)
- Image pulling (future enhancement)
