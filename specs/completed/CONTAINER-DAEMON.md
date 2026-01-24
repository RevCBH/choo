# CONTAINER-DAEMON — Daemon Integration for Dispatching Jobs to Containers and Bridging Events

## Overview

CONTAINER-DAEMON modifies the daemon's job manager to dispatch jobs to containers instead of running the orchestrator in-process. When container mode is enabled, the daemon creates containers using the configured runtime, passes credentials via environment variables, streams container logs in real-time, and parses JSON events from the container's stdout to bridge them to the web UI.

This integration enables isolated job execution where each job runs in its own container with a clean environment. The container receives clone URLs and credentials, performs the orchestration internally, and emits structured JSON events that the daemon parses and forwards to connected clients. After all unit branches merge into the feature branch, the integration runs `choo archive` to move completed specs to the `specs/completed/` directory before pushing to the remote.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DAEMON                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────────┐     ┌──────────────────────────────────────────┐      │
│   │   Job Manager   │     │              Container Runtime            │      │
│   │ (container mode)│────▶│  ┌─────────────────────────────────────┐ │      │
│   └────────┬────────┘     │  │  choo-<job-id>                      │ │      │
│            │              │  │  ┌───────────────────────────────┐  │ │      │
│            │              │  │  │ choo run --json-events        │  │ │      │
│            │              │  │  │         │                     │  │ │      │
│   ┌────────▼────────┐     │  │  │   JSON Events (stdout)        │  │ │      │
│   │  Log Streamer   │◀────│──│──│         │                     │  │ │      │
│   │  & Parser       │     │  │  └─────────┼─────────────────────┘  │ │      │
│   └────────┬────────┘     │  └────────────┼─────────────────────────┘ │      │
│            │              └───────────────┼──────────────────────────┘      │
│   ┌────────▼────────┐                     │                                  │
│   │   Event Bus     │◀────────────────────┘                                  │
│   │  (bridged)      │                                                        │
│   └────────┬────────┘                                                        │
│            │                                                                 │
│   ┌────────▼────────┐                                                        │
│   │   Web UI SSE    │                                                        │
│   └─────────────────┘                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Job manager dispatches jobs to containers when `container_mode` is enabled
2. Pass credentials (GITHUB_TOKEN, ANTHROPIC_API_KEY) via environment variables to container
3. Stream container logs in real-time and parse JSON event lines
4. Track container lifecycle state (creating, running, stopped) in job state
5. Bridge parsed events from container stdout to the daemon's event bus
6. After all unit branches merge into feature branch, run `choo archive`
7. `choo archive` moves completed specs from `specs/` to `specs/completed/`
8. Archive step commits the move with message "chore: archive completed specs"
9. Archive step pushes feature branch to remote after committing
10. Support configurable container runtime (auto-detect, docker, or podman)

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event parsing overhead | < 10ms per event |
| Log streaming latency | < 100ms |
| Container start time | < 5s (excluding image pull) |
| Event bridge throughput | > 100 events/second |

### Constraints

- Depends on CONTAINER-MANAGER for container lifecycle operations
- Depends on JSON-EVENTS for event parsing format
- Credentials passed via environment variables (not mounted files)
- Container must have network access for git clone and API calls
- SSH agent forwarding supported for SSH-based git URLs

## Design

### Module Structure

```
internal/daemon/
├── config.go           # Extended with container config options
├── job_manager.go      # Extended with container dispatch path
├── container_job.go    # Container-specific job logic
├── log_streamer.go     # Real-time log streaming and parsing
└── archive.go          # Archive step implementation

internal/cli/
├── daemon.go           # Extended with --container-mode flag
└── run.go              # Extended with --clone-url, --json-events flags

internal/orchestrator/
└── orchestrator.go     # Extended with archive step in completion flow
```

### Core Types

```go
// internal/daemon/config.go

// Config holds daemon configuration with container options.
type Config struct {
    // Existing fields
    SocketPath string
    PIDFile    string
    DBPath     string
    MaxJobs    int

    // Container mode configuration
    ContainerMode    bool   // Enable container isolation for job execution
    ContainerImage   string // Container image to use, e.g., "choo:latest"
    ContainerRuntime string // "auto", "docker", or "podman"
}
```

```go
// internal/daemon/container_job.go

// ContainerJobConfig extends JobConfig with container-specific settings.
type ContainerJobConfig struct {
    JobConfig

    // GitURL is the repository URL to clone inside the container
    GitURL string

    // TasksDir is the path to specs/tasks/ relative to repo root
    TasksDir string
}

// ContainerJobState tracks the container lifecycle.
type ContainerJobState struct {
    ContainerID   string           // Container ID from runtime
    ContainerName string           // Human-readable name: choo-<job-id>
    Status        ContainerStatus  // creating, running, stopped, failed
    ExitCode      *int             // Exit code when stopped
    StartedAt     *time.Time
    StoppedAt     *time.Time
    Error         string           // Error message if failed
}

// ContainerStatus represents container lifecycle states.
type ContainerStatus string

const (
    ContainerStatusCreating ContainerStatus = "creating"
    ContainerStatusRunning  ContainerStatus = "running"
    ContainerStatusStopped  ContainerStatus = "stopped"
    ContainerStatusFailed   ContainerStatus = "failed"
)
```

```go
// internal/daemon/log_streamer.go

// LogStreamer reads container logs and parses JSON events.
type LogStreamer struct {
    containerID string
    manager     container.Manager
    eventBus    *events.Bus
    parser      *json.Decoder

    cancel context.CancelFunc
    done   chan struct{}
}

// JSONEvent represents the wire format for events from container stdout.
// This type is defined in the JSON-EVENTS spec and imported here.
// See JSON-EVENTS spec for the canonical definition.
type JSONEvent = events.JSONEvent
```

```go
// internal/cli/run.go

// RunOptions holds flags for the run command.
type RunOptions struct {
    // Existing fields
    Parallelism  int
    TargetBranch string
    DryRun       bool
    NoPR         bool
    Unit         string
    SkipReview   bool
    TasksDir     string

    // Container mode additions
    CloneURL   string // URL to clone before running (used in container)
    JSONEvents bool   // Emit events as JSON to stdout (for daemon parsing)
}
```

```go
// internal/cli/daemon.go

// DaemonStartOptions holds flags for daemon start command.
type DaemonStartOptions struct {
    // Existing fields
    Foreground bool
    Verbose    bool

    // Container mode additions
    ContainerMode    bool   // Enable container isolation
    ContainerImage   string // Image to use for jobs
    ContainerRuntime string // Runtime selection
}
```

### API Surface

```go
// internal/daemon/job_manager.go

// StartContainerJob creates and starts a job in a container.
func (jm *JobManager) StartContainerJob(ctx context.Context, cfg ContainerJobConfig) (string, error)

// GetContainerState returns the container state for a job.
func (jm *JobManager) GetContainerState(jobID string) (*ContainerJobState, error)
```

```go
// internal/daemon/log_streamer.go

// NewLogStreamer creates a log streamer for a container.
func NewLogStreamer(containerID string, manager container.Manager, eventBus *events.Bus) *LogStreamer

// Start begins streaming and parsing logs.
func (s *LogStreamer) Start(ctx context.Context) error

// Stop halts log streaming.
func (s *LogStreamer) Stop()

// Done returns a channel that closes when streaming completes.
func (s *LogStreamer) Done() <-chan struct{}
```

```go
// internal/orchestrator/orchestrator.go

// Complete runs the completion flow including archive.
func (o *Orchestrator) Complete(ctx context.Context) error
```

```go
// internal/cli/archive.go (new command)

// Archive moves completed specs to specs/completed/.
func Archive(specsDir string) error
```

### Container Configuration

```yaml
# ~/.choo.yaml example

daemon:
  container_mode: true
  container_image: "ghcr.io/org/choo:latest"
  container_runtime: "auto"  # auto-detect docker or podman
```

### Command Line Interface

```
choo daemon start
    --container-mode           # Enable container isolation
    --container-image IMAGE    # Container image (default: choo:latest)
    --container-runtime RT     # auto, docker, or podman (default: auto)

choo run [tasks-dir]
    --clone-url URL           # Clone repository before running (container mode)
    --json-events             # Emit events as JSON to stdout

choo archive [--specs DIR]    # Move completed specs to specs/completed/
```

## Implementation Notes

### Container Job Flow

```go
func (jm *JobManager) startContainerJob(ctx context.Context, cfg ContainerJobConfig) (string, error) {
    // 1. Generate job ID
    jobID := ulid.Make().String()

    // 2. Create container configuration
    containerCfg := container.ContainerConfig{
        Image: jm.cfg.ContainerImage,
        Name:  fmt.Sprintf("choo-%s", jobID),
        Env: map[string]string{
            "GIT_URL":           cfg.GitURL,
            "GITHUB_TOKEN":      os.Getenv("GITHUB_TOKEN"),
            "ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
        },
        Cmd: []string{
            "choo", "run",
            "--clone-url", cfg.GitURL,
            "--tasks", cfg.TasksDir,
            "--json-events",
        },
    }

    // 3. Record job in database with container state
    now := time.Now()
    run := &db.Run{
        ID:            jobID,
        RepoPath:      cfg.RepoPath,
        Status:        db.RunStatusRunning,
        DaemonVersion: Version,
        StartedAt:     &now,
        ContainerID:   "", // Set after creation
    }
    if err := jm.db.CreateRun(ctx, run); err != nil {
        return "", fmt.Errorf("failed to create run record: %w", err)
    }

    // 4. Create container
    containerID, err := jm.runtime.Create(ctx, containerCfg)
    if err != nil {
        jm.markJobFailed(ctx, jobID, fmt.Sprintf("container create failed: %v", err))
        return "", fmt.Errorf("failed to create container: %w", err)
    }

    // 5. Update run with container ID
    run.ContainerID = containerID
    if err := jm.db.UpdateRun(ctx, run); err != nil {
        jm.runtime.Remove(ctx, containerID)
        return "", fmt.Errorf("failed to update run record: %w", err)
    }

    // 6. Start container
    if err := jm.runtime.Start(ctx, containerID); err != nil {
        jm.markJobFailed(ctx, jobID, fmt.Sprintf("container start failed: %v", err))
        return "", fmt.Errorf("failed to start container: %w", err)
    }

    // 7. Create event bus for this job
    jobEvents := events.NewBus()
    jm.jobs[jobID] = &ManagedJob{
        ID:        jobID,
        Events:    jobEvents,
        StartedAt: time.Now(),
    }

    // 8. Start log streamer in background
    streamer := NewLogStreamer(containerID, jm.runtime, jobEvents)
    go func() {
        defer jm.cleanup(jobID)
        if err := streamer.Start(ctx); err != nil {
            log.Printf("Log streamer error for job %s: %v", jobID, err)
        }
        // Wait for container to exit
        exitCode, err := jm.runtime.Wait(ctx, containerID)
        if err != nil {
            jm.markJobFailed(ctx, jobID, err.Error())
        } else if exitCode != 0 {
            jm.markJobFailed(ctx, jobID, fmt.Sprintf("container exited with code %d", exitCode))
        } else {
            jm.markJobComplete(ctx, jobID)
        }
    }()

    return jobID, nil
}
```

### Log Streaming and Event Parsing

```go
func (s *LogStreamer) Start(ctx context.Context) error {
    ctx, s.cancel = context.WithCancel(ctx)
    defer close(s.done)

    // Get log reader from container manager
    reader, err := s.manager.Logs(ctx, container.ContainerID(s.containerID))
    if err != nil {
        return fmt.Errorf("failed to get container logs: %w", err)
    }
    defer reader.Close()

    // Parse line by line
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        line := scanner.Bytes()
        if err := s.parseLine(line); err != nil {
            // Log parse error but continue - may be non-JSON output
            log.Printf("Failed to parse log line: %v", err)
        }
    }

    return scanner.Err()
}

func (s *LogStreamer) parseLine(line []byte) error {
    // Skip empty lines
    if len(bytes.TrimSpace(line)) == 0 {
        return nil
    }

    // Attempt to parse as JSON event
    var evt JSONEvent
    if err := json.Unmarshal(line, &evt); err != nil {
        // Not a JSON line - likely stderr or debug output
        return nil
    }

    // Convert to internal event type and publish
    internalEvt, err := convertJSONEvent(evt)
    if err != nil {
        return fmt.Errorf("failed to convert event: %w", err)
    }

    s.eventBus.Publish(internalEvt)
    return nil
}

func convertJSONEvent(evt JSONEvent) (events.Event, error) {
    switch evt.Type {
    case "unit.started":
        var payload events.UnitStartedPayload
        if err := json.Unmarshal(evt.Payload, &payload); err != nil {
            return nil, err
        }
        return &events.UnitStarted{
            Timestamp: evt.Timestamp,
            Payload:   payload,
        }, nil
    case "task.completed":
        var payload events.TaskCompletedPayload
        if err := json.Unmarshal(evt.Payload, &payload); err != nil {
            return nil, err
        }
        return &events.TaskCompleted{
            Timestamp: evt.Timestamp,
            Payload:   payload,
        }, nil
    // ... handle other event types
    default:
        return nil, fmt.Errorf("unknown event type: %s", evt.Type)
    }
}
```

### Branch Completion with Archive

```go
func (o *Orchestrator) complete(ctx context.Context) error {
    // 1. All unit branches have been merged to feature branch
    log.Printf("All units complete, running archive step")

    // 2. Run archive to move completed specs
    archiveCmd := exec.CommandContext(ctx, "choo", "archive", "--specs", o.cfg.TasksDir)
    archiveCmd.Dir = o.cfg.RepoPath
    archiveCmd.Stdout = os.Stdout
    archiveCmd.Stderr = os.Stderr
    if err := archiveCmd.Run(); err != nil {
        return fmt.Errorf("archive failed: %w", err)
    }

    // 3. Commit the archive changes
    commitCmd := exec.CommandContext(ctx, "git", "commit", "-am", "chore: archive completed specs")
    commitCmd.Dir = o.cfg.RepoPath
    if err := commitCmd.Run(); err != nil {
        if !isNoChangesError(err) {
            return fmt.Errorf("commit failed: %w", err)
        }
        // No changes to commit - specs were already archived
        log.Printf("No spec changes to archive")
    }

    // 4. Push feature branch to remote
    pushCmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", o.cfg.FeatureBranch)
    pushCmd.Dir = o.cfg.RepoPath
    if err := pushCmd.Run(); err != nil {
        return fmt.Errorf("push failed: %w", err)
    }

    // 5. Create PR
    return o.createPR(ctx)
}

func isNoChangesError(err error) bool {
    if exitErr, ok := err.(*exec.ExitError); ok {
        // git commit returns exit code 1 when there's nothing to commit
        return exitErr.ExitCode() == 1
    }
    return false
}
```

### Archive Command Implementation

```go
// internal/cli/archive.go

func Archive(specsDir string) error {
    // Determine source and destination
    srcDir := specsDir
    if srcDir == "" {
        srcDir = "specs"
    }
    dstDir := filepath.Join(srcDir, "completed")

    // Ensure completed directory exists
    if err := os.MkdirAll(dstDir, 0755); err != nil {
        return fmt.Errorf("failed to create completed directory: %w", err)
    }

    // Find spec files to archive (those with status: complete in frontmatter)
    entries, err := os.ReadDir(srcDir)
    if err != nil {
        return fmt.Errorf("failed to read specs directory: %w", err)
    }

    var archived []string
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
            continue
        }

        srcPath := filepath.Join(srcDir, entry.Name())
        if shouldArchive(srcPath) {
            dstPath := filepath.Join(dstDir, entry.Name())
            if err := os.Rename(srcPath, dstPath); err != nil {
                return fmt.Errorf("failed to move %s: %w", entry.Name(), err)
            }
            archived = append(archived, entry.Name())
        }
    }

    if len(archived) > 0 {
        log.Printf("Archived %d specs: %v", len(archived), archived)
    }

    return nil
}

func shouldArchive(path string) bool {
    // Read file and check frontmatter for status: complete
    data, err := os.ReadFile(path)
    if err != nil {
        return false
    }

    // Simple frontmatter parsing
    content := string(data)
    if !strings.HasPrefix(content, "---") {
        return false
    }

    endIdx := strings.Index(content[3:], "---")
    if endIdx == -1 {
        return false
    }

    frontmatter := content[3 : 3+endIdx]
    return strings.Contains(frontmatter, "status: complete")
}
```

### Credential Handling

The daemon passes credentials to containers via environment variables:

```go
func buildContainerEnv(cfg ContainerJobConfig) map[string]string {
    env := map[string]string{
        "GIT_URL": cfg.GitURL,
    }

    // Pass through credential environment variables
    credVars := []string{
        "GITHUB_TOKEN",
        "ANTHROPIC_API_KEY",
        "OPENAI_API_KEY",
        "SSH_AUTH_SOCK", // For SSH agent forwarding
    }

    for _, v := range credVars {
        if val := os.Getenv(v); val != "" {
            env[v] = val
        }
    }

    return env
}
```

### Error Handling

Container failures should be clearly reported:

```go
func (jm *JobManager) handleContainerFailure(ctx context.Context, jobID string, containerID string, err error) {
    // Get container logs for debugging
    logs, _ := jm.runtime.Logs(ctx, containerID, container.LogsOptions{
        Tail: 100, // Last 100 lines
    })

    // Read logs into string
    var logBuf bytes.Buffer
    io.Copy(&logBuf, logs)
    logs.Close()

    // Update job with failure info
    jm.db.UpdateRunError(ctx, jobID, db.RunError{
        Message:       err.Error(),
        ContainerLogs: logBuf.String(),
        FailedAt:      time.Now(),
    })

    // Publish failure event
    jm.eventBus.Publish(&events.JobFailed{
        JobID:   jobID,
        Error:   err.Error(),
        Details: logBuf.String(),
    })
}
```

## Testing Strategy

### Unit Tests

```go
// internal/daemon/log_streamer_test.go

func TestLogStreamer_ParseJSONEvent(t *testing.T) {
    tests := []struct {
        name    string
        line    string
        wantEvt string
        wantErr bool
    }{
        {
            name:    "valid unit.started event",
            line:    `{"type":"unit.started","timestamp":"2024-01-15T10:00:00Z","payload":{"unit_id":"unit-1"}}`,
            wantEvt: "unit.started",
            wantErr: false,
        },
        {
            name:    "valid task.completed event",
            line:    `{"type":"task.completed","timestamp":"2024-01-15T10:01:00Z","payload":{"task_id":"task-1","unit_id":"unit-1"}}`,
            wantEvt: "task.completed",
            wantErr: false,
        },
        {
            name:    "non-JSON line ignored",
            line:    "Starting orchestrator...",
            wantEvt: "",
            wantErr: false,
        },
        {
            name:    "empty line ignored",
            line:    "",
            wantEvt: "",
            wantErr: false,
        },
        {
            name:    "malformed JSON",
            line:    `{"type":"broken`,
            wantEvt: "",
            wantErr: false, // Non-JSON lines are silently ignored
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            bus := events.NewBus()
            streamer := &LogStreamer{eventBus: bus}

            var received events.Event
            unsub := bus.Subscribe(func(evt events.Event) {
                received = evt
            })
            defer unsub()

            err := streamer.parseLine([]byte(tt.line))
            if (err != nil) != tt.wantErr {
                t.Errorf("parseLine() error = %v, wantErr %v", err, tt.wantErr)
            }

            if tt.wantEvt != "" && received == nil {
                t.Errorf("expected event %s but none received", tt.wantEvt)
            }
        })
    }
}

func TestLogStreamer_EventParsingOverhead(t *testing.T) {
    bus := events.NewBus()
    streamer := &LogStreamer{eventBus: bus}

    // Generate test event JSON
    eventJSON := `{"type":"task.completed","timestamp":"2024-01-15T10:00:00Z","payload":{"task_id":"task-1","unit_id":"unit-1","duration_ms":1234}}`

    // Measure parsing time
    iterations := 1000
    start := time.Now()
    for i := 0; i < iterations; i++ {
        streamer.parseLine([]byte(eventJSON))
    }
    elapsed := time.Since(start)

    avgLatency := elapsed / time.Duration(iterations)
    if avgLatency > 10*time.Millisecond {
        t.Errorf("Event parsing overhead %v exceeds 10ms target", avgLatency)
    }
}
```

```go
// internal/daemon/container_job_test.go

func TestContainerJobConfig_BuildEnv(t *testing.T) {
    // Set test environment
    os.Setenv("GITHUB_TOKEN", "test-gh-token")
    os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
    defer os.Unsetenv("GITHUB_TOKEN")
    defer os.Unsetenv("ANTHROPIC_API_KEY")

    cfg := ContainerJobConfig{
        GitURL:   "https://github.com/org/repo.git",
        TasksDir: "specs/tasks",
    }

    env := buildContainerEnv(cfg)

    if env["GIT_URL"] != cfg.GitURL {
        t.Errorf("GIT_URL = %q, want %q", env["GIT_URL"], cfg.GitURL)
    }
    if env["GITHUB_TOKEN"] != "test-gh-token" {
        t.Errorf("GITHUB_TOKEN not passed through")
    }
    if env["ANTHROPIC_API_KEY"] != "test-anthropic-key" {
        t.Errorf("ANTHROPIC_API_KEY not passed through")
    }
}

func TestContainerJobConfig_MissingCredentials(t *testing.T) {
    // Ensure credentials are not set
    os.Unsetenv("GITHUB_TOKEN")
    os.Unsetenv("ANTHROPIC_API_KEY")

    cfg := ContainerJobConfig{
        GitURL: "https://github.com/org/repo.git",
    }

    env := buildContainerEnv(cfg)

    if _, ok := env["GITHUB_TOKEN"]; ok {
        t.Error("GITHUB_TOKEN should not be present when unset")
    }
}
```

```go
// internal/cli/archive_test.go

func TestArchive_MovesCompletedSpecs(t *testing.T) {
    // Create temp directory structure
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    os.MkdirAll(specsDir, 0755)

    // Create test spec files
    completeSpec := `---
status: complete
---
# Complete Spec`
    incompleteSpec := `---
status: in_progress
---
# Incomplete Spec`

    os.WriteFile(filepath.Join(specsDir, "COMPLETE.md"), []byte(completeSpec), 0644)
    os.WriteFile(filepath.Join(specsDir, "INCOMPLETE.md"), []byte(incompleteSpec), 0644)

    // Run archive
    err := Archive(specsDir)
    if err != nil {
        t.Fatalf("Archive() error = %v", err)
    }

    // Verify complete spec moved
    completedDir := filepath.Join(specsDir, "completed")
    if _, err := os.Stat(filepath.Join(completedDir, "COMPLETE.md")); os.IsNotExist(err) {
        t.Error("COMPLETE.md should be in completed directory")
    }
    if _, err := os.Stat(filepath.Join(specsDir, "COMPLETE.md")); !os.IsNotExist(err) {
        t.Error("COMPLETE.md should not be in specs directory")
    }

    // Verify incomplete spec not moved
    if _, err := os.Stat(filepath.Join(specsDir, "INCOMPLETE.md")); os.IsNotExist(err) {
        t.Error("INCOMPLETE.md should remain in specs directory")
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Container job execution | Start daemon with container mode, submit job, verify container created and events bridged |
| Log streaming | Create container with known output, verify events parsed and published |
| Container failure handling | Submit job that fails, verify error captured with container logs |
| Archive on completion | Complete all units, verify specs moved and committed |
| Credential passthrough | Verify container receives credentials from daemon environment |

### Manual Testing

- [ ] `choo daemon start --container-mode` starts daemon with container support
- [ ] `choo run` in container mode creates container with correct name
- [ ] Events from container appear in web UI via SSE
- [ ] Container failure shows helpful error message with logs
- [ ] `choo archive` moves completed specs to `specs/completed/`
- [ ] Full workflow: job completes, specs archived, committed, PR created
- [ ] Missing credentials produce clear error message

## Design Decisions

### Why Environment Variables for Credentials?

**Decision**: Pass credentials via environment variables rather than mounted files.

**Rationale**:
- Simpler to implement across different container runtimes
- Environment variables are the standard pattern for CI/CD systems
- No need to manage file permissions inside container
- Works with both Docker and Podman without configuration differences
- Tradeoff: Credentials visible in process listing (acceptable for local daemon)

### Why Parse JSON from stdout?

**Decision**: Container emits JSON events to stdout, daemon parses line by line.

**Rationale**:
- Simple, reliable IPC mechanism that works everywhere
- No additional dependencies (no sockets, no shared volumes)
- Easy to debug - can run container manually and see output
- Supports streaming - events arrive in real-time
- Alternative considered: Unix socket inside container (rejected - complexity)

### Why Archive as Separate Step?

**Decision**: Archive runs after all units merge, before push.

**Rationale**:
- Clean separation of concerns - orchestrator focuses on execution
- Archive can be run manually with `choo archive` for testing
- Commit message clearly indicates automated archival
- If archive fails, units are still complete (safe failure mode)
- Tradeoff: Extra git commit in history (acceptable for clarity)

### Why Auto-detect Container Runtime?

**Decision**: Default to "auto" which tries docker, then podman.

**Rationale**:
- Most users have one or the other installed
- No configuration needed for common cases
- Explicit override available for edge cases
- Detection is fast (just check binary exists)
- Tradeoff: Potential inconsistency if both installed (mitigated by deterministic order)

## Future Enhancements

1. **Volume Mounting**: Mount host directories for caching dependencies between runs
2. **Resource Limits**: Configure CPU and memory limits per container
3. **Container Reuse**: Keep warm containers for faster job startup
4. **Registry Authentication**: Support private container registries
5. **Health Checks**: Monitor container health during execution
6. **Log Persistence**: Store container logs in database for post-mortem analysis
7. **GPU Support**: Pass through GPU devices for AI workloads

## References

- [CONTAINER-MANAGER Spec](CONTAINER-MANAGER.md) - Container lifecycle operations
- [JSON-EVENTS Spec](JSON-EVENTS.md) - Event format specification
- [DAEMON-CORE Spec](completed/DAEMON-CORE.md) - Base daemon architecture
- [Docker SDK for Go](https://pkg.go.dev/github.com/docker/docker/client)
- [Podman API](https://docs.podman.io/en/latest/_static/api.html)
