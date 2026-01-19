# Create Technical Specifications

Generate technical specifications from a PRD or requirements document.

## Usage

```
/spec <path-to-prd-or-requirements>
```

## Arguments

`$ARGUMENTS` - Path to the PRD or requirements document to convert into specs

## Instructions

Follow the spec writing skill at `.claude/skills/spec.md` which defines:

1. **Phase 1: Analysis** - Read the source document, identify specs to create, extract per-spec context
2. **Phase 2: Parallel Execution** - Spawn subagents using the embedded prompt template (do NOT have subagents read external files)
3. **Phase 3: Verification** - Verify all specs were created and follow the template

**Critical workflow rules:**
- Read `.claude/skills/spec.md` FIRST for the complete style guide and subagent prompt template
- Read an existing spec (e.g., `specs/completed/CLI.md`) for style reference
- Extract PRD content and embed it directly in subagent prompts
- Do NOT tell subagents to read the skill file or PRD - embed all context in their prompt
- Spawn subagents in PARALLEL for each spec

## Input

The source document at: $ARGUMENTS
