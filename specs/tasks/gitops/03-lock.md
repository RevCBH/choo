---
task: 3
status: pending
backpressure: "go test ./internal/git/... -run TestRepoLock -v"
depends_on: []
---

# Per-Repo Write Lock

**Parent spec**: `/specs/GITOPS.md`
**Task**: #3 of 7 in implementation plan

## Objective

Implement a per-repository write lock to prevent concurrent workers from corrupting git state.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (independent of type definitions)

### Package Dependencies
- Standard library only (`sync`)

## Deliverables

### Files to Create/Modify

```
internal/git/
├── gitops_lock.go      # CREATE: Lock registry and getRepoLock function
└── gitops_lock_test.go # CREATE: Lock tests
```

### Functions to Implement

```go
package git

import "sync"

// repoLocks is a global lock registry keyed by canonical repo path.
var repoLocks = struct {
    sync.Mutex
    locks map[string]*sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

// getRepoLock returns (or creates) a mutex for the given repo path.
// The path must be canonical (absolute, resolved symlinks, cleaned).
func getRepoLock(path string) *sync.Mutex {
    repoLocks.Lock()
    defer repoLocks.Unlock()
    if repoLocks.locks[path] == nil {
        repoLocks.locks[path] = &sync.Mutex{}
    }
    return repoLocks.locks[path]
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -run TestRepoLock -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRepoLock_ReturnsSameLock` | Same path returns same mutex instance |
| `TestRepoLock_DifferentPaths` | Different paths return different mutexes |
| `TestRepoLock_ConcurrentAccess` | Concurrent getRepoLock calls don't race |

### Test Implementation

```go
// internal/git/gitops_lock_test.go
package git

import (
    "sync"
    "testing"
)

func TestRepoLock_ReturnsSameLock(t *testing.T) {
    lock1 := getRepoLock("/path/to/repo")
    lock2 := getRepoLock("/path/to/repo")

    if lock1 != lock2 {
        t.Error("expected same lock instance for same path")
    }
}

func TestRepoLock_DifferentPaths(t *testing.T) {
    lock1 := getRepoLock("/path/to/repo1")
    lock2 := getRepoLock("/path/to/repo2")

    if lock1 == lock2 {
        t.Error("expected different lock instances for different paths")
    }
}

func TestRepoLock_ConcurrentAccess(t *testing.T) {
    var wg sync.WaitGroup
    locks := make([]*sync.Mutex, 100)

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            locks[idx] = getRepoLock("/concurrent/test/path")
        }(i)
    }

    wg.Wait()

    // All should be the same lock
    for i := 1; i < 100; i++ {
        if locks[i] != locks[0] {
            t.Errorf("lock %d differs from lock 0", i)
        }
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Uses a global registry protected by its own mutex
- Locks are never removed from the registry (acceptable for typical usage patterns)
- Path must be canonical before calling getRepoLock (caller's responsibility)

## NOT In Scope

- Lock cleanup/pruning (future enhancement)
- Lock timeouts (future enhancement)
- Try-lock semantics (future enhancement)
