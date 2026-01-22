---
task: 1
status: complete
backpressure: "go build ./internal/worker/..."
depends_on: []
---

# Worker Struct Updates

**Parent spec**: `/specs/GITOPS-WORKER.md`
**Task**: #1 of 5 in implementation plan

## Objective

Add gitOps field to Worker struct and update WorkerDeps to support GitOps injection.

## Dependencies

### External Specs (must be implemented)
- GITOPS — provides GitOps interface

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- Internal: `internal/git` (for GitOps interface)

## Deliverables

### Files to Modify

```
internal/worker/
└── worker.go    # MODIFY: Add gitOps field to Worker, GitOps to WorkerDeps
```

### Types to Modify

```go
// Worker executes a single unit in an isolated worktree
type Worker struct {
    unit         *discovery.Unit
    config       WorkerConfig
    events       *events.Bus
    git          *git.WorktreeManager

    // Phase 1: GitOps added alongside gitRunner
    // Phase 3: gitRunner removed, only gitOps remains
    gitOps    git.GitOps // Safe git operations interface
    gitRunner git.Runner // Deprecated: raw runner for unmigrated code

    github       *github.PRClient
    provider     provider.Provider
    escalator    escalate.Escalator
    mergeMu      *sync.Mutex

    // Keep raw path for provider invocation (providers need filesystem path)
    worktreePath string
    branch       string
    currentTask  *discovery.Task

    reviewer     provider.Reviewer
    reviewConfig *config.CodeReviewConfig
    prNumber     int
}

// WorkerConfig bundles worker configuration options
type WorkerConfig struct {
    // ... existing fields
    WorktreeBase string          // Base directory for worktrees
    RepoRoot     string          // Repository root path
    AuditLogger  git.AuditLogger // Optional: log all git operations
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
    Events       *events.Bus
    Git          *git.WorktreeManager

    // Phase 1: Both GitOps and GitRunner supported
    // Phase 3: Only GitOps required
    GitOps    git.GitOps // Preferred: safe git interface
    GitRunner git.Runner // Deprecated: raw runner

    GitHub       *github.PRClient
    Provider     provider.Provider
    Escalator    escalate.Escalator
    MergeMu      *sync.Mutex
    Reviewer     provider.Reviewer
    ReviewConfig *config.CodeReviewConfig
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/worker/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Worker has gitOps field | `w.gitOps` accessible |
| WorkerDeps has GitOps field | `deps.GitOps` accessible |
| Existing code compiles | No breaking changes |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- This is an additive-only change; no existing behavior modified
- The gitRunner field remains for backward compatibility
- worktreePath is retained for provider invocation
- AuditLogger is optional in WorkerConfig

## NOT In Scope

- NewWorker modifications (Task #2)
- Method migrations (Tasks #3, #4)
- Test updates (Task #5)
