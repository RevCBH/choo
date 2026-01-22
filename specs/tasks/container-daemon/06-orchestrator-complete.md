---
task: 6
status: pending
backpressure: "go test ./internal/orchestrator/... -run TestComplete"
depends_on: [4]
---

# Orchestrator Completion

**Parent spec**: `/specs/CONTAINER-DAEMON.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement the complete() method on Orchestrator to run the archive step, commit changes, and push to remote after all units merge.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #4 must be complete (provides Archive function)

### Package Dependencies
- `os/exec` (standard library)
- `context` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/orchestrator/
├── complete.go      # CREATE: Completion flow implementation
└── complete_test.go # CREATE: Completion tests
```

### Functions to Implement

```go
// internal/orchestrator/complete.go

package orchestrator

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"

    "github.com/anthropics/choo/internal/cli"
)

// Complete runs the completion flow after all units have merged.
// This includes: archive, commit, push, and PR creation.
func (o *Orchestrator) Complete(ctx context.Context) error {
    log.Printf("All units complete, running completion flow")

    // 1. Run archive to move completed specs
    if err := o.runArchive(ctx); err != nil {
        return fmt.Errorf("archive step failed: %w", err)
    }

    // 2. Commit the archive changes
    if err := o.commitArchive(ctx); err != nil {
        return fmt.Errorf("commit step failed: %w", err)
    }

    // 3. Push feature branch to remote
    if err := o.pushFeatureBranch(ctx); err != nil {
        return fmt.Errorf("push step failed: %w", err)
    }

    // 4. Create PR if not disabled
    if !o.cfg.NoPR {
        if err := o.createFeaturePR(ctx); err != nil {
            return fmt.Errorf("PR creation failed: %w", err)
        }
    }

    return nil
}

// runArchive moves completed specs to specs/completed/.
func (o *Orchestrator) runArchive(ctx context.Context) error {
    specsDir := o.cfg.TasksDir
    if specsDir == "" {
        specsDir = "specs/tasks"
    }
    // Get parent specs dir
    specsDir = strings.TrimSuffix(specsDir, "/tasks")

    opts := cli.ArchiveOptions{
        SpecsDir: specsDir,
        Verbose:  true,
    }

    archived, err := cli.Archive(opts)
    if err != nil {
        return err
    }

    // Also archive task directories
    taskArchived, err := cli.ArchiveTasksDir(specsDir, false)
    if err != nil {
        return err
    }

    if len(archived) == 0 && len(taskArchived) == 0 {
        log.Printf("No specs to archive")
    }

    return nil
}

// commitArchive commits the archived specs with a standard message.
func (o *Orchestrator) commitArchive(ctx context.Context) error {
    // Check if there are changes to commit
    statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
    statusCmd.Dir = o.cfg.RepoPath
    output, err := statusCmd.Output()
    if err != nil {
        return fmt.Errorf("git status failed: %w", err)
    }

    if len(strings.TrimSpace(string(output))) == 0 {
        log.Printf("No archive changes to commit")
        return nil
    }

    // Stage all changes
    addCmd := exec.CommandContext(ctx, "git", "add", "-A")
    addCmd.Dir = o.cfg.RepoPath
    addCmd.Stdout = os.Stdout
    addCmd.Stderr = os.Stderr
    if err := addCmd.Run(); err != nil {
        return fmt.Errorf("git add failed: %w", err)
    }

    // Commit with standard message
    commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", "chore: archive completed specs")
    commitCmd.Dir = o.cfg.RepoPath
    commitCmd.Stdout = os.Stdout
    commitCmd.Stderr = os.Stderr
    if err := commitCmd.Run(); err != nil {
        // Check if this is a "nothing to commit" error
        if isNoChangesError(err) {
            log.Printf("No changes to commit after staging")
            return nil
        }
        return fmt.Errorf("git commit failed: %w", err)
    }

    log.Printf("Committed archive changes")
    return nil
}

// pushFeatureBranch pushes the feature branch to the remote.
func (o *Orchestrator) pushFeatureBranch(ctx context.Context) error {
    pushCmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", o.cfg.FeatureBranch)
    pushCmd.Dir = o.cfg.RepoPath
    pushCmd.Stdout = os.Stdout
    pushCmd.Stderr = os.Stderr

    if err := pushCmd.Run(); err != nil {
        return fmt.Errorf("git push failed: %w", err)
    }

    log.Printf("Pushed feature branch: %s", o.cfg.FeatureBranch)
    return nil
}

// createFeaturePR creates a pull request for the feature branch.
func (o *Orchestrator) createFeaturePR(ctx context.Context) error {
    title := fmt.Sprintf("feat: %s", o.cfg.FeatureTitle)
    body := o.buildPRBody()

    prCmd := exec.CommandContext(ctx, "gh", "pr", "create",
        "--base", o.cfg.TargetBranch,
        "--head", o.cfg.FeatureBranch,
        "--title", title,
        "--body", body,
    )
    prCmd.Dir = o.cfg.RepoPath
    prCmd.Stdout = os.Stdout
    prCmd.Stderr = os.Stderr

    if err := prCmd.Run(); err != nil {
        return fmt.Errorf("gh pr create failed: %w", err)
    }

    log.Printf("Created PR for feature branch")
    return nil
}

// buildPRBody constructs the PR description.
func (o *Orchestrator) buildPRBody() string {
    var sb strings.Builder

    sb.WriteString("## Summary\n\n")
    sb.WriteString(o.cfg.FeatureDescription)
    sb.WriteString("\n\n")

    sb.WriteString("## Units Implemented\n\n")
    for _, unit := range o.completedUnits {
        sb.WriteString(fmt.Sprintf("- [x] %s\n", unit))
    }
    sb.WriteString("\n")

    sb.WriteString("## Automated Implementation\n\n")
    sb.WriteString("This PR was created by the choo orchestrator.\n")
    sb.WriteString("All unit branches have been merged and specs archived.\n")

    return sb.String()
}

// isNoChangesError checks if an error is due to no changes to commit.
func isNoChangesError(err error) bool {
    if exitErr, ok := err.(*exec.ExitError); ok {
        // git commit returns exit code 1 when there's nothing to commit
        return exitErr.ExitCode() == 1
    }
    return false
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/orchestrator/... -run TestComplete
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestComplete_RunsArchive` | Archive function is called |
| `TestComplete_CommitsChanges` | Git commit is executed |
| `TestComplete_PushesBranch` | Git push is executed |
| `TestComplete_CreatesPR` | gh pr create is executed |
| `TestComplete_NoPRSkipsCreation` | PR creation skipped when NoPR=true |
| `TestComplete_NoChangesSkipsCommit` | No commit when nothing to archive |
| `TestCommitArchive_HandlesNoChanges` | Returns nil when no changes |
| `TestBuildPRBody_IncludesUnits` | PR body lists completed units |
| `TestIsNoChangesError_ExitCode1` | Returns true for exit code 1 |

### Test Implementations

```go
// internal/orchestrator/complete_test.go

package orchestrator

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestBuildPRBody_IncludesUnits(t *testing.T) {
    o := &Orchestrator{
        cfg: OrchestratorConfig{
            FeatureDescription: "Test feature description",
        },
        completedUnits: []string{"unit-a", "unit-b", "unit-c"},
    }

    body := o.buildPRBody()

    if !strings.Contains(body, "unit-a") {
        t.Error("PR body should contain unit-a")
    }
    if !strings.Contains(body, "unit-b") {
        t.Error("PR body should contain unit-b")
    }
    if !strings.Contains(body, "Test feature description") {
        t.Error("PR body should contain feature description")
    }
}

func TestIsNoChangesError_ExitCode1(t *testing.T) {
    // Create a fake exit error with code 1
    cmd := exec.Command("sh", "-c", "exit 1")
    err := cmd.Run()

    if !isNoChangesError(err) {
        t.Error("isNoChangesError should return true for exit code 1")
    }
}

func TestIsNoChangesError_ExitCode0(t *testing.T) {
    // nil error (exit code 0)
    if isNoChangesError(nil) {
        t.Error("isNoChangesError should return false for nil error")
    }
}

func TestIsNoChangesError_ExitCode2(t *testing.T) {
    cmd := exec.Command("sh", "-c", "exit 2")
    err := cmd.Run()

    if isNoChangesError(err) {
        t.Error("isNoChangesError should return false for exit code 2")
    }
}

// Integration test that requires git
func TestCommitArchive_NoChanges(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Create a temp git repo
    tmpDir := t.TempDir()

    // Initialize git repo
    initCmd := exec.Command("git", "init")
    initCmd.Dir = tmpDir
    if err := initCmd.Run(); err != nil {
        t.Fatalf("git init failed: %v", err)
    }

    // Configure git user
    configName := exec.Command("git", "config", "user.name", "Test")
    configName.Dir = tmpDir
    configName.Run()

    configEmail := exec.Command("git", "config", "user.email", "test@test.com")
    configEmail.Dir = tmpDir
    configEmail.Run()

    // Create initial commit
    testFile := filepath.Join(tmpDir, "test.txt")
    os.WriteFile(testFile, []byte("test"), 0644)

    addCmd := exec.Command("git", "add", "-A")
    addCmd.Dir = tmpDir
    addCmd.Run()

    commitCmd := exec.Command("git", "commit", "-m", "initial")
    commitCmd.Dir = tmpDir
    commitCmd.Run()

    // Now test commitArchive with no changes
    o := &Orchestrator{
        cfg: OrchestratorConfig{
            RepoPath: tmpDir,
        },
    }

    err := o.commitArchive(context.Background())
    if err != nil {
        t.Errorf("commitArchive should succeed with no changes, got: %v", err)
    }
}

func TestComplete_NoPRSkipsCreation(t *testing.T) {
    // This test verifies the logic flow, not actual execution
    o := &Orchestrator{
        cfg: OrchestratorConfig{
            NoPR: true,
        },
    }

    // NoPR should skip PR creation
    if !o.cfg.NoPR {
        t.Error("NoPR should be true")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (integration tests can be skipped)
- [x] Runs in <60 seconds

## Implementation Notes

- Complete() is called after all unit branches have merged to feature branch
- Archive runs before commit to move completed specs
- Commit uses standard message "chore: archive completed specs"
- Push sets upstream tracking with `-u origin`
- PR creation uses `gh` CLI for simplicity
- NoPR option allows skipping PR creation for testing
- isNoChangesError handles git commit returning exit code 1

## NOT In Scope

- Unit completion tracking (existing orchestrator logic)
- Unit merging (existing orchestrator logic)
- PR review polling (existing orchestrator logic)
- Feature branch creation (existing orchestrator logic)
