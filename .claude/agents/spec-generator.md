# Spec Generator Agent

**Type**: `spec-generator`

**Description**: Generates detailed unit specs and writes them directly to files. This agent writes to the filesystem to avoid returning large spec text through the context window.

**When to use**:
- When generating multiple unit specs in parallel
- To avoid context compaction from large spec text returns
- After PRD is finalized, before ralph-prep

---

## Invocation

```
Task tool with subagent_type: "general-purpose"

Prompt: "Generate the {UNIT} spec following the spec-generator agent instructions.

Unit: {UNIT}
PRD: docs/MVP DESIGN SPEC.md
Output: specs/{UNIT}.md
Reference format: specs/CLI.md

Section focus: PRD §{X.Y} - {Section Name}
"
```

---

## Agent Instructions

When you receive a spec generation request:

### 1. Read Required Context

```
Read these files:
1. docs/MVP DESIGN SPEC.md - the PRD (focus on the specified section)
2. specs/CLI.md OR specs/CONFIG.md - format reference
3. Any existing related specs for type consistency
```

### 2. Generate Spec Following This Structure

```markdown
# {UNIT} — {Full Title}

## Overview
- 2-3 paragraph description
- ASCII diagram showing component's role in system
- Key responsibilities

## Requirements

### Functional Requirements
1. Numbered list of capabilities

### Performance Requirements
| Metric | Target |
|--------|--------|
| ... | ... |

### Constraints
- Bullet list of limitations and dependencies

## Design

### Module Structure
```
internal/{unit}/
├── file.go      # Description
└── ...
```

### Core Types
```go
// Fully documented Go types with comments
type Foo struct {
    Field string `yaml:"field"`
}
```

### API Surface
```go
// Public functions with doc comments
func NewFoo() *Foo
func (f *Foo) Method() error
```

### (Component-specific sections as needed)
- State machines, flows, diagrams

## Implementation Notes
- Key algorithms
- Edge cases
- Integration points

## Testing Strategy

### Unit Tests
```go
// Example test code
func TestFoo(t *testing.T) { ... }
```

### Integration Tests
| Scenario | Setup |
|----------|-------|
| ... | ... |

### Manual Testing
- [ ] Checklist items

## Design Decisions
- Why X over Y?
- Alternatives considered

## Future Enhancements
1. Numbered list

## References
- Links to PRD sections
- External docs
```

### 3. Write the Spec File

**CRITICAL**: Write the spec directly to `specs/{UNIT}.md` using the Write tool.

Do NOT return the spec content in your response. Instead:
1. Write the file
2. Return a brief summary: "Wrote specs/{UNIT}.md ({N} lines) covering: {key topics}"

---

## Unit-Specific Guidance

### DISCOVERY
- PRD §4.1 Discovery Phase
- Focus: YAML frontmatter parsing, Unit/Task types, validation, glob patterns
- Exports: Unit, Task, UnitStatus, TaskStatus, Discover()

### SCHEDULER
- PRD §4.2 Scheduling
- Focus: Dependency graph, unit state machine, ready queue, dispatch
- Imports: Unit, Task from discovery
- Exports: Scheduler, Schedule(), Dispatch()

### WORKER
- PRD §4.3 Worker Flow
- Focus: Ralph loop, Claude CLI invocation, backpressure, task prompts
- Imports: Unit, Task, Event types
- Key: ALWAYS use Claude Code subprocess, NEVER API

### GIT
- PRD §4.3 Phase 1, §4.5 Merge Serialization
- Focus: Worktree management, branch naming (Claude haiku), merge mutex
- Conditional setup commands, conflict resolution (3 attempts)
- Branch cleanup after batch merged

### GITHUB
- PRD §4.4 PR Lifecycle
- Focus: PR creation (delegated to Claude), emoji review state machine
- Polling: 👀 → 👍, 2h timeout default
- Squash merge strategy

### EVENTS
- PRD §4.6 Event System
- Focus: EventBus, event types, handlers, fan-out
- All unit/task state changes emit events

### CLI
- PRD §8 CLI Interface
- Focus: Cobra commands, flags, status display, signal handling
- Commands: run, status, resume, cleanup, version

### CONFIG
- PRD §9 Configuration
- Focus: .choo.yaml loading, env overrides, validation, defaults
- Auto-detect GitHub owner/repo from git remote

---

## Example Prompts

### Generate single spec
```
Generate the SCHEDULER spec following the spec-generator agent instructions.

Unit: SCHEDULER
PRD: docs/MVP DESIGN SPEC.md
Output: specs/SCHEDULER.md
Reference format: specs/CLI.md

Section focus: PRD §4.2 - Scheduling
```

### Generate with context awareness
```
Generate the WORKER spec following the spec-generator agent instructions.

Unit: WORKER
PRD: docs/MVP DESIGN SPEC.md
Output: specs/WORKER.md
Reference format: specs/CLI.md

Section focus: PRD §4.3 - Worker Flow

Important context:
- Claude invocation MUST use subprocess (claude CLI), NEVER the API
- Backpressure commands are per-task
- Baseline checks are per-unit (Phase 2.5)
- Agent-driven task selection: Claude chooses from ready queue
```

---

## Output Format

Your response should be brief:

```
Wrote specs/{UNIT}.md (847 lines)

Coverage:
- Core types: Unit, Task, UnitStatus, TaskStatus
- Functions: Discover(), ParseFrontmatter(), ValidateUnit()
- Testing: 12 unit test cases, 4 integration scenarios

Key design decisions:
- YAML over TOML for frontmatter
- Glob-based file discovery
- Strict validation at parse time
```

This keeps context usage minimal while confirming successful generation.

---

## Parallel Generation

To generate all specs in parallel, launch 8 agents simultaneously:

```
// From main context, call Task tool 8 times in parallel:

Task(subagent_type="general-purpose", prompt="Generate DISCOVERY spec...")
Task(subagent_type="general-purpose", prompt="Generate SCHEDULER spec...")
Task(subagent_type="general-purpose", prompt="Generate WORKER spec...")
Task(subagent_type="general-purpose", prompt="Generate GIT spec...")
Task(subagent_type="general-purpose", prompt="Generate GITHUB spec...")
Task(subagent_type="general-purpose", prompt="Generate EVENTS spec...")
Task(subagent_type="general-purpose", prompt="Generate CLI spec...")
Task(subagent_type="general-purpose", prompt="Generate CONFIG spec...")
```

Each agent:
1. Reads PRD and format reference
2. Writes directly to specs/{UNIT}.md
3. Returns brief confirmation

Then run spec-validator to check consistency.
