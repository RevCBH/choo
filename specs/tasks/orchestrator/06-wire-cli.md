---
task: 6
status: pending
backpressure: "go build ./cmd/choo/... && go test ./internal/cli/... -run TestRun"
depends_on: [1, 2, 3, 4, 5]
---

# Wire Orchestrator into CLI

**Parent spec**: `/specs/ORCHESTRATOR.md`
**Task**: #6 of 6 in implementation plan

## Objective

Replace the TODO placeholders in the CLI run command with actual orchestrator integration.

## Dependencies

### External Specs (must be implemented)
- CLI - provides App, RunOptions, NewRunCmd
- CONFIG - provides LoadConfig
- All orchestrator tasks (#1-5)

### Task Dependencies (within this unit)
- Task #1 - Core types must be defined
- Task #2 - Run() method must be implemented
- Task #3 - Event handling must be implemented
- Task #4 - Shutdown must be implemented
- Task #5 - Dry-run must be implemented

### Package Dependencies
- `github.com/anthropics/choo/internal/cli`
- `github.com/anthropics/choo/internal/config`
- `github.com/anthropics/choo/internal/orchestrator`
- `github.com/anthropics/choo/internal/escalate`

## Deliverables

### Files to Create/Modify
```
internal/cli/
├── run.go      # MODIFY: Replace TODOs with orchestrator calls
└── run_test.go # MODIFY: Add integration tests
```

### Code Changes

Replace the `RunOrchestrator` method in `internal/cli/run.go`:

```go
// RunOrchestrator executes the main orchestration loop
func (a *App) RunOrchestrator(ctx context.Context, opts RunOptions) error {
	// Validate options (defensive)
	if err := opts.Validate(); err != nil {
		return err
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handler
	handler := NewSignalHandler(cancel)
	handler.OnShutdown(func() {
		fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
	})
	handler.Start()
	defer handler.Stop()

	// Load configuration
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfig(wd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create event bus
	eventBus := events.NewBus(1000)
	defer eventBus.Close()

	// Create Git WorktreeManager
	gitManager := git.NewWorktreeManager(wd, nil)

	// Create GitHub PRClient
	pollInterval, _ := cfg.ReviewPollIntervalDuration()
	reviewTimeout, _ := cfg.ReviewTimeoutDuration()
	ghClient, err := github.NewPRClient(github.PRClientConfig{
		Owner:         cfg.GitHub.Owner,
		Repo:          cfg.GitHub.Repo,
		PollInterval:  pollInterval,
		ReviewTimeout: reviewTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Create escalator (terminal by default)
	esc := escalate.NewTerminal()

	// Build orchestrator config from CLI options and loaded config
	orchCfg := orchestrator.Config{
		Parallelism:     opts.Parallelism,
		TargetBranch:    opts.TargetBranch,
		TasksDir:        opts.TasksDir,
		RepoRoot:        wd,
		WorktreeBase:    cfg.Worktree.BasePath,
		NoPR:            opts.NoPR,
		SkipReview:      opts.SkipReview,
		SingleUnit:      opts.Unit,
		DryRun:          opts.DryRun,
		ShutdownTimeout: orchestrator.DefaultShutdownTimeout,
	}

	// Create orchestrator
	orch := orchestrator.New(orchCfg, orchestrator.Dependencies{
		Bus:       eventBus,
		Escalator: esc,
		Git:       gitManager,
		GitHub:    ghClient,
	})
	defer orch.Close()

	// Run orchestrator
	result, err := orch.Run(ctx)

	// Print summary
	if result != nil {
		fmt.Printf("\nOrchestration complete:\n")
		fmt.Printf("  Total units:     %d\n", result.TotalUnits)
		fmt.Printf("  Completed:       %d\n", result.CompletedUnits)
		fmt.Printf("  Failed:          %d\n", result.FailedUnits)
		fmt.Printf("  Blocked:         %d\n", result.BlockedUnits)
		fmt.Printf("  Duration:        %s\n", result.Duration.Round(time.Millisecond))
	}

	return err
}
```

Add necessary imports to `run.go`:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/choo/internal/config"
	"github.com/anthropics/choo/internal/escalate"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
	"github.com/anthropics/choo/internal/orchestrator"
	"github.com/spf13/cobra"
)
```

### Tests to Implement

```go
// Add to internal/cli/run_test.go

func TestRunOrchestrator_DryRun(t *testing.T) {
	// Create temp directory with tasks
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	// Change to tmpDir for config loading
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewApp()
	opts := RunOptions{
		TasksDir:    tasksDir,
		Parallelism: 1,
		DryRun:      true,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	err := app.RunOrchestrator(ctx, opts)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Execution Plan") {
		t.Error("expected dry-run output")
	}
}

func TestRunOrchestrator_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "sleep 60"
depends_on: []
---
# Long Task
`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewApp()
	opts := RunOptions{
		TasksDir:    tasksDir,
		Parallelism: 1,
	}

	// Cancel after short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := app.RunOrchestrator(ctx, opts)

	// Should return context cancellation
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRunOrchestrator_InvalidTasksDir(t *testing.T) {
	app := NewApp()
	opts := RunOptions{
		TasksDir:    "/nonexistent/path",
		Parallelism: 1,
	}

	ctx := context.Background()
	err := app.RunOrchestrator(ctx, opts)

	if err == nil {
		t.Error("expected error for invalid tasks directory")
	}
}

func TestRunCmd_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	unitDir := filepath.Join(tasksDir, "test-unit")
	os.MkdirAll(unitDir, 0755)

	os.WriteFile(filepath.Join(unitDir, "IMPLEMENTATION_PLAN.md"), []byte(`---
unit: test-unit
depends_on: []
---
# Test Unit
`), 0644)

	os.WriteFile(filepath.Join(unitDir, "01-task.md"), []byte(`---
task: 1
status: pending
backpressure: "echo ok"
depends_on: []
---
# Task 1
`), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewApp()
	cmd := NewRunCmd(app)

	// Set args for dry-run
	cmd.SetArgs([]string{tasksDir, "--dry-run"})

	// Capture output
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
}
```

## Backpressure

### Validation Command
```bash
go build ./cmd/choo/... && go test ./internal/cli/... -run TestRun
```

### Must Pass
| Test | Assertion |
|------|-----------|
| TestRunOrchestrator_DryRun | Dry-run mode prints execution plan |
| TestRunOrchestrator_ContextCancellation | Context cancellation triggers shutdown |
| TestRunOrchestrator_InvalidTasksDir | Invalid directory returns error |
| TestRunCmd_Integration | CLI command executes successfully |
| `go build ./cmd/choo/...` | Binary compiles without errors |

## NOT In Scope
- Resume command integration
- Status command integration
- Cleanup command integration
- TUI display updates
