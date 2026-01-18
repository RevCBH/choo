# Choo Agent Guidelines

## Project Overview

**Choo** (formerly ralph-orch) is a Go application that orchestrates parallel development task execution with PR lifecycle management.

## Important Context

### Worktree Awareness

**You are likely running in a git worktree, not the main repository clone.**

- **Prefer relative paths** over absolute paths when referencing files in the repo
- **Be suspicious of absolute paths** - they may point to a different worktree or the main repo
- When generating code that references repo files, use paths relative to the repo root (e.g., `specs/CONFIG.md` not `/Users/.../worktrees/unit-x/specs/CONFIG.md`)
- The worktree root is your current working directory

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Binary name | `choo` | Short, memorable, distinct from ralph.sh |
| Config file | `.choo.yaml` | Matches binary name |
| Event type name | `Bus` | Simpler than `EventBus` |
| Worktree location | `.ralph/worktrees/` | Project-local, gitignored |

## Specifications

**IMPORTANT:** Before implementing any feature, consult the specifications in `specs/`.

- **Assume NOT implemented.** Specs describe planned features that may not yet exist in the codebase.
- **Check the codebase first.** Before concluding something is or isn't implemented, search the actual code. Specs describe intent; code describes reality.
- **Use specs as guidance.** When implementing a feature, follow the design patterns, types, and architecture defined in the relevant spec.

### Unit Specs

| Spec | Purpose |
|------|---------|
| `specs/CLI.md` | Cobra commands, flags, status display |
| `specs/CONFIG.md` | YAML config loading, validation |
| `specs/DISCOVERY.md` | Frontmatter parsing, Unit/Task types |
| `specs/EVENTS.md` | Event bus, handlers, event types |
| `specs/GIT.md` | Worktree management, merge serialization |
| `specs/GITHUB.md` | PR lifecycle, emoji review state machine |
| `specs/SCHEDULER.md` | Dependency graph, dispatch |
| `specs/WORKER.md` | Ralph loop, Claude CLI invocation |

## Ralph Workflow

This project uses a Ralph-style autonomous workflow. See `docs/HOW-TO-RALPH.md` for details.

### Key Principles

1. **Single-concern specs** - Each spec should have ONE clear purpose (no "and")
2. **Atomic tasks** - Implementation plan breaks specs into tasks completable in one iteration
3. **Backpressure** - Tests and checks that must pass before a task is complete
4. **Fresh context** - Each iteration starts clean; read the plan, pick one task, implement, validate, commit

### Running Choo

```bash
# Execute all units in parallel
choo run specs/tasks/

# Limit parallelism
choo run -p 2 specs/tasks/

# Single unit mode (like ralph.sh)
choo run --unit app-shell specs/tasks/

# Dry run (show execution plan)
choo run -n specs/tasks/

# Check status
choo status specs/tasks/

# Resume after interruption
choo resume specs/tasks/

# Clean up worktrees
choo cleanup specs/tasks/
```

### Skills

Custom skills are defined in `.claude/skills/`:

| Skill | Purpose |
|-------|---------|
| `spec.md` | Write technical specifications following project conventions |
| `ralph-prep.md` | Decompose design specs into atomic, Ralph-executable task specs |
| `spec-validate.md` | Validate specs for consistency after parallel generation |

### Agents

Subagent definitions in `.claude/agents/`:

| Agent | Purpose |
|-------|---------|
| `spec-generator.md` | Generate unit specs, writes directly to files |
| `spec-validator.md` | Validate parallel-generated specs for consistency |
| `task-generator.md` | Generate task specs for units, writes directly to files |

## Architecture

### Package Structure

```
choo/
├── cmd/
│   └── choo/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/                  # Cobra commands, display
│   ├── config/               # Config loading
│   ├── discovery/            # Unit/Task discovery
│   ├── events/               # Event bus
│   ├── git/                  # Worktree management
│   ├── github/               # PR lifecycle
│   ├── scheduler/            # Dependency graph
│   ├── worker/               # Task execution
│   └── claude/               # Claude CLI wrapper
└── specs/
    └── tasks/                # Task specs per unit
```

### Key Types

```go
// Unit of work - directory of related tasks
type Unit struct {
    ID        string      // directory name
    DependsOn []string    // other unit IDs
    Status    UnitStatus  // pending, in_progress, complete, failed
    Tasks     []*Task
}

// Individual task within a unit
type Task struct {
    Number     int         // sequential, 1-indexed
    Status     TaskStatus  // pending, in_progress, complete, failed
    DependsOn  []int       // task numbers within unit
    Backpressure string    // validation command
}
```

## Commands

### Building

```bash
go build -o choo ./cmd/choo
```

### Testing

```bash
# All tests
go test ./...

# Specific package
go test ./internal/discovery/

# With verbose output
go test -v ./...
```

### Linting

```bash
# Format
go fmt ./...

# Vet
go vet ./...

# Golangci-lint (if configured)
golangci-lint run
```

## Code Style

### Go

- **Formatting:** Use `gofmt` defaults
- **Errors:** Wrap with context using `fmt.Errorf("context: %w", err)`
- **Naming:** snake_case for files, PascalCase for exported types
- **Imports:** Group std, external, then internal packages

### Comments

Prefer self-documenting code. Add comments only when:
- Logic is non-obvious
- There's a gotcha or platform quirk
- Explaining WHY, not WHAT

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `GITHUB_TOKEN` | GitHub API authentication | Required |
| `RALPH_CLAUDE_CMD` | Claude CLI binary path | `claude` |
| `RALPH_WORKTREE_BASE` | Worktree directory | `.ralph/worktrees/` |
| `RALPH_LOG_LEVEL` | Log verbosity | `info` |

## Common Issues

### "Claude command not found"

Ensure `claude` CLI is installed and in PATH, or set `RALPH_CLAUDE_CMD`.

### "GitHub token missing"

Set `GITHUB_TOKEN` environment variable for PR operations.

### "Worktree already exists"

Run `choo cleanup` to remove orphaned worktrees.

## Before Submitting Code

1. **Format:** `go fmt ./...`
2. **Vet:** `go vet ./...`
3. **Test:** `go test ./...`
4. **Specs match:** Implementation follows relevant spec
