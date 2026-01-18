# Ralph

![Ralph](conductor-ralph.png)

Ralph is a parallel development orchestrator that executes LLM-driven tasks autonomously. It discovers units of work, runs them concurrently in isolated git worktrees, and manages the full PR lifecycle from creation through merge.

## What It Does

- **Discovers** implementation units from `specs/tasks/` directories
- **Parallelizes** independent units across git worktrees
- **Executes** tasks via Claude CLI with backpressure validation
- **Manages PRs** including review polling, feedback addressing, and merge coordination

## Core Concepts

**Units** are collections of related tasks that implement a design spec. Units declare dependencies on other units and run in parallel when independent.

**Tasks** are atomic pieces of work within a unit. Each task has a backpressure command that must pass before the task is considered complete.

**Backpressure** gates progress on correctness. Tasks only complete when their validation passes. Baseline checks (formatting, linting) run at the end of each unit.

## Architecture

Ralph uses an event-driven design with file-based state. All progress is tracked in YAML frontmatter of spec files—no external database required. The event bus enables future UIs (TUI, MCP, web) without architectural changes.

## License

MIT
