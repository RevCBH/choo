package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	mergeMu      *sync.Mutex // Shared mutex for serializing merge operations
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
	MergeMu   *sync.Mutex // Shared mutex for serializing merge operations
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
		mergeMu:   deps.MergeMu,
	}
}

// Run executes the unit through all phases: setup, task loop, baseline, PR
func (w *Worker) Run(ctx context.Context) error {
	// NOTE: Worktrees are intentionally NOT cleaned up on success.
	// They preserve task status information that `choo status` reads.
	// Worktrees should only be removed after PR is merged (via `choo cleanup`).
	defer func() {
		if w.worktreePath != "" {
			fmt.Fprintf(os.Stderr, "Worktree preserved at: %s\n", w.worktreePath)
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

	// Phase 3: Merge to feature branch (replaces PR workflow)
	if !w.config.NoPR { // NoPR now means "no merge" for testing
		if err := w.mergeToFeatureBranch(ctx); err != nil {
			if w.events != nil {
				evt := events.NewEvent(events.UnitFailed, w.unit.ID).WithError(err)
				w.events.Emit(evt)
			}
			return fmt.Errorf("merge to feature branch failed: %w", err)
		}
	}

	// Phase 4: Update unit status and emit UnitCompleted event
	if err := w.updateUnitStatus(discovery.UnitStatusComplete); err != nil {
		return fmt.Errorf("failed to update unit status: %w", err)
	}

	if w.events != nil {
		evt := events.NewEvent(events.UnitCompleted, w.unit.ID)
		w.events.Emit(evt)
	}

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

// mergeToFeatureBranch performs local merge to the feature branch (replaces PR workflow)
// This ensures dependent units have access to their predecessors' code
func (w *Worker) mergeToFeatureBranch(ctx context.Context) error {
	// 1. Review placeholder (log what would be reviewed)
	w.logReviewPlaceholder(ctx)

	// 2-5 are serialized via mergeMu to prevent concurrent merge conflicts
	// Acquire merge lock - only one worker can merge at a time
	if w.mergeMu != nil {
		w.mergeMu.Lock()
		defer w.mergeMu.Unlock()
	}

	// 2. Fetch latest feature branch to ensure we're rebasing onto latest
	if _, err := w.runner().Exec(ctx, w.worktreePath, "fetch", "origin", w.config.TargetBranch); err != nil {
		return fmt.Errorf("failed to fetch target branch: %w", err)
	}

	// 3. Rebase unit branch onto feature branch with conflict resolution
	targetRef := fmt.Sprintf("origin/%s", w.config.TargetBranch)
	hasConflicts, err := git.Rebase(ctx, w.worktreePath, targetRef)
	if err != nil {
		return fmt.Errorf("rebase failed: %w", err)
	}

	if hasConflicts {
		// Attempt to resolve conflicts using Claude
		if err := w.resolveConflictsWithClaude(ctx); err != nil {
			// Clean up - abort the rebase
			_ = git.AbortRebase(ctx, w.worktreePath)
			return fmt.Errorf("failed to resolve merge conflicts: %w", err)
		}
	}

	// 4. Merge unit branch into target branch in the RepoRoot
	// This updates the local target branch so dependent units see the changes.
	//
	// Context assumption: RepoRoot must have TargetBranch checked out. This is satisfied when:
	// - Feature mode with worktree: RepoRoot is the feature worktree (feature branch checked out)
	// - Feature mode from repo root: RepoRoot is the main repo (must have feature branch checked out)
	// - Non-feature mode: RepoRoot is the main repo (main branch checked out)
	//
	// The orchestrator is responsible for ensuring RepoRoot is in the correct state before
	// starting workers. No implicit checkout is performed here.
	if _, err := w.runner().Exec(ctx, w.config.RepoRoot, "merge", w.branch, "--ff-only"); err != nil {
		// Fall back to regular merge if fast-forward fails
		if _, err := w.runner().Exec(ctx, w.config.RepoRoot, "merge", w.branch, "-m", fmt.Sprintf("Merge unit %s", w.unit.ID)); err != nil {
			return fmt.Errorf("failed to merge unit branch into target: %w", err)
		}
	}

	// 5. Emit UnitMerged event
	if w.events != nil {
		evt := events.NewEvent(events.UnitMerged, w.unit.ID).WithPayload(map[string]any{
			"branch":        w.branch,
			"target_branch": w.config.TargetBranch,
		})
		w.events.Emit(evt)
	}

	fmt.Fprintf(os.Stderr, "Successfully merged unit %s to %s (local)\n", w.unit.ID, w.config.TargetBranch)
	return nil
}

// resolveConflictsWithClaude uses Claude to resolve merge conflicts during rebase
func (w *Worker) resolveConflictsWithClaude(ctx context.Context) error {
	// Get list of conflicted files
	conflictedFiles, err := git.GetConflictedFiles(ctx, w.worktreePath)
	if err != nil {
		return fmt.Errorf("failed to get conflicted files: %w", err)
	}

	// Emit conflict event for observability
	if w.events != nil {
		evt := events.NewEvent(events.PRConflict, w.unit.ID).
			WithPayload(map[string]any{
				"files": conflictedFiles,
			})
		w.events.Emit(evt)
	}

	fmt.Fprintf(os.Stderr, "Merge conflicts detected in %d files, invoking Claude to resolve...\n", len(conflictedFiles))

	// Build conflict resolution prompt
	prompt := BuildConflictPrompt(w.config.TargetBranch, conflictedFiles)

	// Retry loop for conflict resolution
	retryResult := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		if err := w.invokeClaude(ctx, prompt); err != nil {
			return err
		}

		// Verify rebase completed (no longer in rebase state)
		inRebase, err := git.IsRebaseInProgress(ctx, w.worktreePath)
		if err != nil {
			return err
		}
		if inRebase {
			// Claude didn't complete the rebase - check if there are still conflicts
			stillConflicted, _ := git.GetConflictedFiles(ctx, w.worktreePath)
			if len(stillConflicted) > 0 {
				return fmt.Errorf("claude did not resolve all conflicts: %v", stillConflicted)
			}
			return fmt.Errorf("claude did not complete rebase (git rebase --continue)")
		}
		return nil
	})

	if !retryResult.Success {
		// Escalate to user if escalator is available
		if w.escalator != nil {
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to resolve merge conflicts",
				Message: fmt.Sprintf(
					"Claude could not resolve conflicts after %d attempts",
					retryResult.Attempts,
				),
				Context: map[string]string{
					"files":  strings.Join(conflictedFiles, ", "),
					"target": w.config.TargetBranch,
					"error":  retryResult.LastErr.Error(),
				},
			})
		}
		return retryResult.LastErr
	}

	fmt.Fprintf(os.Stderr, "Successfully resolved merge conflicts\n")
	return nil
}

// logReviewPlaceholder logs the diff that would be reviewed
// Future: integrate codex review here
func (w *Worker) logReviewPlaceholder(ctx context.Context) {
	// Get the diff between target branch and current HEAD
	diff, err := w.runner().Exec(ctx, w.worktreePath, "diff", fmt.Sprintf("origin/%s...HEAD", w.config.TargetBranch), "--stat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Review placeholder - could not get diff: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "Review placeholder - changes to review for unit %s:\n%s\n", w.unit.ID, diff)
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
