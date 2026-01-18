# Spec Validator Agent

**Type**: `spec-validator`

**Description**: Validates and reconciles specs generated in parallel, ensuring type consistency, valid dependencies, and aligned interfaces. Use after parallel spec generation to catch conflicts before ralph-prep.

**When to use**:
- After running N parallel agents with the spec skill
- Before proceeding to ralph-prep
- When you suspect spec inconsistencies

---

## Invocation

```
Task tool with subagent_type: "spec-validator"

Prompt: "Validate specs in specs/ against PRD at docs/MVP DESIGN SPEC.md"
```

Or with fix mode:
```
Prompt: "Validate and fix specs in specs/ against PRD at docs/MVP DESIGN SPEC.md"
```

---

## Agent Behavior

### Input Requirements

The agent needs:
1. **PRD path** - Master design spec with canonical type definitions
2. **Specs directory** - Where the unit specs live
3. **Mode** - `check` (report only) or `fix` (auto-fix simple issues)

### Validation Checks

1. **Type Consistency**
   - Same type name should have compatible definitions across specs
   - PRD definitions are canonical

2. **Interface Alignment**
   - Function signatures match between exporter and importer
   - Return types and parameters agree

3. **Dependency Validity**
   - All `depends_on` references point to existing units
   - No references to non-existent specs

4. **Circular Dependencies**
   - Unit dependency graph has no cycles
   - Detect and report cycle path

5. **Import/Export Balance**
   - Every imported type has a corresponding export
   - No orphan imports

6. **Naming Consistency**
   - Same concepts use same names
   - Terminology matches PRD

### Output

The agent produces a validation report:

```markdown
# Spec Validation Report

## Summary
| Category | Errors | Warnings | Auto-fixable |
|----------|--------|----------|--------------|
| Type consistency | 1 | 0 | 1 |
| Interface alignment | 1 | 0 | 0 |
| ... | ... | ... | ... |

## Errors (must fix before proceeding)
[Detailed error descriptions with locations]

## Warnings (should fix)
[Detailed warning descriptions]

## Actions Taken (fix mode only)
[List of auto-fixes applied]

## Remaining Issues
[Issues requiring human decision]
```

---

## Auto-Fix Rules

| Conflict Type | Auto-fix? | Rule |
|---------------|-----------|------|
| Type subset | ✅ | Use superset |
| Missing export | ✅ | Add export if type defined |
| Invalid depends_on | ✅ | Remove with warning |
| Naming inconsistency | ✅ | Use PRD term |
| Signature mismatch | ❌ | Requires decision |
| Circular dependency | ❌ | Requires architectural change |

---

## Resolution Priority

When conflicts exist, resolve by:

1. **PRD is canonical** - PRD type definitions win
2. **Exporter wins** - The spec that exports a type owns its definition
3. **More specific wins** - Detailed definition over vague
4. **Upstream wins** - Earlier in dependency order takes precedence

---

## Example Prompts

### Basic validation
```
Validate all specs in specs/ against the PRD at docs/MVP DESIGN SPEC.md.
Report any type inconsistencies, invalid dependencies, or naming conflicts.
```

### Validation with auto-fix
```
Validate and fix specs in specs/ against docs/MVP DESIGN SPEC.md.
Auto-fix what you can (type subsets, missing exports, naming).
Report issues that need human decision.
```

### Targeted validation
```
Validate specs/DISCOVERY.md and specs/SCHEDULER.md for interface alignment.
They share Unit and Task types - ensure definitions match.
```

### Post-parallel-generation validation
```
I just generated 8 unit specs in parallel. Validate them all against
docs/MVP DESIGN SPEC.md and produce a reconciliation report. Fix simple
issues and list what needs my decision.
```

---

## Integration with Parallel Workflow

```
┌─────────────────────────────────────────────────────────────┐
│  1. Create context brief from PRD                           │
│     - List all units to generate                            │
│     - Extract canonical types                               │
│     - Define naming conventions                             │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  2. Parallel spec generation (N agents)                     │
│     Each agent: PRD + context brief + "write spec for X"    │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Spec validation (this agent)                            │
│     - Read all generated specs                              │
│     - Cross-reference for conflicts                         │
│     - Auto-fix simple issues                                │
│     - Report complex issues                                 │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Human review (if needed)                                │
│     - Resolve signature mismatches                          │
│     - Break circular dependencies                           │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Proceed to ralph-prep                                   │
│     - Validated specs ready for task decomposition          │
└─────────────────────────────────────────────────────────────┘
```

---

## Context Brief Template

Before parallel generation, create this brief for all agents:

```markdown
# Spec Generation Context Brief

## PRD Reference
- Path: docs/MVP DESIGN SPEC.md
- Canonical types: §3.4

## Units Being Generated
| Unit | Exports | Imports |
|------|---------|---------|
| discovery | Unit, Task, Discover() | - |
| scheduler | ReadyQueue, Dispatch() | Unit, Task |
| worker | Worker, Execute() | Unit, Task, ReadyQueue |
| git | Worktree, Branch | Unit |
| github | PRClient, ReviewStatus | Unit |
| events | EventBus, Event | - |
| cli | Commands | All |
| config | Config | - |

## Shared Types (from PRD §3.4)
Reference these, don't redefine:
- Unit, UnitStatus
- Task, TaskStatus

## Naming Conventions
- "unit" not "workset"
- "task" not "step"
- "worktree" not "workspace"

## Rules
1. Reference PRD types by section number
2. Export types others will import
3. Use depends_on for unit dependencies
```

---

## Notes

- Agent starts with fresh context (maximum space for reading specs)
- Reads all specs before analyzing (no incremental)
- Produces single comprehensive report
- Human reviews report and resolves flagged issues
- Run again after fixes to confirm clean validation
