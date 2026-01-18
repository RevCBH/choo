# PROJECT-SETUP Implementation Plan

## Overview

This plan decomposes the PROJECT-SETUP design spec into 9 atomic tasks that establish the Koe project foundation: Tauri scaffolding, configuration files, directory structure, entry points, test infrastructure, database schema, and CI workflow.

**Design spec**: `/specs/PROJECT-SETUP.md`

## Prerequisites

Before running Ralph on this workset, the user must manually:

1. Install Rust: `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`
2. Install Node.js 18+ and pnpm
3. Install Tauri CLI: `cargo install tauri-cli`
4. Run `npm create tauri-app@latest` with:
   - App name: `koe`
   - Window title: `Koe`
   - UI recipe: React + TypeScript
   - Package manager: pnpm
5. `cd koe` into the created directory

## Task Sequence

> **Note:** Task status is tracked in YAML frontmatter of each task spec file.
> Run `./ralph.sh specs/tasks/project-setup --status` to see current status.

| # | Task Spec | Description | Dependencies |
|---|-----------|-------------|--------------|
| 1 | [01-core-config.md](./01-core-config.md) | Cargo.toml, package.json, tauri.conf.json | None |
| 2 | [02-directory-structure.md](./02-directory-structure.md) | Create src/ and src-tauri/src/ hierarchy | #1 |
| 3 | [03-frontend-config.md](./03-frontend-config.md) | vite, tsconfig, tailwind, postcss | #1 |
| 4 | [04-dev-tooling.md](./04-dev-tooling.md) | ESLint, Prettier, .gitignore | #1 |
| 5 | [05-rust-entry.md](./05-rust-entry.md) | main.rs, lib.rs, error.rs, state.rs | #1, #2 |
| 6 | [06-frontend-entry.md](./06-frontend-entry.md) | main.tsx, App.tsx, globals.css | #3 |
| 7 | [07-test-infrastructure.md](./07-test-infrastructure.md) | vitest, playwright, test setup | #3 |
| 8 | [08-database-schema.md](./08-database-schema.md) | migrations/001_init.sql, migrations.rs | #5 |
| 9 | [09-ci-workflow.md](./09-ci-workflow.md) | GitHub Actions build.yml | #7 |

## Dependency Graph

```
┌─────────────────┐
│ 01-core-config  │
└────────┬────────┘
         │
    ┌────┼────────────────────┐
    │    │                    │
    ▼    ▼                    ▼
┌────────┐  ┌────────────┐  ┌────────────┐
│02-dirs │  │03-frontend │  │04-dev-tools│
└───┬────┘  │  -config   │  └────────────┘
    │       └─────┬──────┘
    │             │
    ▼             │
┌────────────┐    │
│05-rust-    │    │
│   entry    │    │
└───┬────────┘    │
    │       ┌─────┴─────────────┐
    │       │                   │
    │       ▼                   ▼
    │  ┌────────────┐    ┌────────────┐
    │  │06-frontend │    │07-test-    │
    │  │   -entry   │    │   infra    │
    │  └────────────┘    └─────┬──────┘
    │                          │
    ▼                          ▼
┌────────────┐           ┌────────────┐
│08-database │           │09-ci-      │
│   -schema  │           │  workflow  │
└────────────┘           └────────────┘
```

## Parallelization Opportunities

Tasks can be parallelized within dependency constraints:

- **After #1**: Tasks #2, #3, #4 can run in parallel
- **After #2 + #3**: Tasks #5 and #6 can run in parallel (different layers)
- **After #5 + #7**: Tasks #8 and #9 can run in parallel

For serial Ralph execution, the recommended order is:
`1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9`

## Completion Criteria

All tasks marked complete when:

- [ ] All 9 task specs pass their backpressure validation
- [ ] `npm run tauri:dev` launches the app successfully
- [ ] `npm test -- --run` passes all tests
- [ ] `cargo test` passes all Rust tests
- [ ] CI workflow passes on GitHub (manual verification)

## Estimated Effort

| Task | Lines | Complexity |
|------|-------|------------|
| core-config | ~150 | Low |
| directory-structure | ~30 | Low |
| frontend-config | ~100 | Low |
| dev-tooling | ~80 | Low |
| rust-entry | ~150 | Medium |
| frontend-entry | ~80 | Low |
| test-infrastructure | ~120 | Medium |
| database-schema | ~200 | Medium |
| ci-workflow | ~100 | Low |

**Total**: ~1000 lines across 9 tasks

## Notes

- This is the foundational spec — all other specs depend on PROJECT-SETUP being complete
- The scaffold from `npm create tauri-app` provides ~40% of the structure; we expand it
- Database schema here is the **initial** schema; DATABASE spec adds repositories/queries
- Test infrastructure is minimal; actual tests written alongside features

## Reference

- Design spec: `/specs/PROJECT-SETUP.md`
- Dependency graph: `/specs/README.md`
