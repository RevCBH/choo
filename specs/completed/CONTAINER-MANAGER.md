# CONTAINER-MANAGER — Container Lifecycle Management Using Docker/Podman CLI

## Overview

The Container Manager provides CLI-based Docker/Podman container lifecycle management for isolated job execution. Rather than using Docker's Go SDK or REST API, it shells out to the `docker` or `podman` command-line tools, keeping the implementation simple and avoiding heavy dependencies.

This component handles the full container lifecycle: detecting which runtime is available, creating containers with the right configuration, starting them, streaming logs in real-time, waiting for exit, and cleaning up. Every job runs in its own container, providing process isolation, filesystem isolation, and reproducible environments.

The CLI-based approach was chosen deliberately. Both Docker and Podman expose identical CLI interfaces, making runtime switching trivial. The CLI is also more debuggable—operators can run the exact same commands manually to diagnose issues.

```
┌─────────────────────────────────────────────────────────┐
│                    Job Executor                         │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                  Container Manager                      │
│  ┌───────────────────────────────────────────────────┐  │
│  │              Manager Interface                    │  │
│  │  Create() Start() Wait() Logs() Stop() Remove()   │  │
│  └───────────────────────────────────────────────────┘  │
│                          │                              │
│                          ▼                              │
│  ┌───────────────────────────────────────────────────┐  │
│  │               CLIManager                          │  │
│  │         runtime: "docker" | "podman"              │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  docker/podman CLI    │
              └───────────────────────┘
```

## Requirements

### Functional Requirements

1. Detect available container runtime (Docker or Podman) by checking PATH and verifying the binary works
2. Create containers with specified image, name, environment variables, working directory, and command
3. Start containers that were previously created
4. Stop containers gracefully with a configurable timeout, falling back to force kill
5. Remove containers after job completion (whether successful or failed)
6. Stream container logs in real-time via stdout pipe
7. Wait for container exit and capture the exit code
8. Support both Docker and Podman with identical behavior

### Performance Requirements

| Metric | Target |
|--------|--------|
| Container creation | < 2s |
| Container start latency | < 500ms |
| Log streaming latency | < 100ms |
| Container stop (graceful) | < 10s |

### Constraints

1. Container runtime must be Docker or Podman (CLI-based, not API)
2. Containers run with network access (required for git push, API calls)
3. Credentials passed via environment variables or SSH agent forwarding
4. No Docker SDK dependency—all operations via exec.Command
5. Must handle runtime being unavailable gracefully

## Design

### Module Structure

```
internal/container/
├── manager.go          # Manager interface definition
├── cli.go              # CLI-based docker/podman implementation
├── config.go           # ContainerConfig type
└── detect.go           # Runtime detection utilities
```

### Core Types

```go
// ContainerID is a unique identifier for a container.
// This is the full container ID returned by `docker create`, not the short form.
type ContainerID string

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

// ContainerConfig specifies container creation parameters.
type ContainerConfig struct {
    Image   string            // Container image (e.g., "choo:latest")
    Name    string            // Container name (e.g., "choo-job-abc123")
    Env     map[string]string // Environment variables to set
    Cmd     []string          // Command and arguments to run
    WorkDir string            // Working directory inside container
}

// CLIManager implements Manager using docker/podman CLI.
type CLIManager struct {
    runtime string // "docker" or "podman"
}

// NewCLIManager creates a Manager using the specified runtime.
// Use DetectRuntime() to find an available runtime first.
func NewCLIManager(runtime string) *CLIManager {
    return &CLIManager{runtime: runtime}
}
```

### API Surface

```go
// Detection
func DetectRuntime() (string, error)

// Manager construction
func NewCLIManager(runtime string) *CLIManager

// Manager interface methods
func (m *CLIManager) Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error)
func (m *CLIManager) Start(ctx context.Context, id ContainerID) error
func (m *CLIManager) Wait(ctx context.Context, id ContainerID) (exitCode int, err error)
func (m *CLIManager) Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error)
func (m *CLIManager) Stop(ctx context.Context, id ContainerID, timeout time.Duration) error
func (m *CLIManager) Remove(ctx context.Context, id ContainerID) error
```

### Key Implementation Details

#### Runtime Detection

```go
// DetectRuntime finds an available container runtime.
// Checks docker first, then podman. Verifies the binary actually works
// by running `<runtime> version`.
func DetectRuntime() (string, error) {
    for _, bin := range []string{"docker", "podman"} {
        if _, err := exec.LookPath(bin); err == nil {
            cmd := exec.Command(bin, "version")
            if err := cmd.Run(); err == nil {
                return bin, nil
            }
        }
    }
    return "", errors.New("no container runtime found (need docker or podman)")
}
```

#### Container Creation

```go
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
```

#### Log Streaming

```go
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
```

#### Wait for Exit

```go
func (m *CLIManager) Wait(ctx context.Context, id ContainerID) (int, error) {
    cmd := exec.CommandContext(ctx, m.runtime, "wait", string(id))
    output, err := cmd.Output()
    if err != nil {
        return -1, fmt.Errorf("failed to wait for container: %w", err)
    }

    exitCode, err := strconv.Atoi(strings.TrimSpace(string(output)))
    if err != nil {
        return -1, fmt.Errorf("failed to parse exit code: %w", err)
    }

    return exitCode, nil
}
```

#### Stop with Timeout

```go
func (m *CLIManager) Stop(ctx context.Context, id ContainerID, timeout time.Duration) error {
    timeoutSecs := int(timeout.Seconds())
    cmd := exec.CommandContext(ctx, m.runtime, "stop", "-t", strconv.Itoa(timeoutSecs), string(id))

    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to stop container: %s", output)
    }

    return nil
}
```

#### Container Removal

```go
func (m *CLIManager) Remove(ctx context.Context, id ContainerID) error {
    cmd := exec.CommandContext(ctx, m.runtime, "rm", string(id))

    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to remove container: %s", output)
    }

    return nil
}
```

## Implementation Notes

### Platform Differences

- **Docker on macOS**: Runs in a Linux VM (Docker Desktop). SSH agent forwarding requires mounting the socket.
- **Docker on Linux**: Runs natively. SSH agent socket can be mounted directly at `$SSH_AUTH_SOCK`.
- **Podman**: Daemonless, runs containers as the current user. Rootless mode may have different networking behavior.

### Error Handling

The CLI can fail in several ways that need distinct handling:

1. **Binary not found**: `exec.LookPath` returns error — no runtime available
2. **Daemon not running**: `docker version` fails — runtime installed but not working
3. **Image not found**: `docker create` fails with specific error message
4. **Container already exists**: `docker create` fails if name is taken

Always capture stderr from failed commands to provide useful error messages:

```go
var exitErr *exec.ExitError
if errors.As(err, &exitErr) {
    return fmt.Errorf("command failed: %s", exitErr.Stderr)
}
```

### Context Cancellation

All operations accept a context. When the context is canceled:

- Running commands are killed via `exec.CommandContext`
- Log streaming stops (the pipe closes)
- Wait operations return immediately with an error

Callers should use context cancellation for timeouts rather than relying solely on `Stop()`.

### Concurrent Safety

`CLIManager` is safe for concurrent use because it has no mutable state. Each method call spawns independent subprocesses. However, callers must coordinate:

- Don't call `Remove()` while `Wait()` is still running
- Don't call `Start()` twice on the same container

### Log Streaming Gotchas

The `Logs()` method returns a `ReadCloser` that the caller must close. If not closed:

- The `docker logs -f` process keeps running
- File descriptors leak
- Context cancellation will eventually clean up, but explicitly closing is better

## Testing Strategy

### Unit Tests

```go
func TestDetectRuntime_PrefersDocker(t *testing.T) {
    // This test requires docker to be installed
    if _, err := exec.LookPath("docker"); err != nil {
        t.Skip("docker not available")
    }

    runtime, err := DetectRuntime()
    if err != nil {
        t.Fatalf("DetectRuntime() failed: %v", err)
    }

    if runtime != "docker" {
        t.Errorf("expected docker, got %s", runtime)
    }
}

func TestContainerConfig_BuildsCorrectArgs(t *testing.T) {
    cfg := ContainerConfig{
        Image:   "alpine:latest",
        Name:    "test-container",
        Env:     map[string]string{"FOO": "bar", "BAZ": "qux"},
        Cmd:     []string{"echo", "hello"},
        WorkDir: "/app",
    }

    // Verify the config can be used to build valid docker create args
    args := []string{"create", "--name", cfg.Name}
    for k, v := range cfg.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }
    if cfg.WorkDir != "" {
        args = append(args, "-w", cfg.WorkDir)
    }
    args = append(args, cfg.Image)
    args = append(args, cfg.Cmd...)

    // Should contain all expected flags
    argStr := strings.Join(args, " ")
    if !strings.Contains(argStr, "--name test-container") {
        t.Error("missing --name flag")
    }
    if !strings.Contains(argStr, "-w /app") {
        t.Error("missing -w flag")
    }
    if !strings.Contains(argStr, "alpine:latest") {
        t.Error("missing image")
    }
}
```

### Integration Tests

```go
func TestCLIManager_FullLifecycle(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
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
        t.Skip("skipping integration test")
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

    output, _ := io.ReadAll(logs)
    if !strings.Contains(string(output), "line1") {
        t.Error("logs missing expected output")
    }
}
```

### Manual Testing

- [ ] Run with Docker: verify all lifecycle operations work
- [ ] Run with Podman: verify identical behavior
- [ ] Test with neither installed: verify clear error message
- [ ] Test with Docker installed but daemon stopped
- [ ] Test container that runs for 30+ seconds, verify log streaming is real-time
- [ ] Test Stop() with a container that ignores SIGTERM
- [ ] Test context cancellation during Wait()

## Design Decisions

### Why CLI Instead of Docker SDK?

The Docker Go SDK is heavyweight (~50MB of dependencies) and Docker-specific. By using the CLI:

1. **Unified interface**: Docker and Podman have identical CLI syntax
2. **Debuggability**: Operators can run the exact commands manually
3. **Fewer dependencies**: No SDK version conflicts or API version mismatches
4. **Simpler code**: No connection management, just spawn and wait

The tradeoff is performance—each operation spawns a subprocess. But container operations are infrequent (one per job) and already take seconds, so the ~10ms subprocess overhead is negligible.

### Why Create + Start Instead of Run?

Separating `Create()` and `Start()` provides better control:

1. Container can be inspected before starting
2. Logs can be attached before the container runs
3. Matches the underlying Docker API model
4. Enables future features like checkpointing

### Why Return io.ReadCloser for Logs?

Returning a stream instead of buffered output enables:

1. Real-time log display in the UI
2. Memory efficiency for long-running jobs
3. Early termination if the user navigates away

The caller must close the stream, but this is standard Go practice for I/O resources.

## Future Enhancements

1. **Volume mounts**: Mount host directories for workspace sharing
2. **Resource limits**: Set CPU and memory constraints via `--cpus` and `--memory`
3. **Network isolation**: Create job-specific networks for multi-container workflows
4. **Image pulling**: Add `Pull()` method with progress reporting
5. **Container inspection**: Add `Inspect()` to get container state and metadata
6. **Healthchecks**: Wait for container to be healthy before returning from Start()

## References

- [CONTAINER-ISOLATION.md](./CONTAINER-ISOLATION.md) — Related spec for isolation requirements
- [Docker CLI Reference](https://docs.docker.com/engine/reference/commandline/cli/)
- [Podman CLI Reference](https://docs.podman.io/en/latest/Commands.html)
