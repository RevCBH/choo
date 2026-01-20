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

## Feature Workflow Specs (v0.4)

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
