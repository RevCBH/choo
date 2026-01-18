# Spec Validation Skill

**Description**: Validate and reconcile a set of specs generated in parallel, ensuring type consistency, valid dependencies, and aligned interfaces.

**When to use**: After generating multiple unit specs in parallel (using the spec skill), run this to catch conflicts before proceeding to ralph-prep.

---

## What This Skill Does

Takes a **set of unit specs** and:

1. **Validates** consistency across specs
2. **Reports** conflicts with specific locations
3. **Suggests** or **auto-fixes** simple issues
4. **Escalates** complex conflicts requiring human decision

---

## Validation Checks

### 1. Type Consistency

**Check**: Same type name defined in multiple specs should have compatible definitions.

```
❌ CONFLICT: Type `Unit` defined differently

  specs/DISCOVERY.md:45
    type Unit struct {
        ID       string
        Path     string
        Tasks    []*Task
    }

  specs/SCHEDULER.md:78
    type Unit struct {
        ID       string
        Status   UnitStatus
        Priority int
    }

  → RESOLUTION: Merge fields or clarify which is canonical
```

**Auto-fix**: If one is a subset of the other, use the superset.

### 2. Interface Alignment

**Check**: If spec A exports a function, specs that import it should expect the same signature.

```
❌ CONFLICT: Function signature mismatch

  specs/DISCOVERY.md exports:
    func Discover(path string) ([]*Unit, error)

  specs/SCHEDULER.md expects:
    func Discover(path string, opts DiscoverOptions) ([]*Unit, error)

  → RESOLUTION: Align on single signature
```

**Auto-fix**: None - requires human decision.

### 3. Dependency Validity

**Check**: All `depends_on` references point to existing units.

```
❌ ERROR: Invalid dependency

  specs/tasks/worker/IMPLEMENTATION_PLAN.md
    depends_on: [discovery, scheduler, nonexistent-unit]
                                       ^^^^^^^^^^^^^^^^
  → Unit 'nonexistent-unit' does not exist
```

**Auto-fix**: Remove invalid reference (with warning).

### 4. Circular Dependencies

**Check**: Unit dependency graph has no cycles.

```
❌ ERROR: Circular dependency detected

  discovery → scheduler → worker → discovery
              ^__________________________|

  → Break cycle by removing one dependency
```

**Auto-fix**: None - requires architectural decision.

### 5. Import/Export Balance

**Check**: Every imported type has a corresponding export.

```
⚠️ WARNING: Unresolved import

  specs/WORKER.md imports:
    - SCHEDULER: [ReadyQueue, DispatchResult]
                              ^^^^^^^^^^^^^^
  specs/SCHEDULER.md exports:
    - ReadyQueue
    - (DispatchResult not listed)

  → Either add export or remove import
```

**Auto-fix**: Add missing export if type is defined in the spec.

### 6. Naming Consistency

**Check**: Same concepts use consistent names across specs.

```
⚠️ WARNING: Inconsistent naming

  specs/DISCOVERY.md: "unit directory"
  specs/SCHEDULER.md: "unit folder"
  specs/WORKER.md: "unit path"

  → Standardize on one term
```

**Auto-fix**: Suggest canonical name, optionally replace all.

---

## Input Format

The skill expects:

1. **PRD/Master spec path** - The source of truth for shared types
2. **Unit specs directory** - Where parallel-generated specs live
3. **Validation mode** - `check` (report only) or `fix` (auto-fix simple issues)

```yaml
# Example invocation context
prd: docs/MVP DESIGN SPEC.md
specs_dir: specs/
mode: fix  # or 'check'
```

---

## Output Format

### Validation Report

```markdown
# Spec Validation Report

Generated: 2026-01-18T10:30:00Z
Mode: check
Specs validated: 8

## Summary

| Category | Errors | Warnings | Auto-fixable |
|----------|--------|----------|--------------|
| Type consistency | 1 | 0 | 1 |
| Interface alignment | 1 | 0 | 0 |
| Dependency validity | 0 | 0 | 0 |
| Circular dependencies | 0 | 0 | 0 |
| Import/Export balance | 0 | 2 | 2 |
| Naming consistency | 0 | 3 | 3 |

## Errors (must fix)

### E1: Type `Unit` defined inconsistently
- Location: specs/DISCOVERY.md:45, specs/SCHEDULER.md:78
- Issue: Different field sets
- Resolution: Use PRD definition as canonical (§3.4)
- Auto-fix: ✅ Available

### E2: Function signature mismatch for `Discover`
- Location: specs/DISCOVERY.md:120, specs/SCHEDULER.md:45
- Issue: Different parameter lists
- Resolution: Manual decision required
- Auto-fix: ❌ Not available

## Warnings (should fix)

### W1: Unresolved import `DispatchResult`
...

## Suggested Actions

1. [ ] Review E2 and decide on `Discover` signature
2. [ ] Run with `--fix` to auto-resolve E1, W1-W5
```

---

## Reconciliation Strategy

When conflicts are found:

### Priority Order for Resolution

1. **PRD is canonical** - If PRD defines a type, that's the source of truth
2. **Exporter wins** - The spec that exports a type owns its definition
3. **More specific wins** - More detailed definition preferred over vague
4. **Earlier in dependency order** - Upstream specs take precedence

### Auto-fix Rules

| Conflict Type | Auto-fix? | Rule |
|---------------|-----------|------|
| Type subset | ✅ | Use superset |
| Missing export | ✅ | Add export if type exists |
| Invalid depends_on | ✅ | Remove with warning |
| Naming inconsistency | ✅ | Use PRD term or most common |
| Signature mismatch | ❌ | Requires decision |
| Circular dependency | ❌ | Requires architectural change |
| Semantic conflict | ❌ | Requires human judgment |

---

## Interaction Protocol

### Check Mode (default)

```
User: Validate the specs in specs/

Agent:
1. Load PRD from configured location
2. Scan specs/ for all *.md files
3. Parse type definitions, imports, exports
4. Run all validation checks
5. Generate report
6. Present summary with action items
```

### Fix Mode

```
User: Validate and fix the specs in specs/

Agent:
1. Run check mode first
2. For each auto-fixable issue:
   a. Show the fix
   b. Apply the fix
   c. Note in report
3. Re-run validation to confirm fixes
4. Present remaining issues requiring human decision
```

---

## Integration with Parallel Spec Generation

### Workflow

```
┌─────────────────────────────────────────────────────────────┐
│                    Parallel Generation                       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │ Agent 1 │ │ Agent 2 │ │ Agent 3 │ │ Agent N │           │
│  │ spec A  │ │ spec B  │ │ spec C  │ │ spec N  │           │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘           │
│       │           │           │           │                 │
│       └───────────┴─────┬─────┴───────────┘                 │
│                         ▼                                   │
│              ┌─────────────────────┐                        │
│              │  Spec Validation    │                        │
│              │  (this skill)       │                        │
│              └──────────┬──────────┘                        │
│                         │                                   │
│              ┌──────────▼──────────┐                        │
│              │  Conflicts?         │                        │
│              └──────────┬──────────┘                        │
│                    ┌────┴────┐                              │
│                    ▼         ▼                              │
│              Auto-fix    Human review                       │
│                    │         │                              │
│                    └────┬────┘                              │
│                         ▼                                   │
│              ┌─────────────────────┐                        │
│              │  Validated Specs    │                        │
│              └─────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
```

### Context Brief for Parallel Agents

Before parallel generation, create a brief that all agents receive:

```markdown
# Spec Generation Context Brief

## PRD Reference
- Path: docs/MVP DESIGN SPEC.md
- Canonical types defined in: §3.4

## Units Being Generated
| Unit | Assignee | Exports | Imports |
|------|----------|---------|---------|
| discovery | Agent 1 | Unit, Task, Discover() | - |
| scheduler | Agent 2 | ReadyQueue, Dispatch() | Unit, Task |
| worker | Agent 3 | Worker, Execute() | Unit, Task, ReadyQueue |
| ... | ... | ... | ... |

## Shared Type Definitions (from PRD)
- `Unit`: §3.4, use exactly as defined
- `Task`: §3.4, use exactly as defined
- `UnitStatus`: §3.4, use exactly as defined
- `TaskStatus`: §3.4, use exactly as defined

## Naming Conventions
- Use "unit" not "workset" or "job"
- Use "task" not "step" or "item"
- Use "worktree" not "workspace" for git worktrees

## Rules
1. Reference PRD types by section, don't redefine
2. Export types you create that others will use
3. Import types from other specs by spec name
4. Use depends_on for unit dependencies
```

---

## Quick Reference

### Validation Commands

```bash
# Check only (report issues)
# Skill invocation: "validate specs in specs/"

# Check and fix
# Skill invocation: "validate and fix specs in specs/"

# Validate specific specs
# Skill invocation: "validate specs/DISCOVERY.md and specs/SCHEDULER.md"
```

### Common Resolutions

| Issue | Resolution |
|-------|------------|
| Type defined in multiple specs | Reference PRD or designate one spec as owner |
| Missing import source | Add export to source spec |
| Circular dependency | Introduce interface/abstraction to break cycle |
| Naming inconsistency | Pick one, update all references |

---

## Final Note

The goal of validation is **catching issues early**, before they become implementation bugs. A clean validation report means specs are ready for ralph-prep. Any remaining conflicts should be resolved before proceeding to task generation.
