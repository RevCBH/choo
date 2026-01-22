package spec

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/provider"
	"github.com/RevCBH/choo/internal/skills"
)

// Workflow orchestrates the spec generation workflow phases.
type Workflow struct {
	state         *SpecState
	provider      provider.Provider
	promptBuilder *skills.PromptBuilder
	globalConfig  *config.GlobalConfig
	repoRoot      string
	stdout        io.Writer
	stderr        io.Writer
	dryRun        bool
}

// WorkflowOptions configures a new workflow.
type WorkflowOptions struct {
	// PRDPath is the path to the PRD file (can be relative or absolute).
	PRDPath string

	// SpecsDir is the output directory for specs (relative to repo root).
	// If empty, uses default from global config.
	SpecsDir string

	// RepoRoot is the target repository root (if empty, uses current directory).
	RepoRoot string

	// DryRun if true, prints prompts without executing.
	DryRun bool

	// Stream enables JSON streaming output for visibility into Claude's work.
	Stream bool

	// Verbose enables full text output in streaming mode.
	Verbose bool

	// Debug enables assistant text output in streaming mode.
	Debug bool

	// Stdout for provider output.
	Stdout io.Writer

	// Stderr for provider errors.
	Stderr io.Writer
}

// NewWorkflow creates a new spec workflow.
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	// Load global config
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}

	// Determine repo root
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	// Determine specs directory
	specsDir := opts.SpecsDir
	if specsDir == "" {
		specsDir = globalCfg.DefaultSpecsDir
	}

	// Resolve PRD path
	prdPath := opts.PRDPath
	if !filepath.IsAbs(prdPath) {
		prdPath = filepath.Join(repoRoot, prdPath)
	}

	// Verify PRD exists
	if _, err := os.Stat(prdPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PRD file not found: %s", prdPath)
	}

	// Create prompt builder
	promptBuilder, err := skills.NewPromptBuilderWithDir(globalCfg.ExpandSkillsDir())
	if err != nil {
		return nil, fmt.Errorf("create prompt builder: %w", err)
	}

	// Create provider
	providerType := globalCfg.Provider.Type
	if providerType == "" {
		providerType = config.ProviderClaude
	}
	providerCmd := globalCfg.GetProviderCommandForType(providerType)

	var prov provider.Provider
	if opts.Stream && providerType == config.ProviderClaude {
		// Use streaming Claude provider
		claude := provider.NewClaudeWithStreaming(providerCmd, opts.Verbose)
		claude.SetShowAssistant(opts.Debug)
		prov = claude
	} else {
		var err error
		prov, err = provider.FromConfig(provider.Config{
			Type:    provider.ProviderType(providerType),
			Command: providerCmd,
		})
		if err != nil {
			return nil, fmt.Errorf("create provider: %w", err)
		}
	}

	// Make PRD path relative for state
	relPRDPath, err := filepath.Rel(repoRoot, prdPath)
	if err != nil {
		relPRDPath = prdPath
	}

	// Create state
	state := NewSpecState(relPRDPath, specsDir, repoRoot)

	// Default stdout/stderr
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	return &Workflow{
		state:         state,
		provider:      prov,
		promptBuilder: promptBuilder,
		globalConfig:  globalCfg,
		repoRoot:      repoRoot,
		stdout:        stdout,
		stderr:        stderr,
		dryRun:        opts.DryRun,
	}, nil
}

// Run executes the full workflow from the current state.
func (w *Workflow) Run(ctx context.Context) error {
	return w.runFromPhase(ctx, w.state.NextPhase(), false)
}

// RunPhase executes a single phase.
func (w *Workflow) RunPhase(ctx context.Context, phase Phase) error {
	return w.runFromPhase(ctx, phase, true)
}

// Resume continues from saved state.
func (w *Workflow) Resume(ctx context.Context) error {
	// Load existing state
	existingState, err := LoadState(w.repoRoot)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if existingState == nil {
		return fmt.Errorf("no saved state found for this repository")
	}

	// Use existing state
	w.state = existingState

	if !w.state.IsResumable() {
		return fmt.Errorf("workflow cannot be resumed from phase %s", w.state.Phase)
	}

	// Continue from next phase
	return w.runFromPhase(ctx, w.state.NextPhase(), false)
}

// runFromPhase runs the workflow starting from the given phase.
func (w *Workflow) runFromPhase(ctx context.Context, startPhase Phase, singlePhase bool) error {
	phase := startPhase

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		switch phase {
		case PhaseSpecGeneration:
			if err := w.runSpecGeneration(ctx); err != nil {
				w.state.SetError(err)
				_ = w.state.Save()
				return err
			}
		case PhaseValidation:
			if err := w.runValidation(ctx); err != nil {
				w.state.SetError(err)
				_ = w.state.Save()
				return err
			}
		case PhaseRalphPrep:
			if err := w.runRalphPrep(ctx); err != nil {
				w.state.SetError(err)
				_ = w.state.Save()
				return err
			}
		case PhaseComplete:
			w.state.SetPhase(PhaseComplete)
			_ = w.state.Save()
			fmt.Fprintf(w.stdout, "\n✓ Workflow complete!\n")
			fmt.Fprintf(w.stdout, "  Specs: %d\n", w.state.SpecCount)
			fmt.Fprintf(w.stdout, "  Tasks: %d\n", w.state.TaskCount)
			return nil
		default:
			return fmt.Errorf("unknown phase: %s", phase)
		}

		// Save state after each phase
		if err := w.state.Save(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}

		if singlePhase {
			return nil
		}

		// Advance to next phase
		phase = w.state.NextPhase()
	}
}

// runSpecGeneration executes the spec generation phase.
func (w *Workflow) runSpecGeneration(ctx context.Context) error {
	w.state.SetPhase(PhaseSpecGeneration)

	fmt.Fprintf(w.stdout, "\n── Phase 1: Spec Generation ──\n")
	fmt.Fprintf(w.stdout, "PRD: %s\n", w.state.PRDPath)
	fmt.Fprintf(w.stdout, "Output: %s/\n\n", w.state.SpecsDir)

	// Read PRD content
	prdPath := filepath.Join(w.repoRoot, w.state.PRDPath)
	prdContent, err := os.ReadFile(prdPath)
	if err != nil {
		return fmt.Errorf("read PRD: %w", err)
	}

	// Get existing specs
	specsPath := filepath.Join(w.repoRoot, w.state.SpecsDir)
	existingSpecs, _ := skills.GetExistingSpecs(specsPath)

	// Build prompt
	prompt := w.promptBuilder.BuildSpecGenerationPrompt(
		string(prdContent),
		w.state.SpecsDir,
		existingSpecs,
	)

	w.configureStreamContext(PhaseSpecGeneration, nil, "specs", true)

	if w.dryRun {
		fmt.Fprintf(w.stdout, "=== DRY RUN: Spec Generation Prompt ===\n")
		fmt.Fprintf(w.stdout, "%s\n", prompt)
		fmt.Fprintf(w.stdout, "=== END DRY RUN ===\n")
		return nil
	}

	// Show skill sources
	sources := w.promptBuilder.SkillSources()
	fmt.Fprintf(w.stdout, "Skills loaded:\n")
	for name, source := range sources {
		fmt.Fprintf(w.stdout, "  %s: %s\n", name, source)
	}
	fmt.Fprintf(w.stdout, "\n")

	// Invoke provider
	fmt.Fprintf(w.stdout, "Running spec generation with %s...\n\n", w.provider.Name())
	if err := w.provider.Invoke(ctx, prompt, w.repoRoot, w.stdout, w.stderr); err != nil {
		return fmt.Errorf("spec generation: %w", err)
	}

	// Count generated specs
	newSpecs, _ := skills.GetExistingSpecs(specsPath)
	w.state.SpecCount = len(newSpecs)

	fmt.Fprintf(w.stdout, "\n✓ Spec generation complete (%d specs)\n", w.state.SpecCount)
	return nil
}

// runValidation executes the spec validation phase.
func (w *Workflow) runValidation(ctx context.Context) error {
	w.state.SetPhase(PhaseValidation)

	fmt.Fprintf(w.stdout, "\n── Phase 2: Validation ──\n")
	fmt.Fprintf(w.stdout, "Specs: %s/\n\n", w.state.SpecsDir)

	// Build prompt
	prompt := w.promptBuilder.BuildValidationPrompt(w.state.SpecsDir, w.state.PRDPath)

	var initialItems []string
	specsPath := filepath.Join(w.repoRoot, w.state.SpecsDir)
	if existingSpecs, err := skills.GetExistingSpecs(specsPath); err == nil {
		initialItems = make([]string, 0, len(existingSpecs))
		for _, specPath := range existingSpecs {
			initialItems = append(initialItems, filepath.ToSlash(filepath.Join(w.state.SpecsDir, specPath)))
		}
	}
	w.configureStreamContext(PhaseValidation, initialItems, "validated", true)

	if w.dryRun {
		fmt.Fprintf(w.stdout, "=== DRY RUN: Validation Prompt ===\n")
		fmt.Fprintf(w.stdout, "%s\n", prompt)
		fmt.Fprintf(w.stdout, "=== END DRY RUN ===\n")
		w.state.ValidationOK = true
		return nil
	}

	// Invoke provider
	fmt.Fprintf(w.stdout, "Running validation with %s...\n\n", w.provider.Name())
	if err := w.provider.Invoke(ctx, prompt, w.repoRoot, w.stdout, w.stderr); err != nil {
		return fmt.Errorf("validation: %w", err)
	}

	w.state.ValidationOK = true
	fmt.Fprintf(w.stdout, "\n✓ Validation complete\n")
	return nil
}

// runRalphPrep executes the ralph-prep phase.
func (w *Workflow) runRalphPrep(ctx context.Context) error {
	w.state.SetPhase(PhaseRalphPrep)

	fmt.Fprintf(w.stdout, "\n── Phase 3: Ralph Prep ──\n")
	fmt.Fprintf(w.stdout, "Specs: %s/\n", w.state.SpecsDir)
	fmt.Fprintf(w.stdout, "Tasks: %s/tasks/\n\n", w.state.SpecsDir)

	// Build prompt
	prompt := w.promptBuilder.BuildRalphPrepPrompt(w.state.SpecsDir)

	// Get existing specs to track progress (we process each spec to generate tasks)
	var initialItems []string
	specsPath := filepath.Join(w.repoRoot, w.state.SpecsDir)
	if existingSpecs, err := skills.GetExistingSpecs(specsPath); err == nil {
		initialItems = make([]string, 0, len(existingSpecs))
		for _, specPath := range existingSpecs {
			initialItems = append(initialItems, filepath.ToSlash(filepath.Join(w.state.SpecsDir, specPath)))
		}
	}
	w.configureStreamContext(PhaseRalphPrep, initialItems, "specs", true)

	if w.dryRun {
		fmt.Fprintf(w.stdout, "=== DRY RUN: Ralph Prep Prompt ===\n")
		fmt.Fprintf(w.stdout, "%s\n", prompt)
		fmt.Fprintf(w.stdout, "=== END DRY RUN ===\n")
		return nil
	}

	// Invoke provider
	fmt.Fprintf(w.stdout, "Running ralph-prep with %s...\n\n", w.provider.Name())
	if err := w.provider.Invoke(ctx, prompt, w.repoRoot, w.stdout, w.stderr); err != nil {
		return fmt.Errorf("ralph-prep: %w", err)
	}

	// Count generated tasks
	tasksDir := filepath.Join(w.repoRoot, w.state.SpecsDir, "tasks")
	w.state.TaskCount = countTasks(tasksDir)

	fmt.Fprintf(w.stdout, "\n✓ Ralph prep complete (%d tasks)\n", w.state.TaskCount)
	return nil
}

// countTasks counts the number of task spec files in a directory.
func countTasks(tasksDir string) int {
	count := 0
	_ = filepath.Walk(tasksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			// Don't count IMPLEMENTATION_PLAN.md as a task
			if info.Name() != "IMPLEMENTATION_PLAN.md" && info.Name() != "README.md" {
				count++
			}
		}
		return nil
	})
	return count
}

// State returns the current workflow state.
func (w *Workflow) State() *SpecState {
	return w.state
}

// ParsePhase parses a phase name from string.
func ParsePhase(s string) (Phase, error) {
	switch strings.ToLower(s) {
	case "spec", "spec_generation", "spec-generation":
		return PhaseSpecGeneration, nil
	case "validate", "validation":
		return PhaseValidation, nil
	case "ralph-prep", "ralph_prep", "ralphprep", "tasks":
		return PhaseRalphPrep, nil
	case "all":
		return PhaseNotStarted, nil
	default:
		return "", fmt.Errorf("unknown phase: %s (valid: spec, validate, ralph-prep, all)", s)
	}
}

func (w *Workflow) configureStreamContext(phase Phase, initialItems []string, counterLabel string, enableProgress bool) {
	setter, ok := w.provider.(provider.StreamContextSetter)
	if !ok {
		return
	}
	setter.SetStreamContext(provider.StreamContext{
		PhaseTitle:     phaseTitle(phase),
		CounterLabel:   counterLabel,
		PRDPath:        w.state.PRDPath,
		SpecsDir:       w.state.SpecsDir,
		RepoRoot:       w.repoRoot,
		InitialItems:   initialItems,
		Total:          len(initialItems),
		EnableProgress: enableProgress,
	})
}

func phaseTitle(phase Phase) string {
	switch phase {
	case PhaseSpecGeneration:
		return "Phase 1: Spec Generation"
	case PhaseValidation:
		return "Phase 2: Validation"
	case PhaseRalphPrep:
		return "Phase 3: Ralph Prep"
	default:
		return string(phase)
	}
}
