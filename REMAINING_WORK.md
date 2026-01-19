# Remaining Work for MVP

This document outlines the work remaining to achieve a functional MVP for choo. The MVP will execute the full end-to-end flow: discovery → scheduling → worker execution → PR lifecycle.

## Completed in This PR ✅

### Claude Client Package (`internal/claude/`)
- ✅ **client.go** - Subprocess wrapper for Claude CLI with context/timeout support
- ✅ **types.go** - ExecuteOptions and ExecuteResult structures
- ✅ **errors.go** - Error types (ErrEmptyPrompt, ErrTimeout, ExecutionError, etc.)
- ✅ **Tests** - Full test coverage for client, types, and errors
- ✅ **Worker Integration** - Updated worker package to use real Claude client
- ✅ **CLI Wiring** - Fixed WireOrchestrator to properly initialize worker pool with Claude client

## Critical Gaps Remaining ⚠️

### 1. Run Command Orchestration Logic (HIGHEST PRIORITY)

**File**: `internal/cli/run.go:118-140`

**Current State**: Three explicit TODOs mark where orchestration should happen:
```go
// TODO: Wire orchestrator components (task #9)
// TODO: Run discovery (task #9)
// TODO: Execute scheduler loop (task #9)
```

After dry-run validation, the command just waits for cancellation and returns nil.

**What's Needed**:
```go
// 1. Load config
cfg, err := loadConfig(opts.tasksDir)
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// 2. Wire orchestrator
orch, err := WireOrchestrator(cfg)
if err != nil {
    return fmt.Errorf("failed to wire orchestrator: %w", err)
}
defer orch.Close()

// 3. Discover units
units, err := orch.Discovery.Discover(opts.tasksDir)
if err != nil {
    return fmt.Errorf("discovery failed: %w", err)
}

// 4. Initialize scheduler with discovered units
if err := orch.Scheduler.Schedule(units); err != nil {
    return fmt.Errorf("scheduler initialization failed: %w", err)
}

// 5. Main execution loop
for {
    // Get ready units from scheduler
    readyUnits := orch.Scheduler.GetReadyUnits()

    if len(readyUnits) == 0 {
        // Check if we're done
        if orch.Scheduler.AllComplete() {
            break
        }

        // Check for deadlock/failure
        if orch.Scheduler.HasFailures() {
            return fmt.Errorf("execution failed: some units failed")
        }

        // Wait for work to complete
        time.Sleep(1 * time.Second)
        continue
    }

    // Submit ready units to worker pool
    for _, unit := range readyUnits {
        if err := orch.Workers.Submit(unit); err != nil {
            return fmt.Errorf("failed to submit unit %s: %w", unit.ID, err)
        }
        // Mark unit as dispatched in scheduler
        orch.Scheduler.MarkDispatched(unit.ID)
    }
}

// 6. Wait for all workers to complete
if err := orch.Workers.Wait(); err != nil {
    return fmt.Errorf("worker execution failed: %w", err)
}
```

**Estimated Lines**: ~150 LOC
**Estimated Time**: 2-3 hours

---

### 2. Scheduler State Management Methods

**Files**: `internal/scheduler/scheduler.go`, `internal/scheduler/state.go`

**Current State**: Scheduler has core DAG building and ready queue logic, but missing some orchestration helpers:

**What's Needed**:
- `GetReadyUnits() []*discovery.Unit` - Returns units ready for dispatch
- `MarkDispatched(unitID string)` - Marks unit as dispatched to worker pool
- `AllComplete() bool` - Returns true if all units are complete
- `HasFailures() bool` - Returns true if any units failed

These might already exist in slightly different forms - needs verification against current scheduler API.

**Estimated Lines**: ~50-100 LOC
**Estimated Time**: 1-2 hours

---

### 3. Worker Test Failures

**Files**: `internal/worker/*_test.go`

**Current State**: 11 tests failing with git worktree errors:
```
failed to commit: exit status 128
```

**Root Cause**: Test worktree setup issues - git operations in test environment not properly initialized.

**Options**:
1. Fix test worktree initialization (set git config, proper repo setup)
2. Mock git operations in tests
3. Skip failing tests temporarily and rely on integration tests

**Estimated Time**: 1-2 hours (depending on approach)

---

### 4. Resume Command Implementation

**File**: `internal/cli/resume.go:56`

**Current State**: Stub with TODO:
```go
// TODO: Implement full frontmatter parsing
```

**What's Needed**:
- Parse unit frontmatter to extract `orch_status` and `pr_number`
- Restore scheduler state from saved frontmatter
- Resume worker execution from where it left off
- Handle edge cases (stale worktrees, incomplete PRs)

**Estimated Lines**: ~200 LOC
**Estimated Time**: 2-3 hours

---

## Secondary Improvements (Post-MVP)

### 5. Baseline Checks Configuration
- Currently hardcoded in worker config
- Should load from .choo.yaml baseline_checks field
- Low priority - defaults work for MVP

### 6. Event Handlers
- Event bus exists but no handlers registered
- Could add logging/monitoring handlers
- Not blocking for MVP - events work without handlers

### 7. PR Review Polling
- GitHub PR review logic implemented but not called
- Should integrate into worker post-PR-creation phase
- Can be added after basic execution works

### 8. Error Recovery
- Current error handling stops on first failure
- Could add retry logic, partial recovery
- MVP can fail fast, improve later

---

## Validation Checklist

Once the above critical gaps are addressed, validate with:

1. **Unit Tests**
   ```bash
   go test ./...
   ```

2. **Build Check**
   ```bash
   go build ./cmd/choo
   ```

3. **Dry-Run Test**
   ```bash
   ./choo run --dry-run
   # Should display execution plan from specs/tasks/
   ```

4. **End-to-End Test**
   - Create simple test unit in `specs/tasks/test-unit/IMPLEMENTATION_PLAN.md`
   - Run `./choo run`
   - Verify: discovery → scheduling → worker execution → git commits → PR creation
   - Check GitHub for created PR

5. **Resume Test**
   - Interrupt running execution with Ctrl+C
   - Run `./choo resume`
   - Verify execution continues from saved state

---

## Estimated Total Time to MVP

| Component | Estimated Time |
|-----------|----------------|
| Run command orchestration | 2-3 hours |
| Scheduler state methods | 1-2 hours |
| Worker test fixes | 1-2 hours |
| Resume command | 2-3 hours |
| Validation & debugging | 1-2 hours |
| **Total** | **7-12 hours** |

---

## Architecture Status Summary

```
✅ COMPLETE
├── discovery (scans specs/tasks/*, parses frontmatter)
├── config (loads .choo.yaml, env overrides)
├── events (event bus for component coordination)
├── github (PR operations, review polling, merge)
├── git (worktree management, commits, merge with conflicts)
├── scheduler (DAG building, dependency resolution, ready queue)
└── claude (NEW: subprocess wrapper for Claude CLI)

⚠️  INCOMPLETE
├── cli/run.go (orchestration logic missing - 3 TODOs)
├── cli/resume.go (frontmatter parsing incomplete)
├── scheduler (minor: needs orchestration helper methods)
└── worker (tests failing, but core execution logic complete)
```

---

## Next Steps

**Immediate priority** (to unblock MVP):
1. Implement run command orchestration logic (internal/cli/run.go)
2. Add scheduler orchestration helpers if needed
3. Fix or skip worker tests
4. Manual end-to-end testing

**Follow-up** (post-MVP):
1. Complete resume command
2. Add event handlers for logging/monitoring
3. Integrate PR review polling
4. Improve error recovery
5. Add baseline checks configuration
