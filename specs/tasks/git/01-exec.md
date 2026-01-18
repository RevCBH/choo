---
task: 1
status: pending
backpressure: "go test ./internal/git/..."
depends_on: []
---

# Git Command Execution Utilities

**Parent spec**: `/specs/GIT.md`
**Task**: #1 of 6 in implementation plan

## Objective

Implement the core git command execution utility that all other git operations depend on.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- None (this is the foundation task)

### Package Dependencies
- Standard library only (`os/exec`, `bytes`, `context`)

## Deliverables

### Files to Create/Modify

```
internal/git/
└── exec.go    # CREATE: Git command execution utilities
```

### Functions to Implement

```go
// gitExec executes a git command in the specified directory and returns stdout.
// Returns an error with stderr content if the command fails.
func gitExec(ctx context.Context, dir string, args ...string) (string, error)

// gitExecWithStdin executes a git command with stdin input.
// Used for commands that require piped input.
func gitExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error)
```

### Implementation Notes

From the design spec:

```go
func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = dir

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git %s failed: %w\nstderr: %s",
            strings.Join(args, " "), err, stderr.String())
    }

    return stdout.String(), nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/git/... -v -run TestGitExec
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGitExec_Success` | Returns stdout on successful command |
| `TestGitExec_Failure` | Returns error with stderr on failed command |
| `TestGitExec_Context` | Respects context cancellation |
| `TestGitExec_Directory` | Executes in specified directory |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Temp git repo | Created in test | Test real git commands |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## NOT In Scope

- Worktree operations (Task #2)
- Branch naming (Task #3)
- Commit operations (Task #4)
- Merge operations (Task #5, #6)
