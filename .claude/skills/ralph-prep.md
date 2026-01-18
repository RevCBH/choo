# Ralph Preparation Skill

**Description**: Decompose a design spec into atomic, Ralph-executable task specs with an implementation plan (unit).

**When to use**: When a spec has been written using the spec skill and needs to be prepared for autonomous Ralph-loop execution. The output is a "unit" - a collection of tasks with explicit dependencies that implement a single spec. Task numbers are identifiers, not execution order; the `depends_on` field determines which tasks can run.

---

## What This Skill Does

Takes a **design spec** (comprehensive, multi-concern document) and produces:

1. **Atomic task specs** — Single-concern specs that can be implemented in one Ralph iteration
2. **Implementation plan** — Ordered task list with dependencies
3. **Backpressure criteria** — Tests/checks that validate each task

---

## Core Principles

### From HOW-TO-RALPH.md

> "Break the project into **single-concern specs** (no 'and')."
> "Each task clearly 'done-able' in one iteration"
> "Fresh LLM context every iteration"

### Decomposition Rules

1. **One file/module per task spec** — If implementing requires creating multiple files, it's too big
2. **No "and" in the title** — "Audio Converter" not "Audio Converter and Cache"
3. **Clear doneness** — The task is complete when specific tests pass
4. **Explicit dependencies** — State what must exist before this task can start
5. **~100-150 lines per atomic spec** — Enough detail to implement, short enough to hold in context

---

## Input Format

The input is a design spec following the spec writing skill template. Signs that a spec needs Ralph-prep:

- Multiple modules in "Module Structure" section
- Multiple Tauri commands
- Both backend and frontend code
- Multiple state machines or data flows
- >300 lines total

---

## Parsing Spec Dependencies

Design specs include a `## Dependencies` section with a YAML block that declares:

```yaml
spec_dependencies:
  - PROJECT-SETUP    # Must be implemented first
  - CONFIG           # Provides types we import

type_imports:
  - CONFIG: [AppConfig, AudioConfig]    # Types from other specs
  - DATABASE: [Card, CardRepo]

type_exports:
  - AudioInput       # Types this spec produces
  - WhisperAudio
```

### Reading Dependencies

When Ralph-prepping a spec:

1. **Parse the YAML block** in the `## Dependencies` section
2. **Check `spec_dependencies`** — These specs must be fully implemented before starting this one
3. **Check `type_imports`** — Ensure these types exist (from already-implemented specs)
4. **Note `type_exports`** — These become available to downstream specs

### Dependency Validation

Before generating an implementation plan:

```
✓ All spec_dependencies are implemented?
✓ All type_imports are available from implemented specs?
✓ No circular dependencies?
```

If validation fails, **stop and inform the user** which specs must be completed first.

### Cross-Spec Dependencies in Task Specs

When a design spec depends on another spec, atomic task specs should:

1. **Reference the external spec** in the Dependencies section
2. **Import types explicitly** with their source spec
3. **Not re-implement** types from other specs

Example atomic task spec:

```markdown
## Dependencies

### External Specs (must be implemented)
- CONFIG — provides `AudioConfig`
- DATABASE — provides `Card`, `CardRepo`

### Task Dependencies (within this workset)
- Task #1 must be complete (provides: `AudioInput`)

### Crate Dependencies
- `whisper-rs` in Cargo.toml
```

### Implementation Order Across Specs

When multiple specs need Ralph-prep, process them in dependency order:

```
PROJECT-SETUP     ← No dependencies, do first
    ↓
CONFIG            ← Depends on PROJECT-SETUP
DATABASE          ← Depends on PROJECT-SETUP
MODEL-DOWNLOAD    ← Depends on PROJECT-SETUP, CONFIG
    ↓
AUDIO-PIPELINE    ← Depends on PROJECT-SETUP, CONFIG
STT-ENGINE        ← Depends on MODEL-DOWNLOAD, AUDIO-PIPELINE
    ↓
... and so on
```

To see the full dependency graph, check `specs/README.md`.

---

## Output Structure

### Directory Structure

```
/specs/
├── ORIGINAL-SPEC.md              # Design spec (serves as PRD for this unit)
├── tasks/
│   └── original-spec/            # Unit directory (matches spec name, lowercase)
│       ├── IMPLEMENTATION_PLAN.md
│       ├── 01-core-types.md
│       ├── 02-first-module.md
│       ├── 03-second-module.md
│       └── ...
```

### Implementation Plan Template

The IMPLEMENTATION_PLAN.md file has YAML frontmatter that the orchestrator uses:

```markdown
---
# === Author-provided (required) ===
unit: original-spec                           # Unit ID (directory name)
depends_on: []                                # Other unit IDs this unit depends on

# === Orchestrator-managed (auto-populated at runtime) ===
# orch_status: pending
# orch_branch: ralph/original-spec-a1b2c3
# orch_worktree: .ralph/worktrees/original-spec
# orch_pr_number: null
# orch_started_at: null
# orch_completed_at: null
---

# COMPONENT Implementation Plan

## Overview

Brief description of what we're building and the decomposition strategy.

## Task Sequence

| # | Task Spec | Description | Dependencies | Backpressure |
|---|-----------|-------------|--------------|--------------|
| 1 | 01-core-types.md | Define shared types | None | Compiles |
| 2 | 02-module-a.md | Implement X | #1 | Unit tests pass |
| 3 | 03-module-b.md | Implement Y | #1 | Unit tests pass |
| 4 | 04-commands.md | API layer | #2, #3 | Integration test |
| 5 | 05-integration.md | Wire up components | #4 | E2E test |

## Baseline Checks

These checks run at the END of the unit (after all tasks complete):

```bash
# Example baseline checks (project-specific)
go fmt ./... && go vet ./...
# or for Rust: cargo fmt --check && cargo clippy -- -D warnings
# or for TS: pnpm typecheck && pnpm lint
```

## Completion Criteria

All tasks marked complete when:
- [ ] All task backpressure checks pass
- [ ] Baseline checks pass
- [ ] PR created and merged

## Reference

- Design spec: `/specs/ORIGINAL-SPEC.md`
```

### Atomic Task Spec Template

Task specs MUST have YAML frontmatter for the orchestrator:

```markdown
---
task: 1                                       # Task identifier (NOT execution order)
status: pending                               # pending | in_progress | complete | failed
backpressure: "go test ./internal/discovery/..."  # Validation command
depends_on: []                                # Task IDs this depends on (determines when task can run)
---

# TASK-NAME

**Parent spec**: `/specs/ORIGINAL-SPEC.md`
**Task**: #N of M in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

One sentence describing what this task produces.

## Dependencies

### External Specs (must be implemented)
- CONFIG — provides `AppConfig` (if applicable)

### Task Dependencies (within this unit)
- Task #X must be complete (provides: `SomeType`)

### Package Dependencies
- `gopkg.in/yaml.v3` (for Go)
- `some-crate` in Cargo.toml (for Rust)

## Deliverables

### Files to Create/Modify

```
internal/
└── module/
    └── file.go    # CREATE: Description
```

### Types to Implement

```go
// For Go projects
type Thing struct {
    Field Type `json:"field"`
}
```

```rust
// For Rust projects
pub struct Thing {
    pub field: Type,
}
```

### Functions to Implement

```go
// DoThing does the thing
func DoThing(input Input) (Output, error) {
    // Implementation notes from design spec
}
```

## Backpressure

**CRITICAL**: The backpressure command in frontmatter MUST be executable and specific.

### Validation Command

```bash
go test ./internal/module/... -v
# or: cargo test module::tests --release
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestSpecificBehavior` | `result == expected` |
| `TestEdgeCase` | `err != nil` when input invalid |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `sample.yaml` | `testdata/` | Valid input fixture |

### CI Compatibility

- [ ] No external API keys required
- [ ] No network access required
- [ ] Runs in <60 seconds

## Implementation Notes

- Key gotchas from the design spec relevant to THIS task only
- Platform considerations if applicable

## NOT In Scope

- Things explicitly deferred to other tasks
- Prevents scope creep during implementation
```

---

## Decomposition Strategy

### Step 1: Identify Concerns

Read the design spec and list distinct implementation units:

1. **Data layer** — Types, structs, enums (often shared)
2. **Core logic** — Algorithms, business rules
3. **Persistence** — Database operations, file I/O
4. **API layer** — CLI commands, HTTP handlers, IPC
5. **Integration** — Wiring components together

### Step 2: Find Natural Boundaries

Look for these signals in the design spec:

| Signal | Boundary Type |
|--------|---------------|
| Separate files in "Module Structure" | File boundary |
| Different packages/modules | Logic boundary |
| Different concerns (parsing vs execution) | Concern boundary |
| "This module handles X" | Responsibility boundary |
| Separate public APIs | API boundary |

### Step 3: Define Dependencies

Build a DAG of task dependencies:

```
Types ─┬─► Module A ─┬─► API Layer ─► Integration
       └─► Module B ─┘
```

**Key concept**: Task numbers are identifiers, not execution order. The `depends_on` field is what matters.

- Tasks with no dependencies (or all dependencies satisfied) are "ready"
- The Ralph agent **chooses** which ready task to work on
- Parallelism happens at the **unit** level via the orchestrator
- Within a unit, one task executes at a time, but the agent picks which one

### Step 4: Define Backpressure vs Baseline Checks

**Backpressure** (per-task): Validates THIS task is complete.
**Baseline Checks** (per-unit): Validates the unit as a whole (linting, formatting, type checking).

| Task Type | Backpressure |
|-----------|--------------|
| Types/Structs | `go build ./...` or `cargo check` |
| Pure functions | Unit tests pass |
| I/O operations | Integration tests pass |
| CLI commands | Command executes without error |
| API handlers | Handler returns expected response |

Baseline checks (formatting, linting) run ONCE at the end of the unit, not per-task.

---

## Backpressure Requirements (MANDATORY)

Every atomic task spec MUST have a complete backpressure section. Vague backpressure ("tests should pass") is not acceptable. Ralph cannot validate tasks without explicit criteria.

### Required Backpressure Elements

Each task's `## Backpressure` section must include ALL of:

#### 1. Validation Command (exact, copy-pasteable)

```markdown
### Validation Command

```bash
cargo test -p koe matching::normalize::tests
```
```

**NOT acceptable:**
- "Run the tests"
- "cargo test" (too broad)
- "Tests should pass"

#### 2. Named Tests That Must Pass

```markdown
### Must Pass

| Test | Assertion |
|------|-----------|
| `test_hiragana_passthrough` | `normalize("たべる") == "たべる"` |
| `test_katakana_to_hiragana` | `normalize("タベル") == "たべる"` |
| `test_punctuation_stripped` | `normalize("こんにちは。") == "こんにちは"` |
```

**NOT acceptable:**
- "Unit tests pass"
- Unnamed tests
- Tests without assertions

#### 3. Test Fixtures (if applicable)

```markdown
### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `mono_16khz.wav` | `fixtures/audio/` | Mono audio passthrough test |
| `stereo_44khz.wav` | `fixtures/audio/` | Stereo→mono conversion test |
| `test_pairs.json` | `fixtures/matching/` | 50 normalization test cases |
```

If fixtures don't exist yet, note them as **BLOCKED** and the task cannot proceed until fixtures are created.

#### 4. CI Compatibility

```markdown
### CI Compatibility

- [ ] No external API keys required
- [ ] No network access required
- [ ] No large files (>10MB) required
- [ ] Runs in <60 seconds

**If blocked:** Requires `ELEVENLABS_API_KEY` — use mock in CI
```

### Backpressure Quality Ratings

When reviewing a task spec, rate its backpressure:

| Rating | Criteria | Ralph Can Execute? |
|--------|----------|-------------------|
| **STRONG** | Exact command + named tests + assertions + fixtures | ✓ Yes |
| **MODERATE** | Command + test names but no fixtures | ⚠ With caveats |
| **WEAK** | Vague or missing | ✗ No — must improve |

**Ralph-prep must produce STRONG backpressure for all tasks.**

### External Dependency Strategies

When a task requires external resources (APIs, models, network):

| Dependency | Strategy | Fixture Location |
|------------|----------|------------------|
| ElevenLabs API | Mock HTTP responses | `fixtures/mocks/elevenlabs/` |
| Claude API | Mock streaming JSON | `fixtures/mocks/claude/` |
| Whisper model | Skip with feature flag | `--features=stt-tests` |
| Network requests | Mock with `wiremock` | In-code setup |

Example fixture reference:

```markdown
### Mock Strategy

For CI without API keys:

```rust
#[cfg(test)]
mod tests {
    use crate::fixtures::load_mock_response;

    #[test]
    fn test_tts_generation() {
        let mock = load_mock_response("fixtures/mocks/elevenlabs/success.json");
        // ... test with mock
    }
}
```
```

### Backpressure Checklist (Use Before Finalizing)

Before marking a task spec complete, verify:

- [ ] Validation command is exact and copy-pasteable
- [ ] All test names are listed explicitly
- [ ] Each test has a specific assertion (not just "passes")
- [ ] Fixtures are listed with paths (or marked as needing creation)
- [ ] CI compatibility is documented
- [ ] External dependencies have mock strategies
- [ ] Command runs in <60 seconds

**If any item fails, the task spec is not ready for Ralph.**

---

## Example Decomposition

### Input: AUDIO-PIPELINE.md (630 lines)

**Concerns identified:**
1. Core types (`AudioInput`, `WhisperAudio`, `RecordingInfo`, `AudioResult`)
2. Converter module (`convert_for_whisper`, `mix_to_mono`, `resample`)
3. Cache module (`AudioCache`, file operations)
4. Tauri commands (`process_recording`, `get_reference_audio`, `list_recordings`)
5. Frontend recorder (`AudioRecorder` class, state machine)

### Output: 5 Atomic Specs

```
/specs/tasks/audio-pipeline/
├── IMPLEMENTATION_PLAN.md
├── 01-audio-types.md           # ~80 lines
├── 02-audio-converter.md       # ~100 lines
├── 03-audio-cache.md           # ~120 lines
├── 04-audio-commands.md        # ~100 lines
└── 05-audio-recorder-ui.md     # ~100 lines
```

**Task sequence:**

| # | Spec | Backpressure |
|---|------|--------------|
| 1 | 01-audio-types.md | `cargo check` on audio/mod.rs |
| 2 | 02-audio-converter.md | `test_mono_passthrough`, `test_stereo_to_mono` pass |
| 3 | 03-audio-cache.md | `test_cache_pruning` passes |
| 4 | 04-audio-commands.md | Commands registered, invoke works |
| 5 | 05-audio-recorder-ui.md | Component mounts, recording state transitions work |

---

## Interaction Protocol

When the user asks to Ralph-prep a spec:

1. **Read the design spec** completely
2. **List the concerns** you've identified (ask user to confirm)
3. **Propose task breakdown** with ordering
4. **Generate atomic specs** one at a time
5. **Create IMPLEMENTATION_PLAN.md** linking everything

### Questions to Ask

Before decomposing:

- "Should I keep the original spec in `/specs/design/` as reference?"
- "Are there existing files/types this spec depends on?"
- "Any tasks you want to defer to a later phase?"

### Output Checklist

Before finishing:

- [ ] Each atomic spec is <150 lines
- [ ] Each atomic spec has ONE primary deliverable
- [ ] All backpressure is executable (specific test names or commands)
- [ ] Dependencies form a valid DAG (no cycles)
- [ ] Implementation plan has all tasks in buildable order
- [ ] No atomic spec contains "and" in its objective

---

## Anti-Patterns to Avoid

### Too Big

```markdown
# Audio Pipeline (BAD)

## Deliverables
- AudioInput struct
- AudioConverter module
- AudioCache module
- Three Tauri commands
- Frontend AudioRecorder
```

**Problem**: This is the whole design spec, not a task.

### Too Small

```markdown
# Add AudioInput Struct Field (BAD)

## Deliverables
- Add `sample_rate` field to AudioInput
```

**Problem**: This is a single line change, not worth its own spec.

### Vague Backpressure

```markdown
## Backpressure
- Tests should pass
- Code should work
```

**Problem**: Not executable. Which tests? How to verify?

### Correct Size

```markdown
# Audio Converter Module

## Objective
Implement format conversion from arbitrary audio input to Whisper-compatible 16kHz mono PCM.

## Deliverables
- `src-tauri/src/audio/converter.rs`
- Functions: `convert_for_whisper`, `mix_to_mono`, `resample`

## Backpressure
```bash
cargo test audio::converter::tests
```
Must pass: `test_mono_passthrough`, `test_stereo_to_mono`, `test_resampling_preserves_duration`
```

---

## Quick Reference

### Sizing Guidelines

| Metric | Target |
|--------|--------|
| Lines per atomic spec | 80-150 |
| Files created per task | 1-2 |
| Functions per task | 2-5 |
| Tests per task | 2-4 |

### Common Task Patterns

| Pattern | Example |
|---------|---------|
| Types-first | `01-types.md` → defines structs used by later tasks |
| Module-per-task | `02-converter.md` → one file with its tests |
| API-last | `04-cli.md` → CLI/API layer after core logic |
| Integration-final | `05-integration.md` → wiring after components tested |

### Backpressure Commands

```bash
# Go compilation check
go build ./...

# Go specific package tests
go test ./internal/discovery/... -v

# Go all tests
go test ./... -v

# Rust compilation check
cargo check -p crate_name

# Rust specific test module
cargo test module::tests

# Rust all tests
cargo test -p crate_name

# TypeScript type check
pnpm typecheck

# TypeScript tests
pnpm test -- --testPathPattern=Component
```

### Baseline Check Commands (per-unit, end of unit)

```bash
# Go
go fmt ./... && go vet ./... && staticcheck ./...

# Rust
cargo fmt --check && cargo clippy -- -D warnings

# TypeScript
pnpm typecheck && pnpm lint
```

---

## Final Note

The goal is to make each task **boring and obvious**. If Ralph has to make architectural decisions during implementation, the decomposition failed. All decisions should be in the design spec; atomic specs are just execution instructions.
