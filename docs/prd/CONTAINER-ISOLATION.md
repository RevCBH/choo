---
prd_id: container-isolation
title: "Container Isolation for Daemon Jobs"
status: draft
depends_on:
  - daemon-architecture
# Orchestrator-managed fields
# feature_branch: feature/container-isolation
# feature_status: pending
# spec_review_iterations: 0
---

# Container Isolation for Daemon Jobs

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-21 |
| Target  | v0.6       |

---

## 1. Overview

### 1.1 Problem Statement

Charlotte currently executes LLM agent jobs directly on the host system. While the daemon architecture (v0.5) provides job isolation at the process level, git operations can still interfere between concurrent jobs:

1. **Shared Git State**: Multiple jobs accessing the same repository can create conflicts in worktrees, branches, and index files
2. **Environment Leakage**: Jobs share the host environment, making behavior dependent on local tool versions
3. **Reproducibility**: Jobs may behave differently across development machines and CI environments
4. **Partial Push Risk**: If a job fails mid-execution, partial changes may already be pushed to remote

### 1.2 Solution

Isolate each feature job inside a Docker/Podman container:

1. **One container per feature job** contains a full repository clone with worktrees
2. **Single cross-compiled `choo` binary** runs inside the container (same code paths)
3. **Git changes only push to remote when all units complete** successfully
4. **Container runtime detected automatically** (Docker or Podman via CLI)

### 1.3 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         HOST SYSTEM                             │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    DAEMON PROCESS                        │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │   │
│  │  │ JobManager   │  │ Container    │  │ Event Bus    │   │   │
│  │  │ - dispatch   │  │ Manager      │  │ - SSE to UI  │   │   │
│  │  │ - track      │  │ - spawn      │  │ - DB persist │   │   │
│  │  │ - cleanup    │  │ - logs       │  │              │   │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                    Docker/Podman CLI                            │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    CONTAINER                             │   │
│  │                                                          │   │
│  │  /repo (full clone)                                     │   │
│  │  ├── .git/                                              │   │
│  │  ├── .ralph/worktrees/                                  │   │
│  │  │   ├── unit-a/  (worktree)                           │   │
│  │  │   ├── unit-b/  (worktree)                           │   │
│  │  │   └── unit-c/  (worktree)                           │   │
│  │  └── specs/tasks/                                       │   │
│  │                                                          │   │
│  │  ┌──────────────────────────────────────────────────┐   │   │
│  │  │ choo run (same binary, cross-compiled linux/amd64)│   │   │
│  │  │ - discovers units                                 │   │   │
│  │  │ - creates worktrees                               │   │   │
│  │  │ - runs workers in parallel                        │   │   │
│  │  │ - merges to LOCAL feature branch                  │   │   │
│  │  │ - runs choo archive (moves completed specs)       │   │   │
│  │  │ - pushes feature branch when ALL complete         │   │   │
│  │  │ - creates PR                                      │   │   │
│  │  └──────────────────────────────────────────────────┘   │   │
│  │                                                          │   │
│  │  Baked into image: git, Claude CLI, choo (linux/amd64)  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.4 Design Principles

1. **CLI-Based Container Management**: Use Docker/Podman CLI (not SDK) for portability
2. **Same Binary Everywhere**: Cross-compile `choo` for Linux, same code in/out of container
3. **Environment Variable Credentials**: Pass secrets via env vars, not volume mounts
4. **Push on Batch Completion**: Only push when all units succeed (atomic feature delivery)
5. **No Initial Caching**: Simplicity first, optimize later

### 1.5 Non-Goals (Initial Implementation)

- Git cache volumes for faster clones
- Dependency caching (Go modules, npm)
- Per-unit containers (higher parallelism)
- Resource limits (CPU, memory)
- Container registry integration
- Remote Docker host execution

---

## 2. Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Container scope | One per feature job | Keep worktree model, isolate at job level |
| Binary | Single `choo` binary, cross-compiled | No separate runner, same code paths in/out of container |
| Git workflow | Worktrees inside container, push at end | Matches current model, only final result exposed |
| Container runtime | CLI-based (docker/podman) | Portable, simple, debuggable |
| Credentials | Environment variables | Simplest approach (GITHUB_TOKEN, ANTHROPIC_API_KEY) |
| Caching | None initially | Optimize later |
| Claude CLI | Baked into image | Self-contained, version controlled |

---

## 3. Requirements

### 3.1 Functional Requirements

#### Container Manager

1. Detect available container runtime (Docker or Podman)
2. Create containers with specified image and configuration
3. Start, stop, and remove containers
4. Stream container logs in real-time
5. Wait for container exit and capture exit code

#### Structured Event Output

1. `choo run` must emit JSON events to stdout when running non-interactively
2. Events include unit/task lifecycle (started, completed, failed)
3. Daemon parses container stdout and bridges events to web UI
4. Format: JSON lines (newline-delimited JSON)

#### Container Image

1. Multi-stage Dockerfile building `choo` for `linux/amd64`
2. Include git, Claude CLI, and GitHub CLI (`gh`)
3. Minimal Alpine-based runtime image
4. Build script for cross-compilation and image creation

#### Daemon Container Dispatch

1. Job manager can dispatch jobs to containers instead of in-process
2. Pass credentials via environment variables
3. Stream container logs and parse events
4. Track container lifecycle in job state

#### Clone-and-Run Support

1. `choo run` accepts `--clone-url` flag for container execution
2. Clone repository before running orchestrator
3. Checkout specified branch after clone

#### Branch Completion Flow

1. After all unit branches merge into feature branch, run `choo archive`
2. `choo archive` moves completed specs from `specs/` to `specs/completed/`
3. Archive step commits the move before pushing to remote
4. This ensures the feature branch includes spec archival in final state

#### CLI Integration

1. Daemon supports `--container-mode` and `--container-image` flags
2. `choo run` supports `--clone-url` and `--json-events` flags
3. Container runtime auto-detected or explicitly configured

### 3.2 Performance Requirements

| Metric | Target |
|--------|--------|
| Container creation | < 2s |
| Container start latency | < 500ms |
| Event parsing overhead | < 10ms per event |
| Log streaming latency | < 100ms |
| Container stop (graceful) | < 10s |

### 3.3 Constraints

1. Container runtime must be Docker or Podman (CLI-based, not API)
2. Containers run with network access (required for git push, API calls)
3. Credentials passed via environment variables or SSH agent forwarding
4. Single `choo` binary cross-compiled for Linux (no separate runner binary)
5. No caching in initial implementation (future optimization)
6. Claude CLI baked into container image during build
7. No container registry; images built locally only
8. No CI testing for container isolation (local development feature)

---

## 4. Design

### 4.1 Module Structure

```
internal/container/
├── manager.go          # Manager interface definition
├── cli.go              # CLI-based docker/podman implementation
├── config.go           # ContainerConfig type
└── detect.go           # Runtime detection utilities
```

### 4.2 Core Types

```go
// internal/container/manager.go

// ContainerID is a unique identifier for a container
type ContainerID string

// Manager provides container lifecycle management
type Manager interface {
    // Create creates a new container without starting it
    Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error)

    // Start starts a previously created container
    Start(ctx context.Context, id ContainerID) error

    // Wait blocks until the container exits, returning the exit code
    Wait(ctx context.Context, id ContainerID) (exitCode int, err error)

    // Logs returns a reader for container stdout/stderr
    Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error)

    // Stop signals the container to stop with optional timeout
    Stop(ctx context.Context, id ContainerID, timeout time.Duration) error

    // Remove removes a stopped container
    Remove(ctx context.Context, id ContainerID) error
}
```

```go
// internal/container/config.go

// ContainerConfig specifies container creation parameters
type ContainerConfig struct {
    Image   string            // Container image (e.g., "choo:latest")
    Name    string            // Container name (e.g., "choo-<job-id>")
    Env     map[string]string // Environment variables
    Cmd     []string          // Command and arguments
    WorkDir string            // Working directory inside container
}
```

```go
// internal/container/cli.go

// CLIManager implements Manager using docker/podman CLI
type CLIManager struct {
    runtime string // "docker" or "podman"
}

// NewCLIManager creates a CLI-based container manager
func NewCLIManager(runtime string) (*CLIManager, error)
```

```go
// internal/container/detect.go

// DetectRuntime finds an available container runtime
func DetectRuntime() (string, error)

// ValidateRuntime checks if a specific runtime is available
func ValidateRuntime(runtime string) error
```

### 4.3 Event Format

```go
// internal/events/json.go

// JSONEvent represents a serialized event for container output
type JSONEvent struct {
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data,omitempty"`
}

// Example events:
// {"type":"unit.started","timestamp":"2024-01-15T10:30:00Z","data":{"unit":"web-api"}}
// {"type":"task.completed","timestamp":"2024-01-15T10:31:00Z","data":{"unit":"web-api","task":1}}
// {"type":"unit.completed","timestamp":"2024-01-15T10:35:00Z","data":{"unit":"web-api"}}
// {"type":"run.completed","timestamp":"2024-01-15T10:40:00Z","data":{"success":true}}
```

### 4.4 Daemon Integration Types

```go
// internal/daemon/config.go additions

type Config struct {
    // ... existing fields ...

    ContainerMode    bool   `yaml:"container_mode"`    // Enable container isolation
    ContainerImage   string `yaml:"container_image"`   // e.g., "choo:latest"
    ContainerRuntime string `yaml:"container_runtime"` // "auto", "docker", or "podman"
}
```

### 4.5 API Surface

#### Container Manager

```go
// Create a container manager using the specified or auto-detected runtime
func NewManager(runtime string) (Manager, error)

// CLIManager methods
func (m *CLIManager) Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error)
func (m *CLIManager) Start(ctx context.Context, id ContainerID) error
func (m *CLIManager) Wait(ctx context.Context, id ContainerID) (int, error)
func (m *CLIManager) Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error)
func (m *CLIManager) Stop(ctx context.Context, id ContainerID, timeout time.Duration) error
func (m *CLIManager) Remove(ctx context.Context, id ContainerID) error
```

#### Event Bus Additions

```go
// JSONEmitter writes events as JSON lines to a writer
type JSONEmitter struct { w io.Writer }
func NewJSONEmitter(w io.Writer) *JSONEmitter
func (e *JSONEmitter) Emit(event Event) error

// JSONLineReader reads events from a JSON lines stream
type JSONLineReader struct { r *bufio.Reader }
func NewJSONLineReader(r io.Reader) *JSONLineReader
func (jr *JSONLineReader) Read() (Event, error)

// ParseJSONEvent parses a JSON line into an Event
func ParseJSONEvent(line []byte) (Event, error)
```

#### CLI Additions

```go
// internal/cli/run.go additions
type RunOptions struct {
    // ... existing fields ...
    CloneURL   string // URL to clone before running
    JSONEvents bool   // Emit events as JSON to stdout
}

// internal/cli/daemon.go additions
type DaemonStartOptions struct {
    // ... existing fields ...
    ContainerMode    bool
    ContainerImage   string
    ContainerRuntime string
}
```

---

## 5. Implementation Plan

### Phase 1: Container Manager Package

Create `internal/container/` package with CLI-based container management.

**Files to create:**
- `internal/container/manager.go` - Manager interface
- `internal/container/cli.go` - CLI-based implementation (docker/podman)
- `internal/container/config.go` - ContainerConfig type
- `internal/container/detect.go` - Runtime detection

**Key implementation:**
```go
func DetectRuntime() (string, error) {
    for _, bin := range []string{"docker", "podman"} {
        if _, err := exec.LookPath(bin); err == nil {
            // Verify it works
            cmd := exec.Command(bin, "version")
            if err := cmd.Run(); err == nil {
                return bin, nil
            }
        }
    }
    return "", errors.New("no container runtime found (need docker or podman)")
}
```

### Phase 2: Structured Event Output for `choo run`

Ensure `choo run` outputs JSON events to stdout that can be parsed by the daemon.

**Files to create:**
- `internal/events/json.go` - JSON emitter and parser

**Files to modify:**
- `internal/events/bus.go` - Add JSON stdout subscriber option
- `internal/cli/run.go` - Add `--json-events` flag (or auto-detect non-TTY)

**Event format (JSON lines):**
```json
{"type":"unit.started","unit":"web-api","timestamp":"2024-01-15T10:30:00Z"}
{"type":"task.completed","unit":"web-api","task":1,"timestamp":"2024-01-15T10:31:00Z"}
{"type":"unit.completed","unit":"web-api","timestamp":"2024-01-15T10:35:00Z"}
```

### Phase 3: Container Image

Create Dockerfile that bundles `choo` (cross-compiled) with dependencies.

**Files to create:**
- `Dockerfile` - Multi-stage build for container image
- `scripts/build-image.sh` - Build script with cross-compilation

**Dockerfile:**
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /choo ./cmd/choo

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache git openssh-client ca-certificates bash curl github-cli
COPY --from=builder /choo /usr/local/bin/
WORKDIR /repo
ENTRYPOINT ["choo"]
```

**Build script:**
```bash
#!/bin/bash
set -e
VERSION=${1:-latest}
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/choo-linux-amd64 ./cmd/choo
docker build -t "choo:${VERSION}" .
```

### Phase 4: Daemon Integration

Modify daemon to spawn containers instead of running orchestrator in-process.

**Files to modify:**
- `internal/daemon/job_manager.go` - Add container dispatch path
- `internal/daemon/config.go` - Add container config options

**Container job flow:**
```go
func (jm *jobManagerImpl) startContainerJob(ctx context.Context, cfg JobConfig) (string, error) {
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
    // Create, start, stream logs, wait for completion...
}
```

### Phase 5: Clone-and-Run Wrapper

Add startup logic to `choo run` for container execution.

**Files to modify:**
- `internal/cli/run.go` - Add `--clone-url` flag

**Implementation:**
```go
func (cmd *runCmd) Execute(ctx context.Context, opts RunOptions) error {
    if opts.CloneURL != "" {
        repoPath := "/repo"
        exec.CommandContext(ctx, "git", "clone", opts.CloneURL, repoPath).Run()
        if opts.TargetBranch != "" {
            exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", opts.TargetBranch).Run()
        }
        opts.RepoPath = repoPath
    }
    // Continue with normal orchestrator run...
}
```

### Phase 6: CLI Integration

Add container-related flags to daemon CLI.

**Files to modify:**
- `internal/cli/daemon.go` - Add container flags

**Daemon flags:**
```
choo daemon start --container-mode --container-image choo:latest
```

**Run flags (for container execution):**
```
choo run --clone-url https://github.com/owner/repo.git --tasks specs/tasks
```

### Phase 7: Branch Completion with Archive

Integrate `choo archive` into the branch completion flow.

**Files to modify:**
- `internal/orchestrator/orchestrator.go` - Add archive step after all units complete

**Completion flow:**
```go
func (o *Orchestrator) complete(ctx context.Context) error {
    // 1. All unit branches have been merged to feature branch

    // 2. Run archive to move completed specs
    archiveCmd := exec.CommandContext(ctx, "choo", "archive", "--specs", o.cfg.TasksDir)
    archiveCmd.Dir = o.cfg.RepoPath
    if err := archiveCmd.Run(); err != nil {
        return fmt.Errorf("archive failed: %w", err)
    }

    // 3. Commit the archive changes
    commitCmd := exec.CommandContext(ctx, "git", "commit", "-am", "chore: archive completed specs")
    commitCmd.Dir = o.cfg.RepoPath
    if err := commitCmd.Run(); err != nil {
        // No changes to commit is OK
        if !isNoChangesError(err) {
            return fmt.Errorf("commit failed: %w", err)
        }
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
```

---

## 6. Files Summary

### Files to Create

| Path | Purpose |
|------|---------|
| `internal/container/manager.go` | Container manager interface |
| `internal/container/cli.go` | CLI-based docker/podman implementation |
| `internal/container/config.go` | Container configuration types |
| `internal/container/detect.go` | Runtime detection utilities |
| `internal/events/json.go` | JSON event emitter and parser |
| `Dockerfile` | Container image definition |
| `scripts/build-image.sh` | Cross-compile and build image |

### Files to Modify

| Path | Changes |
|------|---------|
| `internal/daemon/job_manager.go` | Add container dispatch path |
| `internal/daemon/config.go` | Add container config options |
| `internal/cli/daemon.go` | Add --container-mode flag |
| `internal/cli/run.go` | Add --clone-url flag, --json-events flag |
| `internal/events/bus.go` | Add JSON stdout emitter |
| `internal/orchestrator/orchestrator.go` | Add archive step in completion flow |

---

## 7. Acceptance Criteria

- [ ] `choo daemon start --container-mode` starts daemon with container support
- [ ] Daemon auto-detects Docker or Podman runtime
- [ ] Container is created and started for each job
- [ ] Container logs stream to daemon and parse as events
- [ ] Events flow from container to web UI via SSE
- [ ] `--clone-url` flag clones repository before running
- [ ] `--json-events` flag emits JSON lines to stdout
- [ ] Container is cleaned up after job completion
- [ ] After all units complete, `choo archive` runs before push
- [ ] Completed specs are moved to `specs/completed/` and committed
- [ ] Git push only occurs when all units complete successfully
- [ ] Credentials passed via environment variables work correctly
- [ ] `choo run` inside container produces same behavior as host
- [ ] Build script produces working container image
- [ ] Both Docker and Podman work as container runtimes

---

## 8. Verification

### Unit Tests

| Test Case | Description |
|-----------|-------------|
| `TestCLIManager_Create` | Verify container creation with config |
| `TestCLIManager_Start` | Verify container start |
| `TestCLIManager_Wait` | Verify waiting for exit code |
| `TestCLIManager_Logs` | Verify log streaming |
| `TestCLIManager_Stop` | Verify graceful stop with timeout |
| `TestCLIManager_Remove` | Verify container removal |
| `TestDetectRuntime_Docker` | Verify Docker detection |
| `TestDetectRuntime_Podman` | Verify Podman detection |
| `TestDetectRuntime_None` | Verify error when no runtime |
| `TestJSONEmitter_Emit` | Verify JSON event serialization |
| `TestJSONLineReader_Read` | Verify JSON event parsing |
| `TestJobManager_StartContainerJob` | Verify container job creation |
| `TestJobManager_ProcessContainerLogs` | Verify event bridging |

### Integration Tests

| Test Case | Description |
|-----------|-------------|
| `TestContainer_FullLifecycle` | Create, start, wait, remove |
| `TestContainer_LogStreaming` | Verify real-time log output |
| `TestContainer_CloneAndRun` | Clone repo and run choo |
| `TestDaemon_ContainerMode` | Full daemon with container dispatch |
| `TestEventBridge_ContainerToWeb` | Events flow from container to web UI |
| `TestOrchestrator_ArchiveOnComplete` | Archive runs after all units merge, before push |

### Manual Testing

```bash
# Build the image
./scripts/build-image.sh

# Test container directly (GitHub token)
docker run -e GIT_URL=https://github.com/owner/repo.git \
           -e GITHUB_TOKEN=$GITHUB_TOKEN \
           -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
           choo:latest \
           run --clone-url https://github.com/owner/repo.git --tasks specs/tasks

# Test container directly (SSH agent forwarding)
docker run -v $SSH_AUTH_SOCK:/ssh-agent \
           -e SSH_AUTH_SOCK=/ssh-agent \
           -e GIT_URL=git@github.com:owner/repo.git \
           -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
           choo:latest \
           run --clone-url git@github.com:owner/repo.git --tasks specs/tasks

# Start daemon in container mode
choo daemon start --container-mode --container-image choo:latest

# Run a job (daemon dispatches to container)
choo run --tasks specs/tasks/my-feature --feature

# Watch container logs
docker logs -f choo-<job-id>
```

---

## 9. Future Enhancements

1. **Git Cache Volume**: Mount shared volume for faster clones
2. **Dependency Caching**: Cache Go modules, npm packages between runs
3. **Per-Unit Containers**: Higher parallelism with unit-level isolation
4. **Resource Limits**: CPU and memory constraints per container
5. **Container Registry**: Push images to registry for distributed execution
6. **Remote Execution**: Run containers on remote Docker hosts or Kubernetes
7. **GPU Support**: Pass through GPU for ML workloads

---

## 10. Resolved Questions

1. **Claude CLI Installation**: Bake into image during build from official installer. This keeps containers self-contained and version-controlled.

2. **Image Distribution**: No container registry. Images are built locally via `scripts/build-image.sh`. Container isolation is not tested in CI (local development feature only).

3. **Private Repo Authentication**: Both GitHub token (HTTPS) and SSH agent forwarding are supported. Users can choose based on their environment:
   - **GitHub Token**: Pass `GITHUB_TOKEN` env var, clone via HTTPS
   - **SSH Agent Forwarding**: Use `-v $SSH_AUTH_SOCK:/ssh-agent -e SSH_AUTH_SOCK=/ssh-agent` when running container
