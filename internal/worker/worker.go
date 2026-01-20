package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"gopkg.in/yaml.v3"
)

// Worker executes a single unit in an isolated worktree
type Worker struct {
	unit         *discovery.Unit
	config       WorkerConfig
	events       *events.Bus
	git          *git.WorktreeManager
	gitRunner    git.Runner
	github       *github.PRClient
	claude       *ClaudeClient
	escalator    escalate.Escalator
	worktreePath string
	branch       string
	currentTask  *discovery.Task

	// prNumber is the PR number after creation
	//nolint:unused // WIP: used by forcePushAndMerge when conflict resolution is fully integrated
	prNumber int

	// invokeClaudeWithOutput is the function that invokes Claude and captures output
	// Can be overridden for testing
	//nolint:unused // WIP: used in integration tests for PR creation
	invokeClaudeWithOutput func(ctx context.Context, prompt string) (string, error)
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	RepoRoot            string
	TargetBranch        string
	WorktreeBase        string
	BaselineChecks      []BaselineCheck
	MaxClaudeRetries    int
	MaxBaselineRetries  int
	BackpressureTimeout time.Duration
	BaselineTimeout     time.Duration
	NoPR                bool
	SuppressOutput      bool // When true, don't tee Claude output to stdout (TUI mode)
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
	Name    string
	Command string
	Pattern string
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
	Events    *events.Bus
	Git       *git.WorktreeManager
	GitRunner git.Runner
	GitHub    *github.PRClient
	Claude    *ClaudeClient
	Escalator escalate.Escalator
}

// ClaudeClient is a placeholder interface for the Claude client
// This will be replaced when the claude package is implemented
type ClaudeClient any

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
	gitRunner := deps.GitRunner
	if gitRunner == nil {
		gitRunner = git.DefaultRunner()
	}
	return &Worker{
		unit:      unit,
		config:    cfg,
		events:    deps.Events,
		git:       deps.Git,
		gitRunner: gitRunner,
		github:    deps.GitHub,
		claude:    deps.Claude,
		escalator: deps.Escalator,
	}
}

// Run executes the unit through all phases: setup, task loop, baseline, PR
func (w *Worker) Run(ctx context.Context) error {
	// Track success to decide whether to cleanup worktree
	// On failure, preserve worktree for debugging/resume
	success := false
	defer func() {
		if w.worktreePath != "" && success {
			_ = w.cleanup(ctx)
		} else if w.worktreePath != "" {
			fmt.Fprintf(os.Stderr, "Preserving worktree for debugging: %s\n", w.worktreePath)
		}
	}()

	// Phase 1: Setup
	if err := w.setupWorktree(ctx); err != nil {
		if w.events != nil {
			evt := events.NewEvent(events.UnitFailed, w.unit.ID).WithError(err)
			w.events.Emit(evt)
		}
		return fmt.Errorf("worktree setup failed: %w", err)
	}

	// Update unit frontmatter: orch_status=in_progress
	if err := w.updateUnitStatus(discovery.UnitStatusInProgress); err != nil {
		return fmt.Errorf("failed to update unit status: %w", err)
	}

	// Emit UnitStarted event with task count for TUI
	if w.events != nil {
		// Count already-completed tasks (for resume scenarios)
		completedTasks := 0
		for _, task := range w.unit.Tasks {
			if task.Status == discovery.TaskStatusComplete {
				completedTasks++
			}
		}
		evt := events.NewEvent(events.UnitStarted, w.unit.ID).WithPayload(map[string]any{
			"total_tasks":     len(w.unit.Tasks),
			"completed_tasks": completedTasks,
		})
		w.events.Emit(evt)
	}

	// Phase 2: Task Loop
	if err := w.runTaskLoop(ctx); err != nil {
		if w.events != nil {
			evt := events.NewEvent(events.UnitFailed, w.unit.ID).WithError(err)
			w.events.Emit(evt)
		}
		return fmt.Errorf("task loop failed: %w", err)
	}

	// Phase 2.5: Baseline Checks
	if err := w.runBaselinePhase(ctx); err != nil {
		if w.events != nil {
			evt := events.NewEvent(events.UnitFailed, w.unit.ID).WithError(err)
			w.events.Emit(evt)
		}
		return fmt.Errorf("baseline checks failed: %w", err)
	}

	// Phase 3: PR Creation
	if !w.config.NoPR {
		if err := w.createPR(ctx); err != nil {
			if w.events != nil {
				evt := events.NewEvent(events.UnitFailed, w.unit.ID).WithError(err)
				w.events.Emit(evt)
			}
			return fmt.Errorf("PR creation failed: %w", err)
		}
	}

	// Phase 4: Emit UnitCompleted event
	if w.events != nil {
		evt := events.NewEvent(events.UnitCompleted, w.unit.ID)
		w.events.Emit(evt)
	}

	// Mark success so worktree gets cleaned up
	success = true

	return nil
}

// generateBranchName creates a unique branch name for the unit
func (w *Worker) generateBranchName() string {
	// Hash includes unit ID and timestamp for uniqueness
	hasher := sha256.New()
	hasher.Write([]byte(w.unit.ID))
	hasher.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Format: ralph/<unit-id>-<short-hash>
	return fmt.Sprintf("ralph/%s-%s", w.unit.ID, hash[:6])
}

// setupWorktree creates the isolated worktree for this worker, or reuses an existing one
func (w *Worker) setupWorktree(ctx context.Context) error {
	// Create (or get existing) worktree via git.WorktreeManager
	worktree, err := w.git.CreateWorktree(ctx, w.unit.ID, w.config.TargetBranch)
	if err != nil {
		return err
	}

	// Store worktree path
	w.worktreePath = worktree.Path

	// Check if we're resuming an existing worktree by checking the current branch
	currentBranch, err := w.getCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// If the current branch is a worker branch (ralph/<unit>-<hash>), reuse it
	if strings.HasPrefix(currentBranch, fmt.Sprintf("ralph/%s-", w.unit.ID)) {
		w.branch = currentBranch
		fmt.Fprintf(os.Stderr, "Resuming existing worktree on branch %s\n", w.branch)

		// Refresh task statuses from worktree since they may have progressed
		if err := w.refreshTaskStatuses(); err != nil {
			return fmt.Errorf("failed to refresh task statuses: %w", err)
		}
		return nil
	}

	// Generate a new branch name and checkout
	w.branch = w.generateBranchName()
	if _, err := w.runner().Exec(ctx, w.worktreePath, "checkout", "-b", w.branch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", w.branch, err)
	}

	return nil
}

// refreshTaskStatuses re-reads task statuses from the worktree to handle resumption
func (w *Worker) refreshTaskStatuses() error {
	// Compute unit path relative to worktree
	unitPath := w.unit.Path
	if filepath.IsAbs(unitPath) {
		var err error
		unitPath, err = filepath.Rel(w.config.RepoRoot, unitPath)
		if err != nil {
			return fmt.Errorf("failed to get relative unit path: %w", err)
		}
	}

	// Re-parse each task from the worktree
	for _, task := range w.unit.Tasks {
		taskPath := filepath.Join(w.worktreePath, unitPath, task.FilePath)
		updated, err := discovery.ParseTaskFile(taskPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to refresh status for task #%d: %v\n", task.Number, err)
			continue
		}

		if task.Status != updated.Status {
			fmt.Fprintf(os.Stderr, "Task #%d status refreshed: %s -> %s\n", task.Number, task.Status, updated.Status)
			task.Status = updated.Status
		}
	}

	return nil
}

// getCurrentBranch returns the currently checked out branch in the worktree
func (w *Worker) getCurrentBranch(ctx context.Context) (string, error) {
	output, err := w.runner().Exec(ctx, w.worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// runBaselinePhase executes baseline checks with retry loop
func (w *Worker) runBaselinePhase(ctx context.Context) error {
	// Run baseline checks
	passed, output := RunBaselineChecks(ctx, w.config.BaselineChecks, w.worktreePath, w.config.BaselineTimeout)
	if passed {
		return nil
	}

	// If baseline checks fail, retry with Claude fix
	maxRetries := w.config.MaxBaselineRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Build baseline commands string for the prompt
		var baselineCommands strings.Builder
		for _, check := range w.config.BaselineChecks {
			fmt.Fprintf(&baselineCommands, "- %s: `%s`\n", check.Name, check.Command)
		}

		// Build baseline fix prompt with failure output
		promptContent := BuildBaselineFixPrompt(output, baselineCommands.String())
		prompt := TaskPrompt{Content: promptContent}

		// Invoke Claude to fix (using same method as task execution)
		if err := w.invokeClaudeForTask(ctx, prompt); err != nil {
			// Continue to next retry
			continue
		}

		// Commit fixes with --no-verify
		if _, err := w.runner().Exec(ctx, w.worktreePath, "add", "-A"); err != nil {
			return fmt.Errorf("git add failed during baseline fix: %w", err)
		}

		_, err := w.runner().Exec(ctx, w.worktreePath, "commit", "-m", "fix: baseline checks", "--no-verify")
		// Ignore error if nothing to commit
		_ = err

		// Re-run baseline checks
		passed, output = RunBaselineChecks(ctx, w.config.BaselineChecks, w.worktreePath, w.config.BaselineTimeout)
		if passed {
			return nil
		}
	}

	// Return error if still failing
	return fmt.Errorf("baseline checks failed after %d retries: %s", maxRetries, output)
}

// createPR pushes branch and creates pull request
func (w *Worker) createPR(ctx context.Context) error {
	// Push branch to remote
	if _, err := w.runner().Exec(ctx, w.worktreePath, "push", "-u", "origin", w.branch); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// Create PR via gh CLI (since github.PRClient.CreatePR is delegated to Claude)
	prTitle := fmt.Sprintf("feat: %s", w.unit.ID)
	prBody := fmt.Sprintf("Auto-generated PR for unit %s", w.unit.ID)

	createCmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--base", w.config.TargetBranch)
	createCmd.Dir = w.worktreePath
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	// Update unit frontmatter: orch_status=pr_open
	if err := w.updateUnitStatus(discovery.UnitStatusPROpen); err != nil {
		return fmt.Errorf("failed to update unit status to pr_open: %w", err)
	}

	// Emit PRCreated event
	if w.events != nil {
		evt := events.NewEvent(events.PRCreated, w.unit.ID)
		w.events.Emit(evt)
	}

	return nil
}

// cleanup removes the worktree
func (w *Worker) cleanup(ctx context.Context) error {
	if w.worktreePath == "" {
		return nil
	}

	worktree := &git.Worktree{
		Path:   w.worktreePath,
		Branch: w.branch,
		UnitID: w.unit.ID,
	}

	return w.git.RemoveWorktree(ctx, worktree)
}

// updateUnitStatus updates the unit frontmatter status in the IMPLEMENTATION_PLAN.md
func (w *Worker) updateUnitStatus(status discovery.UnitStatus) error {
	implPlanPath := filepath.Join(w.worktreePath, "specs", "units", w.unit.ID, "IMPLEMENTATION_PLAN.md")

	// Read the file
	content, err := os.ReadFile(implPlanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, skip status update
			return nil
		}
		return fmt.Errorf("failed to read IMPLEMENTATION_PLAN.md: %w", err)
	}

	// Parse frontmatter and body
	frontmatterBytes, body, err := discovery.ParseFrontmatter(content)
	if err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse the frontmatter YAML
	var fm discovery.UnitFrontmatter
	if frontmatterBytes != nil {
		if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
			return fmt.Errorf("failed to unmarshal frontmatter: %w", err)
		}
	}

	// Update the status
	fm.OrchStatus = string(status)

	// Re-serialize the frontmatter
	updatedFM, err := yaml.Marshal(&fm)
	if err != nil {
		return fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	// Reconstruct the file content
	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(updatedFM)
	buf.WriteString("---\n")
	buf.Write(body)

	// Write the file back
	if err := os.WriteFile(implPlanPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write IMPLEMENTATION_PLAN.md: %w", err)
	}

	return nil
}
