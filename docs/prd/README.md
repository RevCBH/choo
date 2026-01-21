# Product Requirements Documents (PRDs)

This directory contains Product Requirements Documents for choo features.

## PRD Index

| PRD ID | Title | Status | Version | Dependencies |
|--------|-------|--------|---------|--------------|
| [mvp-orchestrator](MVP-ORCHESTRATOR.md) | Ralph Orchestrator Core | Complete | v0.1 | - |
| [self-hosting](SELF-HOSTING.md) | Choo Self-Hosting | Complete | v0.2 | mvp-orchestrator |
| [web-ui](WEB-UI.md) | Web UI Monitoring | In Progress | v0.3 | self-hosting |
| [feature-workflow](FEATURE-WORKFLOW.md) | PRD-Based Feature Workflow | Draft | v0.4 | web-ui |
| [daemon-architecture](DAEMON-ARCHITECTURE.md) | Charlotte Daemon Architecture | Draft | v0.5 | mvp-orchestrator, web-ui |

## Status Legend

- **Draft**: Initial design, not yet approved
- **Approved**: Ready for implementation
- **In Progress**: Currently being implemented
- **Complete**: Fully implemented and merged
- **Archived**: Deprecated or superseded

## PRD Format

All PRDs use YAML frontmatter for machine-readable metadata:

```yaml
---
prd_id: example-feature       # Unique identifier (used for branch names)
title: "Example Feature"      # Human-readable title
status: draft                 # draft | approved | in_progress | complete | archived
depends_on:                   # Other PRD IDs this depends on
  - prerequisite-feature

# Orchestrator-managed fields (auto-populated during feature workflow):
# feature_branch: feature/example-feature
# feature_status: pending
# spec_review_iterations: 0
---
```

## Workflow

1. **Create PRD**: Write a new PRD in this directory with proper frontmatter
2. **Prioritize**: Run `choo next-feature` to analyze dependencies and recommend next feature
3. **Start Feature**: Run `choo feature start <prd-id>` to begin automated workflow
4. **Execute**: Run `choo run --feature <prd-id>` to develop with unit PRs targeting feature branch
5. **Complete**: Run `choo feature complete <prd-id>` to open PR to main

## Adding a New PRD

1. Create a new file: `docs/prd/YOUR-FEATURE.md`
2. Add frontmatter with required fields (`prd_id`, `title`, `status`)
3. Add optional `depends_on` hints for the prioritizer
4. Document the feature following the standard sections

## Standard Sections

A PRD should typically include:

1. **Document Info** - Metadata table (status, author, date, target version)
2. **Overview** - Goal, current state, proposed solution
3. **Architecture** - Diagrams, component relationships
4. **Requirements** - Functional, performance, constraints
5. **Design** - Detailed design, types, APIs
6. **Implementation Plan** - Phases, tasks, timeline
7. **Files to Create/Modify** - Explicit file lists
8. **Acceptance Criteria** - Testable conditions for completion
9. **Verification** - How to test the implementation
