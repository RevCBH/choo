---
task: 3
status: pending
backpressure: "go test ./internal/daemon/... -run TestContainerJob"
depends_on: [1, 2]
---

# Container Job Manager

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement StartContainerJob method on JobManager to dispatch jobs to containers, manage container lifecycle, and handle failures.

## Dependencies

### External Specs (must be implemented)
- CONTAINER-MANAGER - provides `container.Manager`, `container.ContainerConfig`

### Task Dependencies (within this unit)
- Task #1 must be complete (provides ContainerJobConfig, ContainerJobState)
- Task #2 must be complete (provides LogStreamer)

### Package Dependencies
- `context` (standard library)
- `fmt` (standard library)
- `os` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/daemon/
├── container_job.go      # CREATE: Container job dispatch logic
└── container_job_test.go # CREATE: Container job tests
```

### Types to Implement

```go
// internal/daemon/container_job.go

package daemon

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"
    "os"
    "time"

    "github.com/anthropics/choo/internal/container"
    "github.com/anthropics/choo/internal/db"
    "github.com/anthropics/choo/internal/events"
    "github.com/oklog/ulid/v2"
)

// ManagedContainerJob tracks a running container job.
type ManagedContainerJob struct {
    ID            string
    Config        ContainerJobConfig
    State         *ContainerJobState
    Events        *events.Bus
    Streamer      *LogStreamer
    StartedAt     time.Time
}
```

### Functions to Implement

```go
// StartContainerJob creates and starts a job in a container.
// It returns the job ID and starts container execution in the background.
func (jm *JobManager) StartContainerJob(ctx context.Context, cfg ContainerJobConfig) (string, error) {
    // 1. Generate job ID if not provided
    jobID := cfg.JobID
    if jobID == "" {
        jobID = ulid.Make().String()
    }

    // 2. Build container configuration
    containerCfg := container.ContainerConfig{
        Image: jm.cfg.ContainerImage,
        Name:  fmt.Sprintf("choo-%s", jobID),
        Env:   buildContainerEnv(cfg),
        Cmd: []string{
            "choo", "run",
            "--clone-url", cfg.GitURL,
            "--tasks", cfg.TasksDir,
            "--json-events",
        },
    }

    if cfg.Unit != "" {
        containerCfg.Cmd = append(containerCfg.Cmd, "--unit", cfg.Unit)
    }

    // 3. Record job in database
    now := time.Now()
    run := &db.Run{
        ID:            jobID,
        RepoPath:      cfg.RepoPath,
        Status:        db.RunStatusRunning,
        DaemonVersion: Version,
        StartedAt:     &now,
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
    run.ContainerID = string(containerID)
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
    startTime := time.Now()

    state := &ContainerJobState{
        ContainerID:   string(containerID),
        ContainerName: containerCfg.Name,
        Status:        ContainerStatusRunning,
        StartedAt:     &startTime,
    }

    // 8. Create and store managed job
    managed := &ManagedContainerJob{
        ID:        jobID,
        Config:    cfg,
        State:     state,
        Events:    jobEvents,
        StartedAt: startTime,
    }

    jm.mu.Lock()
    jm.containerJobs[jobID] = managed
    jm.mu.Unlock()

    // 9. Start log streamer in background
    streamer := NewLogStreamer(string(containerID), jm.runtime, jobEvents)
    managed.Streamer = streamer

    go func() {
        defer jm.cleanupContainerJob(jobID)

        // Start streaming logs
        if err := streamer.Start(ctx); err != nil && ctx.Err() == nil {
            log.Printf("Log streamer error for job %s: %v", jobID, err)
        }

        // Wait for container to exit
        exitCode, err := jm.runtime.Wait(ctx, containerID)
        if err != nil {
            jm.handleContainerFailure(ctx, jobID, string(containerID), err)
        } else if exitCode != 0 {
            jm.handleContainerFailure(ctx, jobID, string(containerID),
                fmt.Errorf("container exited with code %d", exitCode))
        } else {
            jm.markContainerJobComplete(ctx, jobID)
        }
    }()

    return jobID, nil
}

// GetContainerState returns the container state for a job.
func (jm *JobManager) GetContainerState(jobID string) (*ContainerJobState, error) {
    jm.mu.RLock()
    defer jm.mu.RUnlock()

    managed, ok := jm.containerJobs[jobID]
    if !ok {
        return nil, fmt.Errorf("container job not found: %s", jobID)
    }

    return managed.State, nil
}

// buildContainerEnv builds environment variables to pass to the container.
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

// handleContainerFailure captures container logs and marks the job as failed.
func (jm *JobManager) handleContainerFailure(ctx context.Context, jobID string, containerID string, err error) {
    // Get container logs for debugging
    logsReader, logsErr := jm.runtime.Logs(ctx, container.ContainerID(containerID))
    var logContent string
    if logsErr == nil {
        var buf bytes.Buffer
        io.Copy(&buf, logsReader)
        logsReader.Close()
        // Take last 100 lines
        logContent = tailLines(buf.String(), 100)
    }

    // Update job state
    jm.mu.Lock()
    if managed, ok := jm.containerJobs[jobID]; ok {
        managed.State.Status = ContainerStatusFailed
        managed.State.Error = err.Error()
        now := time.Now()
        managed.State.StoppedAt = &now
    }
    jm.mu.Unlock()

    // Update database
    jm.db.UpdateRunError(ctx, jobID, db.RunError{
        Message:       err.Error(),
        ContainerLogs: logContent,
        FailedAt:      time.Now(),
    })

    // Publish failure event
    jm.eventBus.Emit(events.NewEvent(events.OrchFailed, "").
        WithPayload(map[string]string{
            "job_id":  jobID,
            "error":   err.Error(),
            "details": logContent,
        }))
}

// markContainerJobComplete marks a container job as successfully completed.
func (jm *JobManager) markContainerJobComplete(ctx context.Context, jobID string) {
    jm.mu.Lock()
    if managed, ok := jm.containerJobs[jobID]; ok {
        managed.State.Status = ContainerStatusStopped
        exitCode := 0
        managed.State.ExitCode = &exitCode
        now := time.Now()
        managed.State.StoppedAt = &now
    }
    jm.mu.Unlock()

    jm.markJobComplete(ctx, jobID)
}

// cleanupContainerJob removes the container and cleans up resources.
func (jm *JobManager) cleanupContainerJob(jobID string) {
    jm.mu.Lock()
    managed, ok := jm.containerJobs[jobID]
    if ok {
        delete(jm.containerJobs, jobID)
    }
    jm.mu.Unlock()

    if ok && managed.State != nil {
        // Best-effort container removal
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        jm.runtime.Remove(ctx, container.ContainerID(managed.State.ContainerID))
    }
}

// tailLines returns the last n lines of a string.
func tailLines(s string, n int) string {
    lines := bytes.Split([]byte(s), []byte("\n"))
    if len(lines) <= n {
        return s
    }
    return string(bytes.Join(lines[len(lines)-n:], []byte("\n")))
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestContainerJob
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestStartContainerJob_CreatesContainer` | Container is created with correct config |
| `TestStartContainerJob_PassesCredentials` | GITHUB_TOKEN and ANTHROPIC_API_KEY are passed |
| `TestStartContainerJob_MissingCredential` | Missing credentials are not included in env |
| `TestStartContainerJob_StartsLogStreamer` | Log streamer is started for the container |
| `TestStartContainerJob_RecordsInDB` | Job is recorded in database |
| `TestContainerJob_SuccessfulCompletion` | Exit code 0 marks job complete |
| `TestContainerJob_FailureCapture` | Non-zero exit captures logs |
| `TestGetContainerState_ReturnsState` | State is retrievable for running job |
| `TestGetContainerState_NotFound` | Error returned for unknown job |
| `TestBuildContainerEnv_IncludesGitURL` | GIT_URL is included |
| `TestBuildContainerEnv_IncludesCredentials` | Credentials are passed through |

### Test Implementations

```go
// internal/daemon/container_job_test.go

package daemon

import (
    "context"
    "os"
    "testing"
    "time"
)

func TestBuildContainerEnv_IncludesGitURL(t *testing.T) {
    cfg := ContainerJobConfig{
        GitURL: "https://github.com/org/repo.git",
    }

    env := buildContainerEnv(cfg)

    if env["GIT_URL"] != cfg.GitURL {
        t.Errorf("GIT_URL = %q, want %q", env["GIT_URL"], cfg.GitURL)
    }
}

func TestBuildContainerEnv_IncludesCredentials(t *testing.T) {
    os.Setenv("GITHUB_TOKEN", "test-gh-token")
    os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
    defer os.Unsetenv("GITHUB_TOKEN")
    defer os.Unsetenv("ANTHROPIC_API_KEY")

    cfg := ContainerJobConfig{
        GitURL: "https://github.com/org/repo.git",
    }

    env := buildContainerEnv(cfg)

    if env["GITHUB_TOKEN"] != "test-gh-token" {
        t.Error("GITHUB_TOKEN not passed through")
    }
    if env["ANTHROPIC_API_KEY"] != "test-anthropic-key" {
        t.Error("ANTHROPIC_API_KEY not passed through")
    }
}

func TestBuildContainerEnv_MissingCredentialsOmitted(t *testing.T) {
    os.Unsetenv("GITHUB_TOKEN")
    os.Unsetenv("ANTHROPIC_API_KEY")

    cfg := ContainerJobConfig{
        GitURL: "https://github.com/org/repo.git",
    }

    env := buildContainerEnv(cfg)

    if _, ok := env["GITHUB_TOKEN"]; ok {
        t.Error("GITHUB_TOKEN should not be present when unset")
    }
    if _, ok := env["ANTHROPIC_API_KEY"]; ok {
        t.Error("ANTHROPIC_API_KEY should not be present when unset")
    }
}

func TestTailLines(t *testing.T) {
    tests := []struct {
        name  string
        input string
        n     int
        want  string
    }{
        {"fewer than n", "a\nb\nc", 5, "a\nb\nc"},
        {"exactly n", "a\nb\nc", 3, "a\nb\nc"},
        {"more than n", "a\nb\nc\nd\ne", 3, "c\nd\ne"},
        {"empty", "", 5, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tailLines(tt.input, tt.n)
            if got != tt.want {
                t.Errorf("tailLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
            }
        })
    }
}
```

### CI Compatibility

- [x] No external API keys required (tests use env setup/teardown)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Job ID uses ULID for time-ordered uniqueness
- Container name follows pattern `choo-<job-id>` for easy identification
- Credentials are passed via environment variables (not mounted files)
- SSH_AUTH_SOCK is passed for SSH-based git URLs
- Container cleanup happens on both success and failure
- Log tail captures last 100 lines for failure debugging

## NOT In Scope

- Container runtime selection (handled by CONTAINER-MANAGER)
- CLI flag parsing (Task #5)
- Orchestrator integration (Task #6)
- Job queuing and rate limiting (existing JobManager handles this)
