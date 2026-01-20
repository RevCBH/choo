package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/RevCBH/choo/internal/feature"
	"github.com/spf13/cobra"
)

// FeatureStatusOptions holds flags for the feature status command
type FeatureStatusOptions struct {
	PRDID  string // optional, shows all if empty
	JSON   bool
	PRDDir string
}

// FeatureStatusOutput represents JSON output format
type FeatureStatusOutput struct {
	Features []FeatureStatusItem `json:"features"`
	Summary  StatusSummary       `json:"summary"`
}

// FeatureStatusItem represents a single feature's status
type FeatureStatusItem struct {
	PRDID            string `json:"prd_id"`
	Status           string `json:"status"`
	Branch           string `json:"branch"`
	StartedAt        string `json:"started_at,omitempty"`
	ReviewIterations int    `json:"review_iterations"`
	MaxReviewIter    int    `json:"max_review_iter"`
	LastFeedback     string `json:"last_feedback,omitempty"`
	SpecCount        int    `json:"spec_count,omitempty"`
	TaskCount        int    `json:"task_count,omitempty"`
	NextAction       string `json:"next_action,omitempty"`
}

// StatusSummary provides aggregate counts
type StatusSummary struct {
	Total      int `json:"total"`
	Ready      int `json:"ready"`
	Blocked    int `json:"blocked"`
	InProgress int `json:"in_progress"`
}

// NewFeatureStatusCmd creates the feature status command
func NewFeatureStatusCmd(app *App) *cobra.Command {
	opts := FeatureStatusOptions{
		PRDDir: "docs/prds",
	}

	cmd := &cobra.Command{
		Use:   "status [prd-id]",
		Short: "Show status of feature workflows",
		Long: `Show status of feature workflows.

This command displays the current status of all in-progress feature workflows
or a specific feature if a PRD ID is provided.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.PRDID = args[0]
			}
			return app.ShowFeatureStatus(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&opts.PRDDir, "prd-dir", opts.PRDDir, "PRDs directory")

	return cmd
}

// ShowFeatureStatus displays feature workflow status
func (a *App) ShowFeatureStatus(ctx context.Context, opts FeatureStatusOptions) error {
	// Load features from PRD directory
	features, err := a.loadFeatures(opts.PRDDir)
	if err != nil {
		return fmt.Errorf("failed to load features: %w", err)
	}

	// If specific PRDID provided, filter to that one
	if opts.PRDID != "" {
		filtered := []feature.FeatureState{}
		for _, f := range features {
			if f.PRDID == opts.PRDID {
				filtered = append(filtered, f)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("feature not found: %s", opts.PRDID)
		}
		features = filtered
	}

	// If JSON flag, output as JSON
	if opts.JSON {
		return a.outputJSON(features)
	}

	// Otherwise, render formatted output
	a.renderFeatureStatus(features)
	return nil
}

// loadFeatures scans PRD directory for feature state
func (a *App) loadFeatures(prdDir string) ([]feature.FeatureState, error) {
	// List all .md files in prdDir
	entries, err := os.ReadDir(prdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []feature.FeatureState{}, nil
		}
		return nil, fmt.Errorf("failed to read PRD directory: %w", err)
	}

	store := feature.NewPRDStore(prdDir)
	var features []feature.FeatureState

	// Parse each for feature state
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		prdID := strings.TrimSuffix(entry.Name(), ".md")
		meta, _, err := store.Load(prdID)
		if err != nil {
			continue // Skip files that can't be parsed
		}

		// Filter to only those with feature_status set
		if meta.FeatureStatus == "" {
			continue
		}

		state := feature.FeatureState{
			PRDID:            prdID,
			Status:           meta.FeatureStatus,
			Branch:           meta.Branch,
			ReviewIterations: meta.ReviewIterations,
			MaxReviewIter:    meta.MaxReviewIter,
			LastFeedback:     meta.LastFeedback,
			SpecCount:        meta.SpecCount,
			TaskCount:        meta.TaskCount,
		}
		if meta.StartedAt != nil {
			state.StartedAt = *meta.StartedAt
		}

		features = append(features, state)
	}

	// Return sorted list (newest first)
	sort.Slice(features, func(i, j int) bool {
		return features[i].StartedAt.After(features[j].StartedAt)
	})

	return features, nil
}

// outputJSON outputs features as JSON
func (a *App) outputJSON(features []feature.FeatureState) error {
	items := make([]FeatureStatusItem, len(features))
	summary := StatusSummary{Total: len(features)}

	for i, f := range features {
		item := FeatureStatusItem{
			PRDID:            f.PRDID,
			Status:           string(f.Status),
			Branch:           f.Branch,
			ReviewIterations: f.ReviewIterations,
			MaxReviewIter:    f.MaxReviewIter,
			LastFeedback:     f.LastFeedback,
			SpecCount:        f.SpecCount,
			TaskCount:        f.TaskCount,
			NextAction:       determineNextAction(f),
		}
		if !f.StartedAt.IsZero() {
			item.StartedAt = f.StartedAt.Format("2006-01-02 15:04:05")
		}
		items[i] = item

		// Update summary counts
		if f.Status == feature.StatusSpecsCommitted {
			summary.Ready++
		} else if f.Status.IsBlocked() {
			summary.Blocked++
		} else {
			summary.InProgress++
		}
	}

	output := FeatureStatusOutput{
		Features: items,
		Summary:  summary,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// renderFeatureStatus outputs formatted status display
func (a *App) renderFeatureStatus(features []feature.FeatureState) {
	if len(features) == 0 {
		fmt.Println("No in-progress features found.")
		return
	}

	// Print header with box drawing characters
	fmt.Println("===============================================================")
	fmt.Println("Feature Workflows")
	fmt.Println("===============================================================")
	fmt.Println()

	summary := StatusSummary{Total: len(features)}

	// For each feature
	for _, f := range features {
		fmt.Printf(" [%s] %s\n", f.PRDID, f.Status)
		if f.Branch != "" {
			fmt.Printf("   Branch: %s\n", f.Branch)
		}
		if !f.StartedAt.IsZero() {
			fmt.Printf("   Started: %s\n", f.StartedAt.Format("2006-01-02 15:04:05"))
		}
		if f.MaxReviewIter > 0 {
			fmt.Printf("   Review iterations: %d/%d", f.ReviewIterations, f.MaxReviewIter)
			if f.ReviewIterations >= f.MaxReviewIter {
				fmt.Print(" (exhausted)")
			}
			fmt.Println()
		}

		// Based on status
		switch f.Status {
		case feature.StatusSpecsCommitted:
			// Print spec/task counts, ready command
			if f.SpecCount > 0 || f.TaskCount > 0 {
				fmt.Printf("   Specs: %d units, %d tasks\n", f.SpecCount, f.TaskCount)
			}
			fmt.Printf("   Ready for: choo run --feature %s\n", f.PRDID)
			summary.Ready++

		case feature.StatusReviewBlocked:
			// Print feedback, action needed, resume command
			if f.LastFeedback != "" {
				fmt.Printf("   Last feedback: \"%s\"\n", f.LastFeedback)
			}
			fmt.Println("   Action: Manual intervention required")
			fmt.Printf("   Resume with: choo feature resume %s --skip-review\n", f.PRDID)
			summary.Blocked++

		default:
			// Print current step
			nextAction := determineNextAction(f)
			if nextAction != "" {
				fmt.Printf("   Next: %s\n", nextAction)
			}
			summary.InProgress++
		}

		fmt.Println()
	}

	// Print summary footer
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf(" Features: %d | Ready: %d | Blocked: %d | In Progress: %d\n",
		summary.Total, summary.Ready, summary.Blocked, summary.InProgress)
	fmt.Println("===============================================================")
}

// determineNextAction returns the actionable next step for a status
func determineNextAction(state feature.FeatureState) string {
	switch state.Status {
	case feature.StatusNotStarted:
		return "Start feature workflow"
	case feature.StatusGeneratingSpecs:
		return "Generating specification files"
	case feature.StatusReviewingSpecs:
		return "Reviewing generated specs"
	case feature.StatusReviewBlocked:
		return fmt.Sprintf("Manual review required - run 'choo feature resume %s --skip-review'", state.PRDID)
	case feature.StatusValidatingSpecs:
		return "Validating specifications"
	case feature.StatusGeneratingTasks:
		return "Generating tasks from specs"
	case feature.StatusSpecsCommitted:
		return fmt.Sprintf("Ready to run - execute 'choo run --feature %s'", state.PRDID)
	default:
		return ""
	}
}
