# Spec Writing Skill

**Description**: Create or edit technical specifications following the
established style and structure for choo projects.

**When to use**: When the user asks to create specs from a PRD, update existing
specs, or write documentation for technical components.

---

## Workflow: Parallel Spec Generation

This skill supports parallel spec generation. When given a PRD or requirements
document:

### Phase 1: Analysis (You do this)

1. **Read the source document** (PRD, requirements, etc.)
2. **Identify specs to create** - List each component that needs a spec
3. **Map dependencies** - Build a dependency graph showing which specs depend on
   others
   - Identify specs with no dependencies (can start immediately)
   - Identify specs that must wait for others
   - Group specs that can be implemented in parallel
4. **Extract per-spec context** - For each spec, note:
   - Component name and brief description
   - Relevant PRD section(s) to reference
   - Key code snippets to include
   - Which other specs it depends on
5. **Read an existing spec** for style reference (e.g.,
   `specs/completed/CLI.md`)

### Phase 2: Parallel Execution (Spawn subagents)

For each spec, spawn a `general-purpose` subagent using the **Subagent Prompt
Template** below. Pass:

- The spec name and output path
- The relevant PRD section content (extracted, not referenced)
- Any code snippets from the PRD
- The style guide (embedded in template)

**Critical**: Do NOT tell subagents to read the skill file or PRD themselves.
Embed all context in their prompt.

## Phase 3: Validation (Spawn spec-validator agent)

After spec-writing subagents complete, spawn a `general-purpose` subagent to
validate and fix cross-spec consistency issues. Use this prompt:

```
You are acting as a Spec Validator. Validate and fix specs against the PRD.

## Files to Read
1. PRD: {{PRD_PATH}}
2. Specs to validate:
{{LIST_OF_SPEC_PATHS}}

## Validation Checks

1. **Type Consistency** - Same type name should have compatible definitions across specs. PRD definitions are canonical.
2. **Interface Alignment** - Function signatures match between exporter and importer
3. **Dependency Validity** - All depends_on references point to existing units
4. **Import/Export Balance** - Every imported type has a corresponding export
5. **Naming Consistency** - Same concepts use same names, terminology matches PRD

## Auto-Fix Rules
- Type subset → Use superset
- Missing export → Add export if type defined
- Invalid depends_on → Remove with warning
- Naming inconsistency → Use PRD term
- Signature mismatch → Report, requires decision

## Resolution Priority
1. PRD is canonical
2. Exporter wins
3. More specific wins
4. Upstream wins

Read all files, apply auto-fixes directly to spec files, and produce a validation report with:
- Summary table (errors/warnings/auto-fixed)
- Detailed error descriptions
- Actions taken
- Remaining issues requiring human decision
```

See `.claude/agents/spec-validator.md` for full validation rules and output
format.

### Phase 4: Documentation (You do this)

After validation completes, update `specs/README.md` with:

- [ ] Table of new specs with descriptions and dependencies
- [ ] Dependency graph (ASCII diagram showing relationships)
- [ ] Suggested implementation order (parallelizable groups)

Example README table format:

```markdown
| Spec                          | Description       | Dependencies |
| ----------------------------- | ----------------- | ------------ |
| **[COMPONENT](COMPONENT.md)** | Brief description | DEP1, DEP2   |
```

---

## Subagent Prompt Template

Use this template when spawning spec-writing subagents. Replace
`{{placeholders}}` with actual content.

```
Create a technical specification at `{{OUTPUT_PATH}}`.

## Spec Details

- **Component**: {{COMPONENT_NAME}}
- **Brief Description**: {{BRIEF_DESCRIPTION}}

## Source Material

{{EXTRACTED_PRD_CONTENT}}

## Code Snippets to Include

{{CODE_SNIPPETS_FROM_PRD}}

---

## STYLE GUIDE (Follow exactly)

### Core Principles

1. **Clarity over cleverness**: Write for engineers who need to implement
2. **Concrete over abstract**: Show actual code, not concepts
3. **Complete but concise**: Enough detail to implement, but focused
4. **Structured consistency**: Follow template sections in order

### Writing Tone

- Technical and precise, but conversational
- No marketing language ("amazing", "powerful")
- Active voice ("The system converts" not "is converted")
- Direct and unambiguous

### Required Sections (in order)

# COMPONENT-NAME — Brief Description

## Overview
- 2-3 paragraphs: WHAT it does, WHY it exists
- Include ASCII architecture diagram if relevant

## Requirements

### Functional Requirements
- Numbered list of capabilities
- Each must be testable/verifiable

### Performance Requirements
| Metric | Target |
|--------|--------|
| [Measurement] | [Specific value] |

### Constraints
- Platform limitations
- Dependencies on other systems

## Design

### Module Structure
```
path/to/module/
├── file.go    # Purpose
└── other.go   # Purpose
```

### Core Types
```go
// Include ALL fields with types
// Add comments for non-obvious choices
type Example struct {
    Field string // Explain if needed
}
```

### API Surface

```go
// Show actual function signatures
func FunctionName(param Type) (ReturnType, error)
```

## Implementation Notes

- Platform-specific issues
- Performance considerations
- Edge cases
- Security concerns

## Testing Strategy

### Unit Tests

```go
func TestSpecificBehavior(t *testing.T) {
    // Show COMPLETE test structure
    // Not "// test here"
}
```

### Integration Tests

- List key scenarios

### Manual Testing

- [ ] Checklist items

## Design Decisions

### Why [Decision]?

- Explain rationale
- Include trade-offs considered

## Future Enhancements

1. Feature not in initial scope
2. Extension points

## References

- Link to PRD sections
- Related specs

### Code Example Rules

**GOOD** - Complete, contextual:

```go
type Worker struct {
    ID       string
    Unit     *Unit
    Worktree string
}

func NewWorker(unit *Unit) *Worker {
    return &Worker{
        ID:   fmt.Sprintf("worker-%s", unit.ID),
        Unit: unit,
    }
}
```

**BAD** - Abstract, incomplete:

```go
type Worker struct {
    // ... fields
}

func NewWorker() {
    // ... creates worker
}
```

### ASCII Diagram Style

Architecture:

```
┌─────────────────────────────────────────┐
│              Component A                 │
├─────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐            │
│  │ Module 1 │  │ Module 2 │            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
```

Flow:

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│  Step 1 │────▶│  Step 2 │────▶│  Step 3 │
└─────────┘     └─────────┘     └─────────┘
```

### Anti-Patterns to AVOID

- Vague requirements ("should be fast")
- Pseudocode instead of real code
- Missing type definitions
- Abstract descriptions without concrete examples
- "// implementation here" comments

---

## Your Task

Write the complete spec for {{COMPONENT_NAME}} following this style guide
exactly.

Use the source material and code snippets provided above. Do NOT read external
files for style guidance - everything you need is in this prompt. Write the
COMPLETE spec in one pass.

```
---

## Example: Spawning Parallel Subagents

When given a PRD with 3 components (Escalation, Orchestrator, CI):
```

I'll create specs for the 3 components in parallel.

[Spawn subagent 1] Task: "Create ESCALATION.md spec" Prompt: [Template with
Escalation details, PRD section 3 content, code snippets]

[Spawn subagent 2] Task: "Create ORCHESTRATOR.md spec" Prompt: [Template with
Orchestrator details, PRD section 4 content, code snippets]

[Spawn subagent 3] Task: "Create CI.md spec" Prompt: [Template with CI details,
PRD section 8 content, code snippets]

```
---

## Quick Reference: What Goes in Each Section

| Section | Content |
|---------|---------|
| Overview | What + Why + Architecture diagram |
| Functional Requirements | Numbered, testable capabilities |
| Performance Requirements | Table with metrics and targets |
| Constraints | Dependencies, platform limits |
| Module Structure | Directory tree with file purposes |
| Core Types | Complete struct/interface definitions |
| API Surface | Public function signatures |
| Implementation Notes | Gotchas, edge cases, platform issues |
| Unit Tests | Complete test examples with assertions |
| Design Decisions | Why choices were made, trade-offs |
| Future Enhancements | Out-of-scope improvements |
| References | Links to PRD, related specs |

---

## Checklist for Spec Quality

**Structure**
- [ ] All required sections present
- [ ] Sections in correct order
- [ ] Consistent header levels (## main, ### sub)

**Content**
- [ ] Overview explains what, why, and context
- [ ] Requirements are specific and measurable
- [ ] Code examples are COMPLETE (not pseudocode)
- [ ] ASCII diagrams render correctly

**Code Quality**
- [ ] All types have all fields defined
- [ ] Functions have full signatures
- [ ] Tests have actual assertions
- [ ] No "// ..." or "// implementation" comments

**Clarity**
- [ ] No vague language ("fast", "robust")
- [ ] No marketing speak
- [ ] Technical terms used correctly
