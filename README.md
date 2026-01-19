# choo

![choo](conductor-ralph.png)

choo is a parallel development orchestrator that executes LLM-driven tasks autonomously. It discovers units of work, runs them concurrently in isolated git worktrees, and manages the full PR lifecycle from creation through merge.

## What It Does

- **Discovers** implementation units from `specs/tasks/` directories
- **Parallelizes** independent units across git worktrees
- **Executes** tasks via Claude CLI with backpressure validation
- **Manages PRs** including review polling, feedback addressing, and merge coordination

## Core Concepts

**Units** are collections of related tasks that implement a design spec. Units declare dependencies on other units and run in parallel when independent.

**Tasks** are atomic pieces of work within a unit. Each task has a backpressure command that must pass before the task is considered complete.

**Backpressure** gates progress on correctness. Tasks only complete when their validation passes. Baseline checks (formatting, linting) run at the end of each unit.

## Building and Setup

### Prerequisites

- Go 1.23.0 or later
- Git
- Claude CLI (for task execution)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/anthropics/choo.git
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

### Installation

After building, you can either:

1. Run the binary directly from the build location:
   ```bash
   ./choo --help
   ```

2. Move it to a location in your PATH:
   ```bash
   sudo mv choo /usr/local/bin/
   ```

3. Or use `go install`:
   ```bash
   go install ./cmd/choo
   ```

### Running choo

Once installed, you can run choo commands:

```bash
# Show version
choo version

# Run the orchestrator
choo run

# For more options
choo --help
```

## Architecture

choo uses an event-driven design with file-based state. All progress is tracked in YAML frontmatter of spec files—no external database required. The event bus enables future UIs (TUI, MCP, web) without architectural changes.

## License

MIT
