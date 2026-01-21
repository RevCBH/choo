package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/RevCBH/choo/internal/feature"
)

// FeatureStatusOptions holds flags for the feature status command
type FeatureStatusOptions struct {
	PRDID string // optional, shows all if empty
	JSON  bool
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
	opts := &FeatureStatusOptions{}

	cmd := &cobra.Command{
		Use:   "status [prd-id]",
		Short: "Show status of feature workflows",
		Long: `Show status of feature workflows.

Displays the current state of in-progress feature workflows, including
their status, branch, review iterations, and next actions.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.PRDID = args[0]
			}
			return app.ShowFeatureStatus(*opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")

	return cmd
}

// ShowFeatureStatus displays feature workflow status
func (a *App) ShowFeatureStatus(opts FeatureStatusOptions) error {
	// Determine PRD directory (default to docs/prd)
	prdDir := "docs/prd"

	// Load features from PRD directory
	features, err := a.loadFeatures(prdDir)
	if err != nil {
		return err
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
			return fmt.Errorf("no feature workflow found for PRD ID: %s", opts.PRDID)
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
	// Check if directory exists
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		return []feature.FeatureState{}, nil
	}

	// List all .md files in prdDir
	pattern := filepath.Join(prdDir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for PRD files: %w", err)
	}

	if len(matches) == 0 {
		return []feature.FeatureState{}, nil
	}

	var features []feature.FeatureState
	store := feature.NewPRDStore(prdDir)

	for _, path := range matches {
		basename := filepath.Base(path)
		prdID := strings.TrimSuffix(basename, filepath.Ext(basename))

		// Load PRD metadata
		metadata, _, err := store.Load(prdID)
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		// Filter to only those with feature_status set
		if metadata.FeatureStatus == "" {
			continue
		}

		// Convert to FeatureState
		state := feature.FeatureState{
			PRDID:            prdID,
			Status:           metadata.FeatureStatus,
			Branch:           metadata.Branch,
			ReviewIterations: metadata.ReviewIterations,
			MaxReviewIter:    metadata.MaxReviewIter,
			LastFeedback:     metadata.LastFeedback,
			SpecCount:        metadata.SpecCount,
			TaskCount:        metadata.TaskCount,
		}

		if metadata.StartedAt != nil {
			state.StartedAt = *metadata.StartedAt
		}

		features = append(features, state)
	}

	// Return sorted list (by started_at, newest first)
	sort.Slice(features, func(i, j int) bool {
		return features[i].StartedAt.After(features[j].StartedAt)
	})

	return features, nil
}

// outputJSON outputs features in JSON format
func (a *App) outputJSON(features []feature.FeatureState) error {
	items := make([]FeatureStatusItem, len(features))
	summary := StatusSummary{
		Total: len(features),
	}

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
		switch f.Status {
		case feature.StatusSpecsCommitted:
			summary.Ready++
		case feature.StatusReviewBlocked:
			summary.Blocked++
		case feature.StatusGeneratingSpecs, feature.StatusReviewingSpecs,
			feature.StatusValidatingSpecs, feature.StatusGeneratingTasks:
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
		fmt.Println("No feature workflows in progress.")
		return
	}

	// Print header with box drawing characters
	fmt.Println("===============================================================")
	fmt.Println("Feature Workflows")
	fmt.Println("===============================================================")
	fmt.Println()

	summary := StatusSummary{
		Total: len(features),
	}

	// For each feature, print details
	for _, f := range features {
		fmt.Printf(" [%s] %s\n", f.PRDID, f.Status)
		if f.Branch != "" {
			fmt.Printf("   Branch: %s\n", f.Branch)
		}
		if !f.StartedAt.IsZero() {
			fmt.Printf("   Started: %s\n", f.StartedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("   Review iterations: %d/%d", f.ReviewIterations, f.MaxReviewIter)
		if f.ReviewIterations >= f.MaxReviewIter && f.MaxReviewIter > 0 {
			fmt.Printf(" (exhausted)")
		}
		fmt.Println()

		// Based on status, print specific information
		switch f.Status {
		case feature.StatusSpecsCommitted:
			if f.SpecCount > 0 || f.TaskCount > 0 {
				fmt.Printf("   Specs: %d units, %d tasks\n", f.SpecCount, f.TaskCount)
			}
			fmt.Printf("   Ready for: choo run --feature %s\n", f.PRDID)
			summary.Ready++

		case feature.StatusReviewBlocked:
			if f.LastFeedback != "" {
				fmt.Printf("   Last feedback: \"%s\"\n", f.LastFeedback)
			}
			fmt.Printf("   Action: Manual intervention required\n")
			fmt.Printf("   Resume with: choo feature resume %s --skip-review\n", f.PRDID)
			summary.Blocked++

		case feature.StatusGeneratingSpecs:
			fmt.Printf("   Status: Generating specifications\n")
			summary.InProgress++

		case feature.StatusReviewingSpecs:
			fmt.Printf("   Status: Reviewing specifications\n")
			summary.InProgress++

		case feature.StatusValidatingSpecs:
			fmt.Printf("   Status: Validating specifications\n")
			summary.InProgress++

		case feature.StatusGeneratingTasks:
			fmt.Printf("   Status: Generating tasks\n")
			summary.InProgress++

		default:
			fmt.Printf("   Status: %s\n", f.Status)
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
	case feature.StatusSpecsCommitted:
		return fmt.Sprintf("choo run --feature %s", state.PRDID)
	case feature.StatusReviewBlocked:
		return fmt.Sprintf("choo feature resume %s --skip-review", state.PRDID)
	case feature.StatusGeneratingSpecs:
		return "Wait for spec generation to complete"
	case feature.StatusReviewingSpecs:
		return "Wait for spec review to complete"
	case feature.StatusValidatingSpecs:
		return "Wait for spec validation to complete"
	case feature.StatusGeneratingTasks:
		return "Wait for task generation to complete"
	default:
		return ""
	}
}
