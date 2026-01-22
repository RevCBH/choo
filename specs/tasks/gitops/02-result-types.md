---
task: 2
status: pending
backpressure: "go build ./internal/git/..."
depends_on: [1]
---

# Result Types and Option Structs

**Parent spec**: `/specs/GITOPS.md`
**Task**: #2 of 7 in implementation plan

## Objective

Define result types (StatusResult, Commit) and operation option structs (CommitOpts, CleanOpts, PushOpts, MergeOpts, LogOpts).

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: error types, base types)

### Package Dependencies
- Standard library only (`time`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── gitops_types.go    # CREATE: Result types and option structs
```

### Types to Implement

```go
package git

import "time"

// StatusResult contains the parsed output of git status.
type StatusResult struct {
    Clean      bool     // True if the working tree has no changes
    Staged     []string // Files with staged changes
    Modified   []string // Files with unstaged modifications
    Untracked  []string // Untracked files
    Conflicted []string // Files with merge conflicts
}

// Commit represents a parsed git commit.
type Commit struct {
    Hash    string
    Author  string
    Date    time.Time
    Subject string
    Body    string
}

// CommitOpts configures commit behavior.
type CommitOpts struct {
    NoVerify   bool   // Skip pre-commit and commit-msg hooks
    Author     string // Override commit author (format: "Name <email>")
    AllowEmpty bool   // Permit creating commits with no changes
}

// CleanOpts configures git clean behavior.
type CleanOpts struct {
    Force       bool // -f flag (required for git clean to do anything)
    Directories bool // -d flag to remove untracked directories
    IgnoredOnly bool // -X flag to only remove ignored files
    IgnoredToo  bool // -x flag to remove ignored and untracked files
}

// PushOpts configures git push behavior.
type PushOpts struct {
    Force          bool // --force push (use with caution)
    SetUpstream    bool // -u flag to set upstream tracking
    ForceWithLease bool // --force-with-lease (safer than Force)
}

// MergeOpts configures git merge behavior.
type MergeOpts struct {
    FFOnly   bool   // Only allows fast-forward merges
    NoFF     bool   // Creates merge commit even for fast-forward merges
    Message  string // Merge commit message
    NoCommit bool   // Performs merge but stops before creating commit
}

// LogOpts configures git log output.
type LogOpts struct {
    MaxCount int       // Limits the number of commits returned
    Since    time.Time // Filters commits after this time
    Until    time.Time // Filters commits before this time
    Path     string    // Filters commits affecting this path
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
| StatusResult usable | `git.StatusResult{Clean: true}` compiles |
| CommitOpts usable | `git.CommitOpts{NoVerify: true}` compiles |
| All option structs have zero values | Default-initialized structs are valid |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All structs use zero values as sensible defaults
- CleanOpts.IgnoredOnly and IgnoredToo are mutually exclusive (runtime check in Clean implementation)
- PushOpts.Force and ForceWithLease are mutually exclusive (ForceWithLease takes precedence)

## NOT In Scope

- GitOps interface definition (Task #4)
- Implementation of any git operations
- Parsing logic for StatusResult (Task #5)
