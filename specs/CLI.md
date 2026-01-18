# CLI — Command-Line Interface for Ralph Orchestrator

## Overview

The CLI package provides the command-line interface for choo using the Cobra framework. It serves as the entry point that wires together all other components: discovery, scheduler, workers, git, github, events, and config.

The CLI supports five commands: `run` (primary orchestration), `status` (progress display), `resume` (continue from last state), `cleanup` (remove worktrees and reset), and `version`. It handles signal interrupts for graceful shutdown and provides rich status output with progress bars and task status symbols.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              CLI (cobra)                                │
│                         choo <command>                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐  │
│   │   run    │  │  status  │  │  resume  │  │ cleanup  │  │ version │  │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬────┘  │
│        │             │             │             │             │        │
│        └─────────────┴──────┬──────┴─────────────┴─────────────┘        │
│                             │                                           │
│                    ┌────────▼────────┐                                  │
│                    │   Orchestrator  │                                  │
│                    │    Assembly     │                                  │
│                    └────────┬────────┘                                  │
│                             │                                           │
│   ┌─────────────────────────┼─────────────────────────────┐             │
│   ▼           ▼             ▼             ▼               ▼             │
│ Discovery  Scheduler     Worker       Git/GitHub       Events          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Parse and execute five subcommands: run, status, resume, cleanup, version
2. Support flags for parallelism control (-p/--parallelism)
3. Support target branch specification (-t/--target)
4. Support dry-run mode (-n/--dry-run) to show execution plan without running
5. Support single-unit mode (--unit) for backward compatibility with ralph.sh
6. Support PR-less execution (--no-pr) for local-only runs
7. Support skip-review mode (--skip-review) for auto-merge workflows
8. Display progress with per-unit progress bars and task status symbols
9. Handle SIGINT/SIGTERM for graceful shutdown
10. Support verbose output mode (-v/--verbose)
11. Load configuration from .choo.yaml when present
12. Wire together all orchestrator components at startup

### Performance Requirements

| Metric | Target |
|--------|--------|
| CLI startup time | <100ms |
| Status command latency | <500ms |
| Signal handling response | <1s to acknowledge |
| Progress update frequency | 100ms minimum interval |

### Constraints

- Depends on all other packages: discovery, scheduler, worker, git, github, events, config
- Must use Cobra v1.8+ for CLI framework
- Must support stdin/stdout/stderr for piping and scripting
- Must preserve exit codes for CI/CD integration

## Design

### Module Structure

```
internal/cli/
├── cli.go          # Root command and common setup
├── run.go          # run command implementation
├── status.go       # status command implementation
├── resume.go       # resume command implementation
├── cleanup.go      # cleanup command implementation
├── version.go      # version command implementation
├── display.go      # Progress bars, status formatting
├── signals.go      # Signal handling for graceful shutdown
└── wire.go         # Component assembly and dependency injection
```

### Core Types

```go
// internal/cli/cli.go

// App represents the CLI application with all wired dependencies
type App struct {
    // Root command
    rootCmd *cobra.Command

    // Wired components (initialized lazily)
    config   *config.Config
    events   *events.Bus

    // Runtime state
    cancel   context.CancelFunc
    shutdown chan struct{}
}

// RunOptions holds flags for the run command
type RunOptions struct {
    // Parallelism is the max concurrent units (default: 4)
    Parallelism int

    // TargetBranch is the branch PRs target (default: main)
    TargetBranch string

    // DryRun shows execution plan without running
    DryRun bool

    // NoPR skips PR creation, just executes tasks
    NoPR bool

    // Unit runs only the specified unit (single-unit mode)
    Unit string

    // SkipReview auto-merges without waiting for review
    SkipReview bool

    // TasksDir is the path to specs/tasks/ directory
    TasksDir string
}

// StatusOptions holds flags for the status command
type StatusOptions struct {
    // TasksDir is the path to specs/tasks/ directory
    TasksDir string

    // JSON outputs status as JSON instead of formatted text
    JSON bool
}

// DisplayConfig controls status output formatting
type DisplayConfig struct {
    // Width is the terminal width for progress bars
    Width int

    // UseColor enables ANSI color codes
    UseColor bool

    // ShowTimestamps includes timestamps in output
    ShowTimestamps bool
}
```

```go
// internal/cli/display.go

// UnitDisplay represents a unit's display state
type UnitDisplay struct {
    ID          string
    Status      discovery.UnitStatus
    Progress    float64  // 0.0 to 1.0
    Tasks       []TaskDisplay
    PRNumber    *int
    PRStatus    string   // "open", "merged", etc.
    BlockedBy   []string // unit IDs blocking this unit
}

// TaskDisplay represents a task's display state
type TaskDisplay struct {
    Number   int
    FileName string
    Status   discovery.TaskStatus
    Active   bool // true if currently executing
}

// StatusSymbol returns the appropriate symbol for a task status
type StatusSymbol string

const (
    SymbolComplete   StatusSymbol = "✓"
    SymbolInProgress StatusSymbol = "●"
    SymbolPending    StatusSymbol = "○"
    SymbolFailed     StatusSymbol = "✗"
    SymbolBlocked    StatusSymbol = "→"
)
```

```go
// internal/cli/signals.go

// SignalHandler manages graceful shutdown on interrupt
type SignalHandler struct {
    signals  chan os.Signal
    shutdown chan struct{}
    cancel   context.CancelFunc

    // Callbacks for cleanup
    onShutdown []func()
}
```

### API Surface

```go
// internal/cli/cli.go

// New creates a new CLI application
func New() *App

// Execute runs the CLI application
func (a *App) Execute() error

// SetVersion sets the version string for the version command
func (a *App) SetVersion(version, commit, date string)
```

```go
// internal/cli/run.go

// NewRunCmd creates the run command
func NewRunCmd(app *App) *cobra.Command

// RunOrchestrator executes the main orchestration loop
func (a *App) RunOrchestrator(ctx context.Context, opts RunOptions) error
```

```go
// internal/cli/status.go

// NewStatusCmd creates the status command
func NewStatusCmd(app *App) *cobra.Command

// ShowStatus displays the current orchestration status
func (a *App) ShowStatus(opts StatusOptions) error

// FormatUnitStatus formats a single unit's status for display
func FormatUnitStatus(unit *UnitDisplay, cfg DisplayConfig) string

// RenderProgressBar renders a progress bar of specified width
func RenderProgressBar(progress float64, width int) string
```

```go
// internal/cli/resume.go

// NewResumeCmd creates the resume command
func NewResumeCmd(app *App) *cobra.Command

// ResumeOrchestrator continues from the last saved state
func (a *App) ResumeOrchestrator(ctx context.Context, opts RunOptions) error
```

```go
// internal/cli/cleanup.go

// NewCleanupCmd creates the cleanup command
func NewCleanupCmd(app *App) *cobra.Command

// Cleanup removes worktrees and optionally resets state
func (a *App) Cleanup(tasksDir string, resetState bool) error
```

```go
// internal/cli/signals.go

// NewSignalHandler creates a signal handler with the given context cancel
func NewSignalHandler(cancel context.CancelFunc) *SignalHandler

// Start begins listening for signals
func (h *SignalHandler) Start()

// OnShutdown registers a callback to run on shutdown
func (h *SignalHandler) OnShutdown(fn func())

// Wait blocks until shutdown is triggered
func (h *SignalHandler) Wait()
```

```go
// internal/cli/wire.go

// WireOrchestrator assembles all components for orchestration
func WireOrchestrator(cfg *config.Config) (*Orchestrator, error)

// Orchestrator holds all wired components
type Orchestrator struct {
    Config    *config.Config
    Events    *events.Bus
    Discovery *discovery.Discovery
    Scheduler *scheduler.Scheduler
    Workers   *worker.Pool
    Git       *git.WorktreeManager
    GitHub    *github.PRClient
}
```

### Command Structure

```
choo
├── run [tasks-dir]           # Execute units in parallel
│   ├── -p, --parallelism     # Max concurrent units (default: 4)
│   ├── -t, --target          # Target branch for PRs (default: main)
│   ├── -n, --dry-run         # Show execution plan
│   ├── --no-pr               # Skip PR creation
│   ├── --unit                # Run only specified unit
│   └── --skip-review         # Auto-merge without review
│
├── status [tasks-dir]        # Show current progress
│   └── --json                # Output as JSON
│
├── resume [tasks-dir]        # Resume from last state
│   └── (inherits run flags)
│
├── cleanup [tasks-dir]       # Remove worktrees
│   └── --reset-state         # Also reset frontmatter status
│
├── version                   # Print version info
│
└── (global flags)
    ├── -v, --verbose         # Verbose output
    └── -h, --help            # Show help
```

### Status Output Format

The status command produces formatted output with progress bars and task status:

```
═══════════════════════════════════════════════════════════════
Ralph Orchestrator Status
Target: main | Parallelism: 4
═══════════════════════════════════════════════════════════════

 [app-shell] ████████████████████ 100% (complete)
   ✓ #1  01-nav-types.md
   ✓ #2  02-navigation.md
   ✓ #3  03-app-shell.md
   ✓ #4  04-route-setup.md
   PR #42 merged

 [deck-list] ████████░░░░░░░░░░░░  40% (in_progress)
   ✓ #1  01-deck-card.md
   ● #2  02-deck-grid.md         ← executing
   ○ #3  03-deck-page.md

 [config] ░░░░░░░░░░░░░░░░░░░░   0% (pending)
   → blocked by: project-setup

───────────────────────────────────────────────────────────────
 Units: 3 | Complete: 1 | In Progress: 1 | Pending: 1
 Tasks: 9 | Complete: 5 | In Progress: 1 | Pending: 3
═══════════════════════════════════════════════════════════════
```

### Signal Handling Flow

```
                    ┌─────────────┐
                    │   Running   │
                    └──────┬──────┘
                           │
                    SIGINT │ SIGTERM
                           ▼
                    ┌─────────────┐
                    │  Shutting   │
                    │    Down     │
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
   ┌───────────┐    ┌───────────┐    ┌───────────┐
   │  Cancel   │    │  Commit   │    │  Update   │
   │  Context  │    │  Progress │    │ Frontmat  │
   └───────────┘    └───────────┘    └───────────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           ▼
                    ┌─────────────┐
                    │    Exit    │
                    │   Code 130  │
                    └─────────────┘
```

## Implementation Notes

### Cobra Command Setup

The root command is configured with persistent flags that apply to all subcommands:

```go
func (a *App) setupRootCmd() {
    a.rootCmd = &cobra.Command{
        Use:   "choo",
        Short: "Parallel development task orchestrator",
        Long: `Ralph Orchestrator executes development units in parallel,
managing git worktrees and the full PR lifecycle.`,
        SilenceUsage:  true,
        SilenceErrors: true,
    }

    // Persistent flags
    a.rootCmd.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false,
        "Verbose output")

    // Add subcommands
    a.rootCmd.AddCommand(
        NewRunCmd(a),
        NewStatusCmd(a),
        NewResumeCmd(a),
        NewCleanupCmd(a),
        NewVersionCmd(a),
    )
}
```

### Graceful Shutdown

On SIGINT, the CLI must:
1. Cancel the context to stop new operations
2. Wait for the current Claude invocation to complete (if any)
3. Commit any in-progress work
4. Update frontmatter to preserve state
5. Exit with code 130 (standard for SIGINT)

```go
func (h *SignalHandler) Start() {
    signal.Notify(h.signals, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        sig := <-h.signals
        log.Printf("Received %v, initiating graceful shutdown...", sig)

        // Cancel context to stop new work
        h.cancel()

        // Run cleanup callbacks
        for _, fn := range h.onShutdown {
            fn()
        }

        close(h.shutdown)
    }()
}
```

### Progress Bar Rendering

Progress bars use Unicode block characters for smooth rendering:

```go
func RenderProgressBar(progress float64, width int) string {
    filled := int(progress * float64(width))
    empty := width - filled

    bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
    percent := int(progress * 100)

    return fmt.Sprintf("%s %3d%%", bar, percent)
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 130 | Interrupted (SIGINT) |
| 131 | Terminated (SIGTERM) |

### Component Wiring

The CLI lazily initializes components to avoid startup overhead for simple commands like `version`:

```go
func WireOrchestrator(cfg *config.Config) (*Orchestrator, error) {
    // Create event bus first (other components depend on it)
    bus := events.NewBus(1000)

    // Wire components with their dependencies
    disc := discovery.New()
    sched := scheduler.New(bus)
    gitMgr := git.NewWorktreeManager(cfg.WorktreeBase)
    ghClient := github.NewPRClient(cfg.GitHub)
    workers := worker.NewPool(cfg.Parallelism, bus, gitMgr)

    return &Orchestrator{
        Config:    cfg,
        Events:    bus,
        Discovery: disc,
        Scheduler: sched,
        Workers:   workers,
        Git:       gitMgr,
        GitHub:    ghClient,
    }, nil
}
```

## Testing Strategy

### Unit Tests

```go
// internal/cli/display_test.go

func TestRenderProgressBar(t *testing.T) {
    tests := []struct {
        name     string
        progress float64
        width    int
        want     string
    }{
        {
            name:     "empty bar",
            progress: 0.0,
            width:    10,
            want:     "░░░░░░░░░░   0%",
        },
        {
            name:     "half full",
            progress: 0.5,
            width:    10,
            want:     "█████░░░░░  50%",
        },
        {
            name:     "complete",
            progress: 1.0,
            width:    10,
            want:     "██████████ 100%",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := RenderProgressBar(tt.progress, tt.width)
            if got != tt.want {
                t.Errorf("RenderProgressBar() = %q, want %q", got, tt.want)
            }
        })
    }
}

func TestStatusSymbol(t *testing.T) {
    tests := []struct {
        status discovery.TaskStatus
        want   StatusSymbol
    }{
        {discovery.TaskStatusComplete, SymbolComplete},
        {discovery.TaskStatusInProgress, SymbolInProgress},
        {discovery.TaskStatusPending, SymbolPending},
        {discovery.TaskStatusFailed, SymbolFailed},
    }

    for _, tt := range tests {
        t.Run(string(tt.status), func(t *testing.T) {
            got := GetStatusSymbol(tt.status)
            if got != tt.want {
                t.Errorf("GetStatusSymbol(%v) = %v, want %v", tt.status, got, tt.want)
            }
        })
    }
}
```

```go
// internal/cli/signals_test.go

func TestSignalHandler_GracefulShutdown(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    handler := NewSignalHandler(cancel)

    callbackCalled := false
    handler.OnShutdown(func() {
        callbackCalled = true
    })

    handler.Start()

    // Simulate signal
    handler.signals <- syscall.SIGINT

    // Wait for shutdown
    select {
    case <-handler.shutdown:
        // Expected
    case <-time.After(time.Second):
        t.Fatal("shutdown not triggered within timeout")
    }

    if !callbackCalled {
        t.Error("shutdown callback was not called")
    }

    if ctx.Err() == nil {
        t.Error("context was not cancelled")
    }
}
```

```go
// internal/cli/run_test.go

func TestRunOptions_Validation(t *testing.T) {
    tests := []struct {
        name    string
        opts    RunOptions
        wantErr bool
    }{
        {
            name: "valid options",
            opts: RunOptions{
                Parallelism:  4,
                TargetBranch: "main",
                TasksDir:     "./specs/tasks",
            },
            wantErr: false,
        },
        {
            name: "invalid parallelism",
            opts: RunOptions{
                Parallelism: 0,
                TasksDir:    "./specs/tasks",
            },
            wantErr: true,
        },
        {
            name: "missing tasks dir",
            opts: RunOptions{
                Parallelism: 4,
                TasksDir:    "",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.opts.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Run command with dry-run | Fixture repo with tasks, verify output shows plan without executing |
| Status command | Fixture with mixed task states, verify formatting matches spec |
| Resume command | Fixture with partially complete unit, verify it continues from saved state |
| Cleanup command | Create worktrees, run cleanup, verify removal |
| Signal handling | Start run, send SIGINT, verify graceful shutdown and state preservation |

### Manual Testing

- [ ] `choo run -n` shows correct execution plan
- [ ] `choo status` displays progress bars correctly at various widths
- [ ] `choo run --unit foo` runs only the specified unit
- [ ] Ctrl+C during execution triggers graceful shutdown
- [ ] `choo version` displays version information
- [ ] `choo cleanup` removes all worktrees
- [ ] Invalid flags produce helpful error messages

## Design Decisions

### Why Cobra for CLI Framework?

Cobra is the de facto standard for Go CLIs (used by kubectl, hugo, gh). It provides:
- Automatic help generation
- Flag parsing with short and long forms
- Subcommand nesting
- Shell completion generation
- Consistent UX patterns

Alternatives considered: urfave/cli (less common), kong (reflection-heavy), standard flag package (too basic for subcommands).

### Why Lazy Component Initialization?

Commands like `version` and `--help` should be instant. Wiring all components at startup would add latency and require valid configuration for every invocation. Lazy initialization defers component creation until actually needed.

### Why Unicode Progress Bars?

Modern terminals universally support Unicode block characters. They provide smoother progress visualization than ASCII alternatives like `[=====>    ]`. The trade-off (potential issues in non-Unicode terminals) is acceptable given the target audience of developers.

### Why Exit Code 130 for SIGINT?

This is the Unix convention: 128 + signal number. SIGINT is signal 2, so 128 + 2 = 130. CI/CD systems and scripts expect this convention for proper interrupt handling.

## Future Enhancements

1. Shell completion generation (`choo completion bash/zsh/fish`)
2. JSON output mode for all commands (machine-readable)
3. Interactive mode with live-updating status display
4. Configuration wizard (`choo init`)
5. Log file output with rotation

## References

- [PRD Section 8: CLI Interface](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [Cobra Documentation](https://cobra.dev/)
- [Go Signal Handling](https://pkg.go.dev/os/signal)
- [Unix Exit Codes](https://tldp.org/LDP/abs/html/exitcodes.html)
