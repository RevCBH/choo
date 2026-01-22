# Specs

This directory contains specifications for choo development.

## Structure

- `completed/` - Archived specs from the initial implementation phase (v0.1)
- Future specs go in the root or in `tasks/` for active work

## Completed Specs (v0.1)

The `completed/` directory contains the original implementation specs that built the core choo system:

- **CLI** - Command-line interface (run, status, resume, cleanup, version)
- **CONFIG** - YAML configuration and environment overrides
- **DISCOVERY** - Spec parsing from `specs/tasks/` directories
- **EVENTS** - Pub/sub event bus with 30+ event types
- **GIT** - Worktree management, branching, commits, merges
- **GITHUB** - PR lifecycle via GitHub API
- **SCHEDULER** - Dependency DAG, state machine, ready queue
- **WORKER** - Task execution loop, Claude CLI invocation, backpressure

Each spec has an `IMPLEMENTATION_PLAN.md` with numbered task files (`01-*.md`, `02-*.md`, etc.) in `completed/tasks/<unit>/`.

## Self-Hosting Specs (v0.2)

These specs close the gaps between v0.1 components and a fully operational self-hosting loop:

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[ESCALATION](ESCALATION.md)** | User notification interface (Terminal, Slack, Webhook, Multi) | - |
| **[ORCHESTRATOR](ORCHESTRATOR.md)** | Main coordination loop wiring all components | ESCALATION |
| **[CLAUDE-GIT](CLAUDE-GIT.md)** | Delegation of git ops (commit, push, PR) to Claude | ESCALATION |
| **[REVIEW-POLLING](REVIEW-POLLING.md)** | PR review state machine with emoji protocol | CLAUDE-GIT |
| **[CONFLICT-RESOLUTION](CONFLICT-RESOLUTION.md)** | Merge conflict detection and Claude-delegated resolution | CLAUDE-GIT |
| **[CI](CI.md)** | GitHub Actions workflow for automated testing | - |

### Dependency Graph

```
         ┌─────────────┐
         │ ESCALATION  │
         └──────┬──────┘
                │
         ┌──────┴──────┐
         ▼             ▼
   ┌───────────┐ ┌───────────┐
   │ORCHESTRATOR│ │CLAUDE-GIT │
   └───────────┘ └─────┬─────┘
                       │
                ┌──────┴──────┐
                ▼             ▼
          ┌────────────┐ ┌────────────┐
          │  REVIEW-   │ │ CONFLICT-  │
          │  POLLING   │ │ RESOLUTION │
          └────────────┘ └────────────┘

   ┌────────┐
   │   CI   │  (independent)
   └────────┘
```

### Implementation Order

1. **ESCALATION** + **CI** (parallel, no dependencies)
2. **ORCHESTRATOR** + **CLAUDE-GIT** (parallel, depend on ESCALATION)
3. **REVIEW-POLLING** + **CONFLICT-RESOLUTION** (parallel, depend on CLAUDE-GIT)

## Monitoring Specs (v0.3)

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[WEB](WEB.md)** | Real-time web dashboard daemon for orchestrator monitoring via HTTP/SSE | EVENTS, CLI |
| **[WEB-PUSHER](WEB-PUSHER.md)** | Event pusher that connects `choo run` to web UI via Unix socket | WEB, EVENTS |
| **[WEB-FRONTEND](WEB-FRONTEND.md)** | Browser UI with D3.js dependency graph visualization | WEB |

### Dependency Graph

```
   ┌────────┐   ┌────────┐
   │ EVENTS │   │  CLI   │
   └───┬────┘   └───┬────┘
       │            │
       └──────┬─────┘
              ▼
          ┌───────┐
          │  WEB  │
          └───┬───┘
              │
       ┌──────┴──────┐
       ▼             ▼
┌────────────┐ ┌─────────────┐
│ WEB-PUSHER │ │ WEB-FRONTEND│
└────────────┘ └─────────────┘
```

### Implementation Order

1. **WEB** (depends on v0.1 EVENTS and CLI)
2. **WEB-PUSHER** + **WEB-FRONTEND** (parallel, both depend on WEB)

## Multi-Provider Specs (v0.4)

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[PROVIDER-INTERFACE](PROVIDER-INTERFACE.md)** | Provider interface definition and factory pattern for CLI-based LLM providers | - |
| **[PROVIDER-IMPLEMENTATIONS](PROVIDER-IMPLEMENTATIONS.md)** | Claude and Codex CLI subprocess implementations of the Provider interface | PROVIDER-INTERFACE |
| **[PROVIDER-CONFIG](PROVIDER-CONFIG.md)** | Configuration schema, environment variables, CLI flags, and precedence resolution | CONFIG, CLI, DISCOVERY |
| **[PROVIDER-INTEGRATION](PROVIDER-INTEGRATION.md)** | Worker and orchestrator changes to use Provider abstraction for task execution | PROVIDER-INTERFACE, PROVIDER-IMPLEMENTATIONS, PROVIDER-CONFIG |

### Dependency Graph

```
                              ┌────────┐  ┌─────┐  ┌───────────┐
                              │ CONFIG │  │ CLI │  │ DISCOVERY │
                              └───┬────┘  └──┬──┘  └─────┬─────┘
                                  │         │           │
                                  └─────────┴───────────┘
                                            │
┌──────────────────┐                        ▼
│PROVIDER-INTERFACE│              ┌─────────────────┐
└────────┬─────────┘              │ PROVIDER-CONFIG │
         │                        └────────┬────────┘
         ▼                                 │
┌────────────────────────┐                 │
│PROVIDER-IMPLEMENTATIONS│                 │
└────────────┬───────────┘                 │
             │                             │
             └──────────────┬──────────────┘
                            │
                            ▼
                ┌────────────────────┐
                │PROVIDER-INTEGRATION│
                └────────────────────┘
```

### Implementation Order

1. **PROVIDER-INTERFACE** + **PROVIDER-CONFIG** (parallel - interface has no deps, config depends on v0.1)
2. **PROVIDER-IMPLEMENTATIONS** (depends on PROVIDER-INTERFACE)
3. **PROVIDER-INTEGRATION** (depends on all three above specs)

## Feature Workflow Specs (v0.5)

These specs enable PRD-based automated feature development:

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[FEATURE-DISCOVERY](FEATURE-DISCOVERY.md)** | PRD frontmatter parsing, discovery, feature event types | - |
| **[FEATURE-PRIORITIZER](FEATURE-PRIORITIZER.md)** | PRD prioritization + `choo next-feature` command | FEATURE-DISCOVERY |
| **[FEATURE-BRANCH](FEATURE-BRANCH.md)** | Feature branch creation and management | FEATURE-DISCOVERY, GIT |
| **[SPEC-REVIEW](SPEC-REVIEW.md)** | Review loop with schema validation and feedback | FEATURE-DISCOVERY |
| **[FEATURE-WORKFLOW](FEATURE-WORKFLOW.md)** | State machine, commit step, drift detection, auto-completion | FEATURE-DISCOVERY, FEATURE-BRANCH, SPEC-REVIEW |
| **[FEATURE-CLI](FEATURE-CLI.md)** | CLI commands (start, status, resume) | FEATURE-WORKFLOW |

### Dependency Graph

```
              ┌───────────────────┐
              │ FEATURE-DISCOVERY │
              └─────────┬─────────┘
                        │
       ┌────────────────┼────────────────┐
       │                │                │
       ▼                ▼                ▼
┌─────────────┐  ┌──────────────┐  ┌───────────┐
│FEATURE-     │  │FEATURE-BRANCH│  │SPEC-REVIEW│
│PRIORITIZER  │  └──────┬───────┘  └─────┬─────┘
└─────────────┘         │                │
                        └────────┬───────┘
                                 ▼
                    ┌────────────────────┐
                    │  FEATURE-WORKFLOW  │
                    └──────────┬─────────┘
                               │
                               ▼
                       ┌─────────────┐
                       │ FEATURE-CLI │
                       └─────────────┘
```

### Implementation Order

1. **FEATURE-DISCOVERY** (foundational, no dependencies)
2. **FEATURE-PRIORITIZER** + **FEATURE-BRANCH** + **SPEC-REVIEW** (parallel, depend on FEATURE-DISCOVERY)
3. **FEATURE-WORKFLOW** (depends on FEATURE-DISCOVERY, FEATURE-BRANCH, SPEC-REVIEW)
4. **FEATURE-CLI** (depends on FEATURE-WORKFLOW)

## Daemon Architecture Specs (v0.5)

These specs convert Charlotte from CLI-based to daemon-based architecture:

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[DAEMON-DB](DAEMON-DB.md)** | SQLite database layer for persistent state storage | - |
| **[DAEMON-GRPC](DAEMON-GRPC.md)** | gRPC interface and protocol buffer definitions | DAEMON-DB |
| **[DAEMON-CORE](DAEMON-CORE.md)** | Daemon process lifecycle, job manager, resume logic | DAEMON-DB, DAEMON-GRPC |
| **[DAEMON-CLIENT](DAEMON-CLIENT.md)** | gRPC client wrapper for CLI communication | DAEMON-GRPC |
| **[DAEMON-CLI](DAEMON-CLI.md)** | CLI commands for daemon management and job control | DAEMON-CLIENT, DAEMON-CORE |

### Dependency Graph

```
              ┌───────────┐
              │ DAEMON-DB │
              └─────┬─────┘
                    │
                    ▼
             ┌────────────┐
             │DAEMON-GRPC │
             └──────┬─────┘
                    │
       ┌────────────┼────────────┐
       ▼                         ▼
┌─────────────┐           ┌─────────────┐
│DAEMON-CORE  │           │DAEMON-CLIENT│
└──────┬──────┘           └──────┬──────┘
       │                         │
       └────────────┬────────────┘
                    ▼
              ┌───────────┐
              │DAEMON-CLI │
              └───────────┘
```

### Implementation Order

1. **DAEMON-DB** (foundational, no dependencies)
2. **DAEMON-GRPC** (depends on DAEMON-DB)
3. **DAEMON-CORE** + **DAEMON-CLIENT** (parallel, depend on DAEMON-GRPC)
4. **DAEMON-CLI** (depends on DAEMON-CORE and DAEMON-CLIENT)

## Container Isolation Specs (v0.6)

These specs enable isolated job execution in Docker/Podman containers:

| Spec | Description | Dependencies |
|------|-------------|--------------|
| **[CONTAINER-MANAGER](CONTAINER-MANAGER.md)** | Container lifecycle management using Docker/Podman CLI | - |
| **[JSON-EVENTS](JSON-EVENTS.md)** | JSON event emitter and parser for container stdout communication | EVENTS |
| **[CONTAINER-IMAGE](CONTAINER-IMAGE.md)** | Dockerfile and build script for choo container image | - |
| **[CONTAINER-DAEMON](CONTAINER-DAEMON.md)** | Daemon integration for dispatching jobs to containers and bridging events | DAEMON-CORE, CONTAINER-MANAGER, JSON-EVENTS |

### Dependency Graph

```
                                    ┌────────┐
                                    │ EVENTS │
                                    └───┬────┘
                                        │
┌───────────────────┐  ┌────────────────┼────────────────┐
│ CONTAINER-MANAGER │  │                ▼                │
└─────────┬─────────┘  │         ┌─────────────┐         │
          │            │         │ JSON-EVENTS │         │
          │            │         └──────┬──────┘         │
          │            │                │                │
          │    ┌───────┴────────┐       │                │
          │    │  DAEMON-CORE   │       │                │
          │    └───────┬────────┘       │                │
          │            │                │                │
          └────────────┼────────────────┘                │
                       │                                 │
                       ▼                                 │
             ┌──────────────────┐                        │
             │ CONTAINER-DAEMON │                        │
             └──────────────────┘                        │
                                                         │
┌─────────────────┐                                      │
│ CONTAINER-IMAGE │  (independent, build-time only)      │
└─────────────────┘
```

### Implementation Order

1. **CONTAINER-MANAGER** + **JSON-EVENTS** + **CONTAINER-IMAGE** (parallel, minimal dependencies)
2. **CONTAINER-DAEMON** (depends on DAEMON-CORE, CONTAINER-MANAGER, JSON-EVENTS)

### Task Breakdown

Each spec has been decomposed into atomic tasks in `specs/tasks/`:

| Unit | Tasks | Description |
|------|-------|-------------|
| `container-manager/` | 3 | Core types, runtime detection, CLI manager implementation |
| `json-events/` | 4 | Wire types, JSON emitter, JSON reader, TTY detection |
| `container-image/` | 2 | Dockerfile, build script |
| `container-daemon/` | 6 | Config types, log streamer, container job, archive, CLI extensions, orchestrator completion |

**Total: 15 atomic tasks**

#### Unit Dependency Graph

```
container-manager ──┐
                    ├──► container-daemon
json-events ────────┘

container-image (independent, build-time only)
```

#### Task Execution Order

**Phase 1** (parallel):
- `container-manager/01-core-types.md`
- `container-manager/02-runtime-detection.md`
- `json-events/01-wire-types.md`
- `container-image/01-dockerfile.md`

**Phase 2** (depends on phase 1):
- `container-manager/03-cli-manager.md`
- `json-events/02-json-emitter.md`
- `json-events/03-json-reader.md`
- `container-image/02-build-script.md`

**Phase 3** (depends on phase 2):
- `json-events/04-tty-detection.md`
- `container-daemon/01-config-types.md`
- `container-daemon/04-archive.md`

**Phase 4** (depends on phase 3):
- `container-daemon/02-log-streamer.md`
- `container-daemon/05-cli-extensions.md`
- `container-daemon/06-orchestrator-complete.md`

**Phase 5** (final integration):
- `container-daemon/03-container-job.md`
