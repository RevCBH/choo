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

	"github.com/anthropics/choo/internal/discovery"
	"github.com/anthropics/choo/internal/events"
	"github.com/anthropics/choo/internal/git"
	"github.com/anthropics/choo/internal/github"
	"gopkg.in/yaml.v3"
)

// Worker executes a single unit in an isolated worktree
type Worker struct {
	unit         *discovery.Unit
	config       WorkerConfig
	events       *events.Bus
	git          *git.WorktreeManager
	github       *github.PRClient
	claude       *ClaudeClient
	worktreePath string
	branch       string
	currentTask  *discovery.Task
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
	GitHub *github.PRClient
	Claude *ClaudeClient
}

// ClaudeClient is a placeholder interface for the Claude client
// This will be replaced when the claude package is implemented
type ClaudeClient interface{}

// NewWorker creates a worker for executing a unit
func NewWorker(unit *discovery.Unit, cfg WorkerConfig, deps WorkerDeps) *Worker {
	return &Worker{
		unit:   unit,
		config: cfg,
		events: deps.Events,
		git:    deps.Git,
		github: deps.GitHub,
		claude: deps.Claude,
	}
}

// Run executes the unit through all phases: setup, task loop, baseline, PR
func (w *Worker) Run(ctx context.Context) error {
	// Use defer for cleanup to ensure worktree removal even on error
	defer func() {
		if w.worktreePath != "" {
			_ = w.cleanup(ctx)
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

	// Emit UnitStarted event
	if w.events != nil {
		evt := events.NewEvent(events.UnitStarted, w.unit.ID)
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

// setupWorktree creates the isolated worktree for this worker
func (w *Worker) setupWorktree(ctx context.Context) error {
	// Generate branch name
	w.branch = w.generateBranchName()

	// Create worktree via git.WorktreeManager
	worktree, err := w.git.CreateWorktree(ctx, w.unit.ID, w.config.TargetBranch)
	if err != nil {
		return err
	}

	// Store worktree path and update branch to our generated one
	w.worktreePath = worktree.Path

	// Checkout our custom branch in the worktree
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", w.branch)
	cmd.Dir = w.worktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", w.branch, err)
	}

	return nil
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
		addCmd := exec.CommandContext(ctx, "git", "add", "-A")
		addCmd.Dir = w.worktreePath
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("git add failed during baseline fix: %w", err)
		}

		commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", "fix: baseline checks", "--no-verify")
		commitCmd.Dir = w.worktreePath
		// Ignore error if nothing to commit
		_ = commitCmd.Run()

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
	pushCmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", w.branch)
	pushCmd.Dir = w.worktreePath
	if err := pushCmd.Run(); err != nil {
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
