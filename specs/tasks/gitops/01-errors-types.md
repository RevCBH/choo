---
task: 1
status: complete
backpressure: "go build ./internal/git/..."
depends_on: []
---

# Error Types and Option Types

**Parent spec**: `/specs/GITOPS.md`
**Task**: #1 of 7 in implementation plan

## Objective

Define all error variables and configuration types for the GitOps interface.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- Standard library only (`errors`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── gitops_opts.go    # CREATE: Error types, GitOpsOpts, BranchGuard, SafetyLevel, AuditEntry
```

### Types to Implement

```go
package git

import (
    "errors"
    "time"
)

// Path errors
var (
    ErrEmptyPath          = errors.New("git: path cannot be empty")
    ErrRelativePath       = errors.New("git: path must be absolute")
    ErrNonCanonicalPath   = errors.New("git: path must be canonical")
    ErrPathNotFound       = errors.New("git: path not found")
    ErrNotDirectory       = errors.New("git: path is not a directory")
    ErrNotGitRepo         = errors.New("git: path is not a git repository")
    ErrPathMismatch       = errors.New("git: path does not match git toplevel")
    ErrRepoRootNotAllowed = errors.New("git: repo root not allowed (use AllowRepoRoot)")
    ErrOutsideWorktreeBase = errors.New("git: path outside worktree base")
)

// Branch/remote errors
var (
    ErrUnexpectedBranch   = errors.New("git: unexpected branch")
    ErrUnexpectedRemote   = errors.New("git: unexpected remote URL")
    ErrProtectedBranch    = errors.New("git: cannot write to protected branch")
)

// Operation errors
var (
    ErrDestructiveNotAllowed = errors.New("git: destructive operation not allowed")
    ErrConcurrentWrite       = errors.New("git: concurrent write operation in progress")
)

// GitOpsOpts configures GitOps safety behavior.
type GitOpsOpts struct {
    WorktreeBase     string       // Required path prefix (e.g., "/tmp/ralph-worktrees")
    AllowRepoRoot    bool         // Allow operating on repo root (not just worktrees)
    BranchGuard      *BranchGuard // Branch and remote validation rules
    AllowDestructive bool         // Must be true for ResetHard, Clean, CheckoutFiles, force push
    SafetyLevel      SafetyLevel  // Strict, Default, or Relaxed
    AuditLogger      AuditLogger  // Optional structured logger for all operations
}

// SafetyLevel controls the aggressiveness of safety checks.
type SafetyLevel int

const (
    SafetyStrict  SafetyLevel = iota // All checks enabled, AllowDestructive=false
    SafetyDefault                     // Path validation + runtime checks, no branch guard
    SafetyRelaxed                     // Path validation only (for tests)
)

// BranchGuard enforces branch/remote constraints on write operations.
type BranchGuard struct {
    ExpectedBranch        string   // Requires HEAD to match this exact branch name
    AllowedBranchPrefixes []string // Allows HEAD to match any of these prefixes
    AllowedRemotes        []string // Restricts Push/Fetch to these remote URLs only
    ProtectedBranches     []string // Blocks all write operations when HEAD is on these branches (default: main, master)
}

// AuditEntry represents a structured log of a git operation.
type AuditEntry struct {
    Timestamp     time.Time     `json:"ts"`
    Operation     string        `json:"op"`
    RepoPath      string        `json:"repo_path"`
    Branch        string        `json:"branch,omitempty"`
    Remote        string        `json:"remote,omitempty"`
    Args          []string      `json:"args,omitempty"`
    SafetyChecks  []string      `json:"safety_checks"`
    ChecksPassed  bool          `json:"checks_passed"`
    FailureReason string        `json:"failure_reason,omitempty"`
    Duration      time.Duration `json:"duration_ms"`
    Error         string        `json:"error,omitempty"`
}

// AuditLogger receives audit entries for all git operations.
type AuditLogger interface {
    Log(entry AuditEntry)
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/git/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| Error variables accessible | `git.ErrEmptyPath != nil` |
| Types usable | `git.GitOpsOpts{}` compiles |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All error variables are package-level `var` declarations using `errors.New`
- Use `errors.Is()` to check error types (supports wrapped errors)
- BranchGuard defaults ProtectedBranches to `["main", "master"]` if empty (checked at runtime)

## NOT In Scope

- GitOps interface definition (Task #4)
- Implementation of any git operations
- Unit tests (compile check only for this task)
