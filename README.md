# choo

![choo](conductor-ralph.png)

choo is a parallel development orchestrator that executes LLM-driven tasks autonomously. It discovers units of work, runs them concurrently in isolated git worktrees, and manages the full PR lifecycle from creation through merge.

## What It Does

- **Discovers** implementation units from `specs/tasks/` directories
- **Parallelizes** independent units across git worktrees
- **Executes** tasks via Claude CLI or Codex CLI with backpressure validation
- **Manages PRs** including review polling, feedback addressing, and merge coordination
- **Feature workflows** for end-to-end PRD-driven development

## Core Concepts

**Units** are collections of related tasks that implement a design spec. Units declare dependencies on other units and run in parallel when independent.

**Tasks** are atomic pieces of work within a unit. Each task has a backpressure command that must pass before the task is considered complete.

**Backpressure** gates progress on correctness. Tasks only complete when their validation passes. Baseline checks (formatting, linting) run at the end of each unit.

**Providers** are LLM CLI tools that execute tasks. choo supports Claude CLI (default) and OpenAI Codex CLI, with flexible configuration at global, per-run, and per-unit levels.

## Building and Setup

### Prerequisites

- Go 1.23.0 or later
- Git
- Claude CLI and/or Codex CLI (for task execution)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/RevCBH/choo.git
cd choo

# Build the binary
go build -o choo ./cmd/choo

# Optional: Install to your PATH
go install ./cmd/choo
```

### Build with Version Information

To build with version information embedded:

```bash
go build -ldflags="-X 'main.version=1.0.0' -X 'main.commit=$(git rev-parse HEAD)' -X 'main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o choo ./cmd/choo
```

## Usage

### Running the Orchestrator

```bash
# Run with interactive TUI
choo run

# Preview execution plan without running
choo run --dry-run

# Run a single unit
choo run --unit my-feature

# Run with custom parallelism
choo run --parallelism 8

# Use Codex as the default provider
choo run --provider codex

# Force all tasks to use a specific provider (overrides per-unit settings)
choo run --force-task-provider codex
```

### Feature Workflows

Feature mode provides end-to-end PRD-driven development:

```bash
# Start a new feature from a PRD
choo feature start my-prd-id

# Check feature status
choo feature status

# Resume a blocked feature
choo feature resume my-prd-id
```

### Other Commands

```bash
# Show current orchestration status
choo status

# Show the prompt that would be sent to the LLM for a unit
choo prompt <unit-id>

# Clean up worktrees
choo cleanup

# Archive completed specs
choo archive

# Start web monitoring server
choo web

# Show version
choo version
```

## Configuration

### Config File (`.choo.yaml`)

```yaml
# Target branch for PRs (default: main)
target_branch: main

# Maximum concurrent units (default: 4)
parallelism: 4

# Provider configuration
provider:
  type: claude  # or "codex"
  providers:
    claude:
      command: claude
    codex:
      command: codex

# Claude-specific settings (legacy, still supported)
claude:
  command: claude
  max_turns: 50

# Baseline checks run after each unit completes
baseline_checks:
  - name: build
    command: go build -v ./...
  - name: test
    command: go test -v ./...
  - name: lint
    command: golangci-lint run

# Worktree settings
worktree:
  base_path: .ralph/worktrees
  setup:
    - command: npm install
      if: package.json
  teardown:
    - command: npm run clean
      if: package.json

# Feature workflow settings
feature:
  prd_dir: docs/prds
  specs_dir: specs
  branch_prefix: feature/
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `RALPH_PROVIDER` | Default provider type (`claude` or `codex`) |
| `RALPH_CLAUDE_CMD` | Override Claude CLI binary path |
| `RALPH_CODEX_CMD` | Override Codex CLI binary path |
| `RALPH_WORKTREE_BASE` | Override worktree base directory |
| `RALPH_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |

### Provider Selection Precedence

For task execution, providers are selected in this order (highest to lowest):

1. `--force-task-provider` CLI flag (overrides everything)
2. Per-unit frontmatter (`provider: codex`)
3. `--provider` CLI flag
4. `RALPH_PROVIDER` environment variable
5. `.choo.yaml` `provider.type`
6. Default: `claude`

### Per-Unit Provider Override

Units can specify their preferred provider in frontmatter:

```yaml
---
unit: my-feature
provider: codex
depends_on:
  - base-types
---
```

## Architecture

choo uses an event-driven design with file-based state. All progress is tracked in YAML frontmatter of spec files—no external database required.

Key components:
- **Orchestrator** - Coordinates unit execution and manages lifecycle
- **Scheduler** - Handles dependency resolution and parallel dispatch
- **Worker Pool** - Executes units in isolated git worktrees
- **Provider Interface** - Abstracts LLM CLI invocation (Claude, Codex)
- **Event Bus** - Enables real-time UI updates (TUI, web)

The event bus enables multiple UIs without architectural changes:
- **TUI** - Interactive terminal interface (default when stdout is a TTY)
- **Web UI** - Real-time monitoring via `choo web`
- **Summary mode** - Non-interactive output with `--no-tui`

## Task Specification Format

Tasks are defined in markdown files with YAML frontmatter:

```markdown
---
number: 1
title: Implement user authentication
status: pending
backpressure: go test ./internal/auth/...
depends_on: []
---

## Description

Implement JWT-based authentication...

## Acceptance Criteria

- [ ] Users can log in with email/password
- [ ] JWT tokens are properly validated
```

## License

MIT
