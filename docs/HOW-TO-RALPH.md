# Ralph-Style Autonomous Workflow with Backpressure

This document summarizes a generalizable workflow inspired by Geoffrey Huntley’s
“Ralph is a bash loop” concept for autonomous LLM-driven work. The workflow applies
to software development and non-software domains (e.g. financial audits, research),
with explicit mechanisms for validation (“backpressure”).

---

## Core Idea

**Ralph = a simple loop + strong constraints.**

An autonomous agent repeatedly:
1. Chooses a single task
2. Executes it in isolation
3. Validates the result against objective and subjective checks
4. Commits progress
5. Repeats until the plan is complete

Correctness is enforced through *backpressure*: tests, checks, reviews, and gates
that must pass before progress is accepted.

---

## Phase 1: Idea → Design Spec / Requirements

### Goal
Turn an ambiguous idea into explicit, testable requirements.

### Steps
- Define **Jobs To Be Done (JTBD)** or objectives.
- Break the project into **single-concern specs** (no "and").
- Write one spec per concern.
- Clarify inputs, outputs, constraints, and success criteria.

### Output
- Design spec(s) in `specs/` — e.g., `MVP DESIGN SPEC.md`, `AUDIO-PIPELINE.md`, etc.
- The filename is not fixed; any descriptive name works.
- Each spec serves as the "PRD" for its corresponding implementation unit.

### Validation (Backpressure)
- Requirements traceability: every goal maps to a spec.
- Completeness checks (missing cases, edge conditions).
- Domain review (regulatory, stakeholder, or policy checks).

---

## Phase 2: Design Spec → Implementation Plan (Unit)

### Goal
Translate specs into an executable task list. This produces a "unit" — a collection of ordered tasks.

### Steps
- Perform a **gap analysis** between specs and current state.
- Generate a prioritized list of **small, atomic tasks**.
- No implementation yet—planning only.

### Output
```
specs/
├── ORIGINAL-SPEC.md           # Design spec
└── tasks/
    └── original-spec/         # Unit directory
        ├── IMPLEMENTATION_PLAN.md
        ├── 01-first-task.md
        ├── 02-second-task.md
        └── ...
```

- `IMPLEMENTATION_PLAN.md` contains:
  - YAML frontmatter with `unit` ID and `depends_on` (other units)
  - Ordered list of tasks with backpressure commands
  - Each task clearly "done-able" in one iteration

### Validation (Backpressure)
- Every spec maps to ≥1 task.
- Tasks are concrete, not vague.
- No task spans multiple unrelated concerns.

---

## Phase 3: Ralph Execution Loop

### Goal
Autonomously execute the plan, one task at a time.

### Loop Structure
1. Read `IMPLEMENTATION_PLAN.md`
2. Select **one incomplete task**
3. Load relevant specs + context
4. Implement only that task
5. Run validation checks
6. Fix failures until checks pass
7. Commit changes and mark task done
8. Restart loop with fresh context

### Key Properties
- Fresh LLM context every iteration
- Strict single-task focus
- Deterministic progress
- Failures block forward motion

---

## Backpressure System (Critical)

Backpressure defines "done." No task is complete until checks pass.

### Task-Level Backpressure (per task)

Defined in each task's YAML frontmatter as a single command:

```yaml
backpressure: "go test ./internal/discovery/... -v"
```

Examples:
- Unit tests for the specific module
- Integration tests for the feature
- Schema validation
- Data consistency checks

### Unit-Level Baseline Checks (end of unit)

Run once after all tasks complete, before PR creation:
- Linting / formatting (`go fmt`, `cargo fmt`)
- Static analysis / type checking (`go vet`, `cargo clippy`)
- Full test suite (optional)

If baseline checks fail, a specialized Claude turn fixes them automatically.

### Semi-Automated / Manual Backpressure
- Browser-based user acceptance testing
- Sample data flow verification
- Reconciliation checks (e.g. financial totals)
- Visual or UX spot checks

### LLM-as-Judge Backpressure
- Secondary agent reviews output against qualitative criteria:
  - Clarity
  - Compliance
  - Readability
  - Alignment with intent

Failures trigger correction, not progression.

---

## Non-Software Generalization

The same loop applies outside coding:

### Financial Audit Example
- Specs: audit objectives, regulations
- Tasks: reconcile accounts, verify transactions
- Backpressure:
  - Cross-ledger reconciliation
  - Rule compliance checks
  - Anomaly detection
  - Narrative review of findings

### Research / Writing Example
- Specs: outline, claims, audience
- Tasks: draft sections
- Backpressure:
  - Fact checking
  - Source validation
  - Plagiarism checks
  - Style and clarity review

---

## Human Role

Humans do not micromanage execution.

They:
- Define good specs
- Define good validation
- Improve backpressure when failures occur

Over time, the system becomes more reliable as checks accumulate.

---

## Summary

**Workflow**
Idea → Design Spec → Implementation Plan (Unit) → Ralph Loop

**Guarantee**
Progress is gated by correctness, not optimism.

**Key Insight**
Autonomy works when failure is cheap, visible, and blocking.

Ralph is simple.
Backpressure does the hard work.

---

## Terminology

| Term | Definition |
|------|------------|
| **Design Spec** | High-level requirements document (any filename, e.g., `MVP DESIGN SPEC.md`) |
| **Unit** | A collection of tasks with dependencies implementing one spec |
| **Task** | An atomic piece of work completable in one Ralph iteration |
| **Ready Task** | A task whose dependencies are all complete (can be worked on) |
| **Backpressure** | Per-task validation command that must pass |
| **Baseline Checks** | Per-unit linting/formatting that runs at end of unit |

## Task Selection

Task numbers are **identifiers**, not execution order. The `depends_on` field determines when a task can run.

```
Tasks: #1-types, #2-parser, #3-validator, #4-cli
Dependencies: #2 depends on #1, #3 depends on #1, #4 depends on #2 and #3

Ready at start: #1
After #1 complete: #2, #3 are both ready
Agent chooses: could do #2 or #3 (either is valid)
```

The Ralph agent sees all ready tasks and **chooses** which to implement. This allows the agent to use context and judgment rather than following a rigid order.
