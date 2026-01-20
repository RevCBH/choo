# Proposal: Mocked Git by Default with Tagged Integration Tests

## Summary
The test suite is slow primarily because many unit tests spawn real git processes, create worktrees, and perform real rebase/push flows. This proposal makes mocked git execution the default for unit tests, and moves real-git coverage behind an explicit build tag (`integration`). The change reduces per-test overhead while preserving end-to-end confidence with a smaller, opt-in integration suite.

This proposal reflects the implemented changes in the current branch and documents the intent, mechanics, and follow-ups.

---

## Problem Statement
The current test suite spends most of its time in integration-style tests that:
- Initialize real git repositories and remotes.
- Create and remove real worktrees.
- Execute many `git` processes across multiple packages.
- Simulate conflict resolution using real `git rebase` workflows.

These tests are valuable, but they run by default and dominate runtime. Most tests are asserting *our logic* (e.g., conflict detection, retry loops, prompt building) rather than git behavior, so they can be safely mocked at the unit-test layer.

---

## Goals
1. **Reduce default test runtime** by avoiding real process and worktree spawning.
2. **Preserve correctness** of our git-related logic via mock-based unit tests.
3. **Retain end-to-end confidence** with an opt-in integration test suite.
4. **Make mocking easy and deterministic** without global side effects leaking between tests.

## Non-Goals
- Remove all integration coverage entirely.
- Re-architect git workflows or change business logic.
- Change CLI or runtime behavior in production.

---

## Proposed Design

### 1) Add a Git Runner Abstraction
Introduce a `git.Runner` interface that executes git commands, with a default OS-backed runner:
- **Interface**: `Exec(ctx, dir, args...)` and `ExecWithStdin(ctx, dir, stdin, args...)`
- **Default implementation**: wraps `exec.CommandContext` and current error formatting.
- **Default management**: `git.DefaultRunner()` + `git.SetDefaultRunner(...)` for tests.

This keeps production behavior the same while allowing tests to inject a fake runner.

### 2) Wire Git Usage to the Runner
Replace direct `exec.CommandContext("git", ...)` usage in our code with the runner:
- `internal/git/exec.go` now uses the runner, and remains the single gateway.
- `internal/worker/worker.go` and `internal/worker/git_delegate.go` now use a runner via `WorkerDeps.GitRunner` (defaulting to `git.DefaultRunner()` when nil).

### 3) Make Tests Mocked by Default
Convert heavy git tests to mock-based equivalents using a fake runner:
- `internal/git/merge_test.go`: mock rebase/fetch/push behavior and conflict flow.
- `internal/git/worktree_test.go`: mock worktree create/remove/list parsing logic.
- `internal/git/commit_test.go`: verify arguments and output parsing.
- `internal/worker/git_delegate_test.go`: mock `rev-parse`, `ls-remote`, and status parsing.

Add lightweight helper fakes in test packages:
- `internal/git/fake_runner_test.go`
- `internal/worker/fake_git_runner_test.go`

### 4) Move Real Git Tests Behind `integration` Tag
Rename and tag current integration tests so they run only when explicitly requested:
- `internal/git/merge_integration_test.go`
- `internal/git/worktree_integration_test.go`
- `internal/git/commit_integration_test.go`
- `internal/git/exec_integration_test.go`
- `internal/worker/git_delegate_integration_test.go`
- `internal/orchestrator/orchestrator_integration_test.go`
- `internal/cli/run_integration_test.go`

### 5) Stub Git Remote Detection in Config Tests
`config.detectGitHubRepo` used to shell out to `git remote get-url`. This now supports injection:
- `gitRemoteGetter` in `internal/config/github.go`
- `internal/config/git_remote_stub_test.go` provides a stub helper for tests.

This avoids shelling out in config unit tests.

---

## Implementation Details (Current Branch)

### Key Code Changes
- **Runner abstraction**: `internal/git/exec.go`
- **Worker usage**: `internal/worker/worker.go`, `internal/worker/git_delegate.go`
- **Git remote stub**: `internal/config/github.go`, `internal/config/git_remote_stub_test.go`

### New / Updated Unit Tests
- Mocked git logic tests:
  - `internal/git/merge_test.go`
  - `internal/git/worktree_test.go`
  - `internal/git/commit_test.go`
  - `internal/worker/git_delegate_test.go`
- Fake runner helpers:
  - `internal/git/fake_runner_test.go`
  - `internal/worker/fake_git_runner_test.go`

### Integration Tests (Build Tag)
- Run only with `-tags=integration`:
  - `internal/git/merge_integration_test.go`
  - `internal/git/worktree_integration_test.go`
  - `internal/git/commit_integration_test.go`
  - `internal/git/exec_integration_test.go`
  - `internal/worker/git_delegate_integration_test.go`
  - `internal/orchestrator/orchestrator_integration_test.go`
  - `internal/cli/run_integration_test.go`

---

## Testing Strategy

### Default Unit Test Run
```
go test ./...
```
Uses fake git runners and avoids spawning processes/worktrees.

### Integration Test Run
```
go test ./... -tags=integration
```
Uses real git, real worktrees, and full conflict flows.

---

## Tradeoffs & Risks

### Pros
- Large reduction in test runtime for the default suite.
- Deterministic, fast unit tests for core logic.
- Clear separation between logic tests and integration tests.

### Cons / Risks
- Mock tests might drift from real git behavior.
- Integration suite is opt-in, so regressions in real git flows could slip if not run regularly.

**Mitigations**
- Keep a small but meaningful integration set.
- Ensure CI runs integration tests on a schedule or on release branches.

---

## Alternatives Considered
1. **Keep integration tests but reduce sleeps/timeouts**
   - Minor improvements only; doesn't address process spawn overhead.
2. **Switch to a full git simulation layer**
   - Too much complexity relative to the runtime benefit.
3. **Use build tags only for heavy subsets**
   - More complex to maintain; hard to reason about test coverage.

The chosen approach offers the best balance of speed and confidence.

---

## Rollout Plan
1. Land this change set (already implemented on this branch).
2. Update CI to optionally run `-tags=integration` on a schedule.
3. Add docs or developer notes on running integration tests locally.

---

## Open Questions
- Should we enforce integration test runs in CI on main merges?
- Do we want a dedicated `make test-integration` target?
- Should more git-related packages (e.g., `internal/config`) adopt runner injection for other external commands in future?

---

## Appendix: Affected Files (High-Level)
- Core runner: `internal/git/exec.go`
- Worker wiring: `internal/worker/worker.go`, `internal/worker/git_delegate.go`
- Git remote stub: `internal/config/github.go`, `internal/config/git_remote_stub_test.go`
- Mock tests: `internal/git/merge_test.go`, `internal/git/worktree_test.go`, `internal/git/commit_test.go`, `internal/worker/git_delegate_test.go`
- Integration tests: `internal/git/*_integration_test.go`, `internal/worker/*_integration_test.go`, `internal/orchestrator/orchestrator_integration_test.go`, `internal/cli/run_integration_test.go`
