# FEATURE-CLI — CLI Commands for Feature Workflow

## Overview

FEATURE-CLI provides the command-line interface for managing feature development workflows. It exposes three commands under the `choo feature` namespace:

- `choo feature start` - Create feature branch, generate specs with review, generate tasks
- `choo feature status` - Show status of feature workflows
- `choo feature resume` - Resume blocked feature workflows

These commands orchestrate the end-to-end workflow from PRD to committed specs and tasks, managing state transitions and providing resume capability for blocked states.

## Requirements

### Functional Requirements

1. **Feature Start Command**
   - Read PRD from configurable directory (default: `docs/prds/<prd-id>.md`)
   - Create feature branch `feature/<prd-id>` from main
   - Track workflow state via PRD frontmatter `feature_status` field
   - Invoke spec-generator, spec-reviewer (with iteration loop), spec-validator, and task-generator agents
   - Commit generated specs and tasks to feature branch
   - Support dry-run mode showing plan without execution

2. **Feature Status Command**
   - Display status of all in-progress features when no argument provided
   - Display detailed status for specific feature when PRD ID provided
   - Show branch name, timestamps, review iteration count, spec/task counts
   - Indicate blocked states with actionable next steps
   - Support JSON output format

3. **Feature Resume Command**
   - Resume workflow from `review_blocked` state
   - Support skipping remaining review iterations
   - Support resuming from validation or task generation phases
   - Continue workflow to completion or next blocked state

### Performance Requirements

1. **Startup Time**: Command parsing and help display must complete in <100ms
2. **Status Display**: Status command must render in <500ms for up to 50 features
3. **Agent Invocation**: Agent processes should stream output to terminal in real-time

### Constraints

1. Must integrate with existing `internal/cli` App pattern
2. Must use Cobra for command structure
3. PRD frontmatter updates must be atomic (read-modify-write with validation)
4. Git operations must validate clean working tree before branch operations
5. Review loop must respect max iteration limit to prevent infinite loops

## Design

### Module Structure

```
internal/
├── cli/
│   ├── feature.go           # Parent command, subcommand registration
│   ├── feature_start.go     # Start command implementation
│   ├── feature_status.go    # Status command implementation
│   └── feature_resume.go    # Resume command implementation
└── feature/
    ├── workflow.go          # Workflow state machine
    ├── state.go             # State types and transitions
    └── prd.go               # PRD parsing and frontmatter updates
```

### Core Types

```go
// internal/feature/state.go

// FeatureStatus represents the current state of a feature workflow
type FeatureStatus string

const (
    StatusNotStarted      FeatureStatus = "not_started"
    StatusGeneratingSpecs FeatureStatus = "generating_specs"
    StatusReviewingSpecs  FeatureStatus = "reviewing_specs"
    StatusReviewBlocked   FeatureStatus = "review_blocked"
    StatusValidatingSpecs FeatureStatus = "validating_specs"
    StatusGeneratingTasks FeatureStatus = "generating_tasks"
    StatusSpecsCommitted  FeatureStatus = "specs_committed"
)

// FeatureState holds the complete state of a feature workflow
type FeatureState struct {
    PRDID            string        `yaml:"prd_id"`
    Status           FeatureStatus `yaml:"feature_status"`
    Branch           string        `yaml:"branch"`
    StartedAt        time.Time     `yaml:"started_at"`
    ReviewIterations int           `yaml:"review_iterations"`
    MaxReviewIter    int           `yaml:"max_review_iter"`
    LastFeedback     string        `yaml:"last_feedback,omitempty"`
    SpecCount        int           `yaml:"spec_count,omitempty"`
    TaskCount        int           `yaml:"task_count,omitempty"`
}

// ValidTransitions defines allowed state transitions
var ValidTransitions = map[FeatureStatus][]FeatureStatus{
    StatusNotStarted:      {StatusGeneratingSpecs},
    StatusGeneratingSpecs: {StatusReviewingSpecs, StatusReviewBlocked},
    StatusReviewingSpecs:  {StatusValidatingSpecs, StatusReviewBlocked},
    StatusReviewBlocked:   {StatusReviewingSpecs, StatusValidatingSpecs, StatusGeneratingTasks},
    StatusValidatingSpecs: {StatusGeneratingTasks},
    StatusGeneratingTasks: {StatusSpecsCommitted},
}
```

```go
// internal/feature/workflow.go

// Workflow manages the feature development state machine
type Workflow struct {
    git       GitOperations
    agents    AgentRunner
    prdStore  PRDStore
    specsDir  string
}

// WorkflowResult contains the outcome of a workflow execution
type WorkflowResult struct {
    FinalStatus   FeatureStatus
    SpecsGenerated int
    TasksGenerated int
    Blocked        bool
    BlockReason    string
}

// NewWorkflow creates a workflow manager
func NewWorkflow(git GitOperations, agents AgentRunner, prdStore PRDStore, specsDir string) *Workflow

// Start begins the feature workflow from a PRD
func (w *Workflow) Start(ctx context.Context, prdID string, opts StartOptions) (*WorkflowResult, error)

// Resume continues a blocked workflow
func (w *Workflow) Resume(ctx context.Context, prdID string, opts ResumeOptions) (*WorkflowResult, error)

// StartOptions configures workflow start behavior
type StartOptions struct {
    SkipSpecReview bool
    MaxReviewIter  int
    DryRun         bool
}

// ResumeOptions configures workflow resume behavior
type ResumeOptions struct {
    SkipReview     bool
    FromValidation bool
    FromTasks      bool
}
```

```go
// internal/feature/prd.go

// PRDStore handles PRD file operations
type PRDStore struct {
    baseDir string
}

// PRDMetadata represents parsed PRD frontmatter
type PRDMetadata struct {
    Title         string        `yaml:"title"`
    FeatureStatus FeatureStatus `yaml:"feature_status,omitempty"`
    Branch        string        `yaml:"branch,omitempty"`
    StartedAt     *time.Time    `yaml:"started_at,omitempty"`
    // Additional fields preserved during updates
    Extra         map[string]interface{} `yaml:",inline"`
}

// NewPRDStore creates a PRD store
func NewPRDStore(baseDir string) *PRDStore

// Load reads and parses a PRD file
func (s *PRDStore) Load(prdID string) (*PRDMetadata, string, error)

// UpdateStatus atomically updates PRD frontmatter status
func (s *PRDStore) UpdateStatus(prdID string, status FeatureStatus) error

// UpdateState atomically updates full PRD frontmatter state
func (s *PRDStore) UpdateState(prdID string, state FeatureState) error
```

### API Surface

```go
// internal/cli/feature.go

// NewFeatureCmd creates the feature parent command
func NewFeatureCmd(app *App) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "feature",
        Short: "Manage feature development workflows",
        Long:  `Commands for starting, monitoring, and resuming feature workflows from PRDs.`,
    }

    cmd.AddCommand(
        NewFeatureStartCmd(app),
        NewFeatureStatusCmd(app),
        NewFeatureResumeCmd(app),
    )

    return cmd
}
```

```go
// internal/cli/feature_start.go

// FeatureStartOptions holds flags for the feature start command
type FeatureStartOptions struct {
    PRDID          string
    PRDDir         string
    SpecsDir       string
    SkipSpecReview bool
    MaxReviewIter  int
    DryRun         bool
}

// NewFeatureStartCmd creates the feature start command
func NewFeatureStartCmd(app *App) *cobra.Command

// RunFeatureStart executes the feature start workflow
func (a *App) RunFeatureStart(ctx context.Context, opts FeatureStartOptions) error
```

```go
// internal/cli/feature_status.go

// FeatureStatusOptions holds flags for the feature status command
type FeatureStatusOptions struct {
    PRDID  string // optional, shows all if empty
    JSON   bool
}

// NewFeatureStatusCmd creates the feature status command
func NewFeatureStatusCmd(app *App) *cobra.Command

// ShowFeatureStatus displays feature workflow status
func (a *App) ShowFeatureStatus(opts FeatureStatusOptions) error
```

```go
// internal/cli/feature_resume.go

// FeatureResumeOptions holds flags for the feature resume command
type FeatureResumeOptions struct {
    PRDID          string
    SkipReview     bool
    FromValidation bool
    FromTasks      bool
}

// NewFeatureResumeCmd creates the feature resume command
func NewFeatureResumeCmd(app *App) *cobra.Command

// RunFeatureResume continues a blocked feature workflow
func (a *App) RunFeatureResume(ctx context.Context, opts FeatureResumeOptions) error
```

### Command Structure

#### `choo feature start`

```
choo feature start <prd-id> [flags]

Create feature branch, generate specs with review, generate tasks, commit to branch.

Arguments:
  prd-id    The PRD ID to start (e.g., "streaming-events")

Flags:
  --prd-dir string        PRDs directory (default: docs/prds)
  --specs-dir string      Output specs directory (default: specs/tasks)
  --skip-spec-review      Skip automated spec review loop
  --max-review-iter int   Max spec review iterations (default: 3)
  --dry-run               Show plan without executing

Examples:
  choo feature start streaming-events
  choo feature start streaming-events --skip-spec-review
  choo feature start streaming-events --dry-run
```

**Workflow Steps:**
1. Read PRD from `docs/prds/<prd-id>.md`
2. Create feature branch `feature/<prd-id>` from main
3. Update PRD frontmatter: `feature_status: generating_specs`
4. Invoke spec-generator agent with PRD
5. Loop: spec-reviewer → validate output → feedback → update specs (max iterations)
   - On iteration exhaustion or malformed output: transition to `review_blocked`, notify, exit
6. Update PRD frontmatter: `feature_status: validating_specs`
7. Invoke spec-validator agent
8. Update PRD frontmatter: `feature_status: generating_tasks`
9. Invoke task-generator agent
10. Commit specs and tasks to feature branch
11. Update PRD frontmatter: `feature_status: specs_committed`

#### `choo feature status`

```
choo feature status [prd-id]

Show status of feature workflows.

Arguments:
  prd-id    Specific PRD to check (optional, shows all if omitted)

Flags:
  --json    Output as JSON

Examples:
  choo feature status                    # Show all in-progress features
  choo feature status streaming-events   # Show specific feature
```

**Output Format:**
```
═══════════════════════════════════════════════════════════════
Feature Workflows
═══════════════════════════════════════════════════════════════

 [streaming-events] specs_committed
   Branch: feature/streaming-events
   Started: 2026-01-20 14:30:00
   Review iterations: 2/3
   Specs: 4 units, 18 tasks
   Ready for: choo run --feature streaming-events

 [multi-repo] review_blocked
   Branch: feature/multi-repo
   Started: 2026-01-20 10:15:00
   Review iterations: 3/3 (exhausted)
   Last feedback: "Missing error handling in merger spec"
   Action: Manual intervention required
   Resume with: choo feature resume multi-repo --skip-review

───────────────────────────────────────────────────────────────
 Features: 2 | Ready: 1 | Blocked: 1 | In Progress: 0
═══════════════════════════════════════════════════════════════
```

#### `choo feature resume`

```
choo feature resume <prd-id> [flags]

Resume a feature workflow from a blocked state.

Arguments:
  prd-id    The PRD ID to resume

Flags:
  --skip-review           Skip remaining review iterations and proceed to validation
  --from-validation       Resume from spec validation (after manual spec edits)
  --from-tasks            Resume from task generation (after manual spec edits)

Examples:
  choo feature resume streaming-events
  choo feature resume streaming-events --skip-review
  choo feature resume streaming-events --from-validation
```

**Use Cases:**
- Resume after `review_blocked` (iteration exhaustion or malformed output)
- Resume after manual spec edits
- Resume after user intervention on any blocked state

## Implementation Notes

### PRD Frontmatter Management

PRD files use YAML frontmatter to track workflow state:

```yaml
---
title: Streaming Events
feature_status: generating_specs
branch: feature/streaming-events
started_at: 2026-01-20T14:30:00Z
review_iterations: 1
max_review_iter: 3
---

# Streaming Events PRD
...
```

Updates must be atomic to prevent corruption:
1. Read entire file
2. Parse frontmatter and body separately
3. Modify frontmatter fields
4. Write complete file with updated frontmatter

### Git Branch Operations

```go
// Branch creation sequence
func (w *Workflow) createFeatureBranch(prdID string) error {
    branch := fmt.Sprintf("feature/%s", prdID)

    // Ensure clean working tree
    if !w.git.IsClean() {
        return fmt.Errorf("working tree has uncommitted changes")
    }

    // Fetch latest main
    if err := w.git.Fetch("origin", "main"); err != nil {
        return fmt.Errorf("fetch main: %w", err)
    }

    // Create and checkout branch from main
    if err := w.git.CreateBranch(branch, "origin/main"); err != nil {
        return fmt.Errorf("create branch: %w", err)
    }

    return nil
}
```

### Review Loop Implementation

```go
func (w *Workflow) runReviewLoop(ctx context.Context, prdID string, maxIter int) error {
    for i := 0; i < maxIter; i++ {
        // Update iteration count
        w.prdStore.UpdateField(prdID, "review_iterations", i+1)

        // Run reviewer agent
        result, err := w.agents.RunSpecReviewer(ctx, prdID)
        if err != nil {
            return fmt.Errorf("spec reviewer: %w", err)
        }

        // Check for approval
        if result.Approved {
            return nil
        }

        // Check for malformed output
        if result.Malformed {
            w.prdStore.UpdateStatus(prdID, StatusReviewBlocked)
            w.prdStore.UpdateField(prdID, "last_feedback", "Malformed reviewer output")
            return ErrReviewBlocked
        }

        // Store feedback and continue
        w.prdStore.UpdateField(prdID, "last_feedback", result.Feedback)

        // Apply feedback to specs
        if err := w.agents.RunSpecUpdater(ctx, prdID, result.Feedback); err != nil {
            return fmt.Errorf("apply feedback: %w", err)
        }
    }

    // Exhausted iterations
    w.prdStore.UpdateStatus(prdID, StatusReviewBlocked)
    return ErrIterationExhausted
}
```

### Status Display Rendering

```go
func (a *App) renderFeatureStatus(features []feature.FeatureState) {
    fmt.Println("═══════════════════════════════════════════════════════════════")
    fmt.Println("Feature Workflows")
    fmt.Println("═══════════════════════════════════════════════════════════════")
    fmt.Println()

    var ready, blocked, inProgress int

    for _, f := range features {
        fmt.Printf(" [%s] %s\n", f.PRDID, f.Status)
        fmt.Printf("   Branch: %s\n", f.Branch)
        fmt.Printf("   Started: %s\n", f.StartedAt.Format("2006-01-02 15:04:05"))
        fmt.Printf("   Review iterations: %d/%d", f.ReviewIterations, f.MaxReviewIter)

        if f.ReviewIterations >= f.MaxReviewIter {
            fmt.Print(" (exhausted)")
        }
        fmt.Println()

        switch f.Status {
        case feature.StatusSpecsCommitted:
            ready++
            fmt.Printf("   Specs: %d units, %d tasks\n", f.SpecCount, f.TaskCount)
            fmt.Printf("   Ready for: choo run --feature %s\n", f.PRDID)
        case feature.StatusReviewBlocked:
            blocked++
            if f.LastFeedback != "" {
                fmt.Printf("   Last feedback: %q\n", f.LastFeedback)
            }
            fmt.Println("   Action: Manual intervention required")
            fmt.Printf("   Resume with: choo feature resume %s --skip-review\n", f.PRDID)
        default:
            inProgress++
        }
        fmt.Println()
    }

    fmt.Println("───────────────────────────────────────────────────────────────")
    fmt.Printf(" Features: %d | Ready: %d | Blocked: %d | In Progress: %d\n",
        len(features), ready, blocked, inProgress)
    fmt.Println("═══════════════════════════════════════════════════════════════")
}
```

### Dry Run Mode

```go
func (a *App) runDryRun(opts FeatureStartOptions) error {
    fmt.Println("Dry run - showing planned actions:")
    fmt.Println()
    fmt.Printf("1. Read PRD from %s/%s.md\n", opts.PRDDir, opts.PRDID)
    fmt.Printf("2. Create branch feature/%s from main\n", opts.PRDID)
    fmt.Println("3. Generate specs using spec-generator agent")

    if !opts.SkipSpecReview {
        fmt.Printf("4. Review specs (max %d iterations)\n", opts.MaxReviewIter)
        fmt.Println("5. Validate specs using spec-validator agent")
    } else {
        fmt.Println("4. Skip spec review (--skip-spec-review)")
        fmt.Println("5. Validate specs using spec-validator agent")
    }

    fmt.Println("6. Generate tasks using task-generator agent")
    fmt.Printf("7. Commit specs and tasks to feature/%s\n", opts.PRDID)
    fmt.Println()
    fmt.Println("Run without --dry-run to execute.")

    return nil
}
```

## Testing Strategy

### Unit Tests

```go
// internal/feature/workflow_test.go

func TestWorkflow_Start(t *testing.T) {
    tests := []struct {
        name        string
        prdID       string
        opts        StartOptions
        setupMocks  func(*mockGit, *mockAgents, *mockPRDStore)
        wantStatus  FeatureStatus
        wantErr     bool
    }{
        {
            name:  "successful workflow completion",
            prdID: "test-feature",
            opts:  StartOptions{MaxReviewIter: 3},
            setupMocks: func(git *mockGit, agents *mockAgents, prd *mockPRDStore) {
                git.On("IsClean").Return(true)
                git.On("CreateBranch", "feature/test-feature", "origin/main").Return(nil)
                agents.On("RunSpecGenerator", mock.Anything, "test-feature").Return(nil)
                agents.On("RunSpecReviewer", mock.Anything, "test-feature").Return(
                    &ReviewResult{Approved: true}, nil)
                agents.On("RunSpecValidator", mock.Anything, "test-feature").Return(nil)
                agents.On("RunTaskGenerator", mock.Anything, "test-feature").Return(nil)
                git.On("CommitAll", mock.Anything).Return(nil)
            },
            wantStatus: StatusSpecsCommitted,
        },
        {
            name:  "review blocked after max iterations",
            prdID: "blocked-feature",
            opts:  StartOptions{MaxReviewIter: 2},
            setupMocks: func(git *mockGit, agents *mockAgents, prd *mockPRDStore) {
                git.On("IsClean").Return(true)
                git.On("CreateBranch", mock.Anything, mock.Anything).Return(nil)
                agents.On("RunSpecGenerator", mock.Anything, mock.Anything).Return(nil)
                agents.On("RunSpecReviewer", mock.Anything, mock.Anything).Return(
                    &ReviewResult{Approved: false, Feedback: "needs work"}, nil).Times(2)
                agents.On("RunSpecUpdater", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
            },
            wantStatus: StatusReviewBlocked,
            wantErr:    true,
        },
        {
            name:  "dirty working tree fails",
            prdID: "dirty-tree",
            opts:  StartOptions{},
            setupMocks: func(git *mockGit, agents *mockAgents, prd *mockPRDStore) {
                git.On("IsClean").Return(false)
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            git := &mockGit{}
            agents := &mockAgents{}
            prd := &mockPRDStore{}
            tt.setupMocks(git, agents, prd)

            w := NewWorkflow(git, agents, prd, "specs/tasks")
            result, err := w.Start(context.Background(), tt.prdID, tt.opts)

            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.wantStatus, result.FinalStatus)
            }
        })
    }
}
```

### Integration Tests

```go
// internal/cli/feature_start_test.go

func TestFeatureStartCmd_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Set up temp directory with test PRD
    tmpDir := t.TempDir()
    prdDir := filepath.Join(tmpDir, "docs/prds")
    require.NoError(t, os.MkdirAll(prdDir, 0755))

    prdContent := `---
title: Test Feature
---

# Test Feature PRD

This is a test PRD.
`
    require.NoError(t, os.WriteFile(
        filepath.Join(prdDir, "test-feature.md"),
        []byte(prdContent),
        0644,
    ))

    // Initialize git repo
    cmd := exec.Command("git", "init")
    cmd.Dir = tmpDir
    require.NoError(t, cmd.Run())

    // Run command
    app := NewTestApp(tmpDir)
    err := app.RunFeatureStart(context.Background(), FeatureStartOptions{
        PRDID:         "test-feature",
        PRDDir:        prdDir,
        SpecsDir:      filepath.Join(tmpDir, "specs/tasks"),
        MaxReviewIter: 1,
        DryRun:        true,
    })

    require.NoError(t, err)
}
```

### CLI Tests

```go
// internal/cli/feature_test.go

func TestFeatureCommands(t *testing.T) {
    tests := []struct {
        name     string
        args     []string
        wantErr  bool
        contains string
    }{
        {
            name:     "start requires prd-id",
            args:     []string{"feature", "start"},
            wantErr:  true,
            contains: "requires exactly 1 arg",
        },
        {
            name:     "start with dry-run",
            args:     []string{"feature", "start", "test", "--dry-run"},
            contains: "Dry run",
        },
        {
            name:     "status no args shows all",
            args:     []string{"feature", "status"},
            contains: "Feature Workflows",
        },
        {
            name:     "resume requires prd-id",
            args:     []string{"feature", "resume"},
            wantErr:  true,
            contains: "requires exactly 1 arg",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := NewRootCmd()
            buf := new(bytes.Buffer)
            cmd.SetOut(buf)
            cmd.SetErr(buf)
            cmd.SetArgs(tt.args)

            err := cmd.Execute()

            if tt.wantErr {
                require.Error(t, err)
            }
            if tt.contains != "" {
                assert.Contains(t, buf.String(), tt.contains)
            }
        })
    }
}
```

## Design Decisions

### PRD Frontmatter as State Store

**Decision**: Store workflow state in PRD frontmatter rather than a separate database.

**Rationale**:
- State lives with the PRD, making it self-contained and portable
- Git tracks state changes alongside code changes
- No external database dependency
- Easy to inspect and manually edit if needed

**Trade-offs**:
- Concurrent access requires care (addressed by atomic updates)
- Querying all features requires scanning PRD directory

### Branch-per-Feature Model

**Decision**: Create `feature/<prd-id>` branch for each feature workflow.

**Rationale**:
- Isolates generated specs/tasks from main branch until ready
- Enables parallel feature development
- Facilitates code review via PR workflow
- Matches existing git-flow patterns

### Review Loop with Hard Limit

**Decision**: Enforce maximum review iterations with transition to blocked state.

**Rationale**:
- Prevents infinite loops from non-converging review cycles
- Provides clear intervention point for human review
- Malformed agent output triggers immediate block rather than retry
- Resume capability allows continuing after manual fixes

### Dry Run Mode

**Decision**: Implement comprehensive dry-run that shows all planned actions.

**Rationale**:
- Enables verification before costly agent invocations
- Helps users understand workflow steps
- Useful for CI/CD pipeline testing

## Future Enhancements

1. **Parallel Agent Execution**: Run independent agents concurrently where possible
2. **Webhook Notifications**: Notify external systems on status changes
3. **Custom Review Criteria**: Allow per-PRD review configuration
4. **Workflow Templates**: Support different workflow patterns (e.g., skip validation)
5. **Progress Persistence**: Resume from exact step after crash, not just phase
6. **Feature Dependencies**: Support features that depend on other features

## References

- PRD: `docs/prds/prd-workflow-system.md` sections 4.2, 4.3, 4.4, 10
- Existing CLI pattern: `internal/cli/root.go`
- Cobra documentation: https://cobra.dev/
- YAML frontmatter parsing: `gopkg.in/yaml.v3`
