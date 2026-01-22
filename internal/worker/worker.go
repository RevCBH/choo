package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/escalate"
	"github.com/RevCBH/choo/internal/events"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/github"
	"github.com/RevCBH/choo/internal/provider"
	"gopkg.in/yaml.v3"
)

// Worker executes a single unit in an isolated worktree
type Worker struct {
	unit         *discovery.Unit
	config       WorkerConfig
	events       *events.Bus
	git          *git.WorktreeManager

	// Phase 1: GitOps added alongside gitRunner
	// Phase 3: gitRunner removed, only gitOps remains
	gitOps    git.GitOps // Safe git operations interface
	gitRunner git.Runner // Deprecated: raw runner for unmigrated code

	github       *github.PRClient
	provider     provider.Provider
	escalator    escalate.Escalator
	mergeMu      *sync.Mutex // Shared mutex for serializing merge operations

	// Keep raw path for provider invocation (providers need filesystem path)
	worktreePath string
	branch       string
	currentTask  *discovery.Task

	reviewer     provider.Reviewer        // For code review (may be nil if disabled)
	reviewConfig *config.CodeReviewConfig // Review configuration
	//nolint:unused // WIP: used by merge flow when PR creation is wired up.
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
	SuppressOutput      bool            // When true, don't tee Claude output to stdout (TUI mode)
	ClaudeCommand       string          // Claude CLI command for non-task operations (conflict resolution, etc.)
	AuditLogger         git.AuditLogger // Optional: log all git operations
}

// BaselineCheck represents a single baseline validation command
type BaselineCheck struct {
	Name    string
	Command string
	Pattern string
}

// WorkerDeps bundles worker dependencies for injection
type WorkerDeps struct {
	Events *events.Bus
	Git    *git.WorktreeManager

	// Phase 1: Both GitOps and GitRunner supported
	// Phase 3: Only GitOps required
	GitOps    git.GitOps // Preferred: safe git interface
	GitRunner git.Runner // Deprecated: raw runner

	GitHub       *github.PRClient
	Provider     provider.Provider
	Escalator    escalate.Escalator
	MergeMu      *sync.Mutex              // Shared mutex for serializing merge operations
	Reviewer     provider.Reviewer        // Optional: for code review
	ReviewConfig *config.CodeReviewConfig // Optional: review settings
}

// ClaudeClient is deprecated - use Provider instead
// Kept for backward compatibility during migration
// Deprecated: Use Provider field in WorkerDeps instead
type ClaudeClient any

// NewWorker creates a worker for executing a unit.
// Uses convenience constructors for appropriate safety defaults.
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) (*Worker, error) {
	// Use provided GitOps or create from WorktreeBase
	gitOps := deps.GitOps
	if gitOps == nil && cfg.WorktreeBase != "" && unit != nil {
		worktreePath := filepath.Join(cfg.WorktreeBase, unit.ID)

		// Use convenience constructor with safety options
		// NewWorktreeGitOps sets AllowDestructive=true because:
		// - Worktrees are isolated from main repository
		// - cleanupWorktree() needs Reset, Clean, CheckoutFiles
		// - Worktrees are disposable and meant to be reset
		var err error
		gitOps, err = git.NewWorktreeGitOps(worktreePath, cfg.WorktreeBase)
		if err != nil {
			// During Phase 1-2, path may not exist yet (created later)
			// Only fail on validation errors, not path-not-found
			if !errors.Is(err, git.ErrPathNotFound) && !errors.Is(err, git.ErrNotGitRepo) {
				return nil, fmt.Errorf("invalid worktree path %q: %w", worktreePath, err)
			}
			// Path doesn't exist yet - that's OK, gitOps will be nil
			gitOps = nil
		}
	}

	// Fall back to raw runner for backward compatibility
	gitRunner := deps.GitRunner
	if gitRunner == nil {
		gitRunner = git.DefaultRunner()
	}

	return &Worker{
		unit:         unit,
		config:       cfg,
		events:       deps.Events,
		git:          deps.Git,
		gitOps:       gitOps,
		gitRunner:    gitRunner,
		github:       deps.GitHub,
		provider:     deps.Provider,
		escalator:    deps.Escalator,
		mergeMu:      deps.MergeMu,
		reviewer:     deps.Reviewer,
		reviewConfig: deps.ReviewConfig,
	}, nil
}

// InitGitOps initializes GitOps after worktree is created.
// Call this after setupWorktree() to enable safe git operations.
func (w *Worker) InitGitOps() error {
	if w.gitOps != nil {
		return nil // Already initialized
	}

	if w.worktreePath == "" {
		return fmt.Errorf("worktree path not set")
	}

	var err error
	w.gitOps, err = git.NewWorktreeGitOps(w.worktreePath, w.config.WorktreeBase)
	if err != nil {
		return fmt.Errorf("initializing gitops: %w", err)
	}

	return nil
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

		// Invoke Provider to fix (using same method as task execution)
		if err := w.invokeProvider(ctx, prompt); err != nil {
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
	// 1. Determine if we're working with a remote or local-only target branch
	// Check if the remote tracking branch exists locally
	targetRef, useRemote := w.getTargetRef(ctx)

	// 2. Run code review (advisory - doesn't block merge)
	w.runCodeReview(ctx)

	// 3-6 are serialized via mergeMu to prevent concurrent merge conflicts
	// Acquire merge lock - only one worker can merge at a time
	if w.mergeMu != nil {
		w.mergeMu.Lock()
		defer w.mergeMu.Unlock()
	}

	// 3. Fetch latest feature branch if working with remote
	if useRemote {
		if _, err := w.runner().Exec(ctx, w.worktreePath, "fetch", "origin", w.config.TargetBranch); err != nil {
			return fmt.Errorf("failed to fetch target branch: %w", err)
		}
	}

	// 4. Rebase unit branch onto feature branch with conflict resolution
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
	if err := w.mergeWithCleanup(ctx); err != nil {
		return err
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

// mergeWithCleanup performs the merge to RepoRoot with conflict resolution and cleanup
func (w *Worker) mergeWithCleanup(ctx context.Context) error {
	// Try fast-forward merge first
	_, err := w.runner().Exec(ctx, w.config.RepoRoot, "merge", w.branch, "--ff-only")
	if err == nil {
		return nil // Fast-forward succeeded
	}

	// Try regular merge
	_, err = w.runner().Exec(ctx, w.config.RepoRoot, "merge", w.branch, "-m", fmt.Sprintf("Merge unit %s", w.unit.ID))
	if err == nil {
		return nil // Merge succeeded
	}

	// Check if merge failed due to conflicts
	conflictedFiles, conflictErr := git.GetConflictedFiles(ctx, w.config.RepoRoot)
	if conflictErr != nil || len(conflictedFiles) == 0 {
		// Not a conflict error, or couldn't determine - abort and return original error
		_, _ = w.runner().Exec(ctx, w.config.RepoRoot, "merge", "--abort")
		return fmt.Errorf("failed to merge unit branch into target: %w", err)
	}

	// Emit conflict event
	if w.events != nil {
		evt := events.NewEvent(events.PRConflict, w.unit.ID).
			WithPayload(map[string]any{
				"files": conflictedFiles,
				"stage": "merge_to_target",
			})
		w.events.Emit(evt)
	}

	fmt.Fprintf(os.Stderr, "Merge conflicts in RepoRoot (%d files), invoking Claude to resolve...\n", len(conflictedFiles))

	// Build merge conflict resolution prompt (different from rebase - we're in RepoRoot now)
	prompt := BuildMergeConflictPrompt(w.branch, w.config.TargetBranch, conflictedFiles)

	// Retry loop for conflict resolution
	retryResult := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
		// Invoke Claude in RepoRoot to resolve conflicts
		if err := w.invokeClaudeInDir(ctx, w.config.RepoRoot, prompt); err != nil {
			return err
		}

		// Verify merge completed (no longer in merge state)
		inMerge, err := git.IsMergeInProgress(ctx, w.config.RepoRoot)
		if err != nil {
			return err
		}
		if inMerge {
			stillConflicted, _ := git.GetConflictedFiles(ctx, w.config.RepoRoot)
			if len(stillConflicted) > 0 {
				return fmt.Errorf("claude did not resolve all merge conflicts: %v", stillConflicted)
			}
			return fmt.Errorf("claude did not complete merge (need to commit)")
		}
		return nil
	})

	if !retryResult.Success {
		// Clean up - abort the merge
		_, _ = w.runner().Exec(ctx, w.config.RepoRoot, "merge", "--abort")

		// Escalate to user
		if w.escalator != nil {
			_ = w.escalator.Escalate(ctx, escalate.Escalation{
				Severity: escalate.SeverityBlocking,
				Unit:     w.unit.ID,
				Title:    "Failed to resolve merge conflicts in target branch",
				Message: fmt.Sprintf(
					"Claude could not resolve merge conflicts after %d attempts",
					retryResult.Attempts,
				),
				Context: map[string]string{
					"files":  strings.Join(conflictedFiles, ", "),
					"branch": w.branch,
					"target": w.config.TargetBranch,
					"error":  retryResult.LastErr.Error(),
				},
			})
		}
		return fmt.Errorf("failed to resolve merge conflicts: %w", retryResult.LastErr)
	}

	fmt.Fprintf(os.Stderr, "Successfully resolved merge conflicts in target branch\n")
	return nil
}

// getTargetRef determines the appropriate git ref to use for rebasing/diffing.
// Returns the ref string and whether we're using remote tracking branch.
// For local-only branches (no remote tracking), returns the local branch name.
func (w *Worker) getTargetRef(ctx context.Context) (ref string, useRemote bool) {
	remoteRef := fmt.Sprintf("origin/%s", w.config.TargetBranch)

	// Check if remote tracking branch exists locally (we've fetched it before)
	_, err := w.runner().Exec(ctx, w.worktreePath, "rev-parse", "--verify", remoteRef)
	if err == nil {
		// Remote tracking branch exists, use it
		return remoteRef, true
	}

	// Remote tracking branch doesn't exist, use local branch directly
	return w.config.TargetBranch, false
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
