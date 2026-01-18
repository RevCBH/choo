# Task Generator Agent

**Type**: `task-generator`

**Description**: Generates atomic task specs from a unit spec and writes them directly to files. This agent writes to the filesystem to avoid returning large task text through the context window.

**When to use**:
- When generating task breakdowns for multiple unit specs in parallel
- To avoid context compaction from large task text returns
- After unit specs are validated, before implementation

---

## Invocation

```
Task tool with subagent_type: "general-purpose"

Prompt: "Generate tasks for the {UNIT} spec following the task-generator agent instructions.

Unit: {UNIT}
Spec: specs/{UNIT}.md
Output: specs/tasks/{unit-lowercase}/
Skill reference: .claude/skills/ralph-prep.md
"
```

---

## Agent Instructions

When you receive a task generation request:

### 1. Read Required Context

```
Read these files:
1. specs/{UNIT}.md - the unit spec to decompose
2. .claude/skills/ralph-prep.md - decomposition rules and templates
3. docs/MVP DESIGN SPEC.md - for cross-unit context if needed
```

### 2. Identify Decomposition Strategy

Analyze the spec and identify:
- **Concerns**: Types, core logic, API layer, tests
- **Boundaries**: Natural file/module boundaries
- **Dependencies**: What must exist before what

### 3. Generate Files

Create the following directory and files:

```
specs/tasks/{unit-lowercase}/
├── IMPLEMENTATION_PLAN.md
├── 01-{first-task}.md
├── 02-{second-task}.md
└── ...
```

### 4. IMPLEMENTATION_PLAN.md Template

```markdown
---
unit: {unit-lowercase}
depends_on: []
---

# {UNIT} Implementation Plan

## Overview

{Brief description of what this unit implements and decomposition strategy}

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-{task}.md | {description} | None | {validation command} |
| 2 | 02-{task}.md | {description} | #1 | {validation command} |
...

## Baseline Checks

```bash
go fmt ./internal/{package}/... && go vet ./internal/{package}/...
```

## Completion Criteria

- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and approved

## Reference

- Design spec: `specs/{UNIT}.md`
```

### 5. Task Spec Template

Each task spec MUST have this structure:

```markdown
---
task: {N}
status: pending
backpressure: "{exact validation command}"
depends_on: [{task numbers this depends on}]
---

# {Task Name}

**Parent spec**: `specs/{UNIT}.md`
**Task**: #{N} of {M} in implementation plan

## Objective

{One sentence describing what this task produces}

## Dependencies

### Task Dependencies (within this unit)
- {Task #X must be complete for reason}

### Package Dependencies
- {external packages needed}

## Deliverables

### Files to Create/Modify

```
internal/{package}/
└── {file}.go    # {CREATE|MODIFY}: {description}
```

### Types to Implement

```go
// {Type description}
type {Name} struct {
    {fields}
}
```

### Functions to Implement

```go
// {Function description}
func {Name}({params}) ({returns}) {
    // {implementation notes}
}
```

## Backpressure

### Validation Command

```bash
{exact command}
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `Test{Name}` | {specific assertion} |

## NOT In Scope

- {things deferred to other tasks}
```

### 6. Write Files Directly

**CRITICAL**: Write all files directly using the Write tool.

Do NOT return the task content in your response. Instead:
1. Create the directory structure
2. Write IMPLEMENTATION_PLAN.md
3. Write each task spec file
4. Return a brief summary

---

## Unit-Specific Guidance

### DISCOVERY
- Task 1: Core types (Unit, Task, UnitStatus, TaskStatus)
- Task 2: Frontmatter parsing (YAML)
- Task 3: Directory discovery (glob)
- Task 4: Validation logic
- Backpressure: `go test ./internal/discovery/...`

### SCHEDULER
- Task 1: Graph types and construction
- Task 2: State machine and transitions
- Task 3: Ready queue and dispatch
- Task 4: Failure propagation
- Backpressure: `go test ./internal/scheduler/...`

### WORKER
- Task 1: Worker types and config
- Task 2: Claude CLI invocation
- Task 3: Task prompt construction
- Task 4: Backpressure runner
- Task 5: Baseline checks
- Task 6: Ralph loop orchestration
- Backpressure: `go test ./internal/worker/...`

### GIT
- Task 1: Worktree management
- Task 2: Branch naming (Claude haiku)
- Task 3: Commit operations
- Task 4: Merge serialization
- Task 5: Conflict resolution
- Backpressure: `go test ./internal/git/...`

### GITHUB
- Task 1: PR client types
- Task 2: PR creation (Claude delegation)
- Task 3: Review polling (emoji state machine)
- Task 4: Merge operations
- Backpressure: `go test ./internal/github/...`

### EVENTS
- Task 1: Event types and bus
- Task 2: Handler registration
- Task 3: Built-in handlers (log, state)
- Backpressure: `go test ./internal/events/...`

### CLI
- Task 1: Root command and app struct
- Task 2: Run command
- Task 3: Status command
- Task 4: Resume and cleanup commands
- Task 5: Signal handling
- Task 6: Display formatting
- Backpressure: `go test ./internal/cli/...`

### CONFIG
- Task 1: Config types
- Task 2: YAML loading
- Task 3: Environment overrides
- Task 4: Validation
- Task 5: GitHub auto-detection
- Backpressure: `go test ./internal/config/...`

---

## Output Format

Your response should be brief:

```
Created specs/tasks/{unit}/

Files written:
- IMPLEMENTATION_PLAN.md
- 01-{task}.md
- 02-{task}.md
- ... (N total tasks)

Task overview:
| # | Name | Backpressure |
|---|------|--------------|
| 1 | {name} | {command} |
| 2 | {name} | {command} |
...
```

This keeps context usage minimal while confirming successful generation.

---

## Parallel Generation

To generate tasks for all specs in parallel, launch 8 agents simultaneously:

```
// From main context, call Task tool 8 times in parallel:

Task(subagent_type="general-purpose", prompt="Generate tasks for DISCOVERY...")
Task(subagent_type="general-purpose", prompt="Generate tasks for SCHEDULER...")
Task(subagent_type="general-purpose", prompt="Generate tasks for WORKER...")
Task(subagent_type="general-purpose", prompt="Generate tasks for GIT...")
Task(subagent_type="general-purpose", prompt="Generate tasks for GITHUB...")
Task(subagent_type="general-purpose", prompt="Generate tasks for EVENTS...")
Task(subagent_type="general-purpose", prompt="Generate tasks for CLI...")
Task(subagent_type="general-purpose", prompt="Generate tasks for CONFIG...")
```

Each agent:
1. Reads the unit spec
2. Decomposes into atomic tasks
3. Writes directly to specs/tasks/{unit}/
4. Returns brief confirmation

---

## Quality Checklist

Before finishing, ensure:

- [ ] IMPLEMENTATION_PLAN.md has valid YAML frontmatter
- [ ] All task specs have valid YAML frontmatter with task number, status, backpressure, depends_on
- [ ] Backpressure commands are exact and executable
- [ ] Task numbers are sequential starting from 1
- [ ] Dependencies form a valid DAG (no cycles)
- [ ] Each task is atomic (one primary deliverable)
- [ ] Each task is ~80-150 lines
- [ ] No task title contains "and"
