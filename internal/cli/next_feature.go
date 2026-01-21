package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/RevCBH/choo/internal/feature"
)

// NextFeatureOptions holds flags for the next-feature command
type NextFeatureOptions struct {
	PRDDir  string
	Explain bool
	TopN    int
	JSON    bool
}

// NewNextFeatureCmd creates the next-feature command
func NewNextFeatureCmd(app *App) *cobra.Command {
	opts := NextFeatureOptions{
		PRDDir:  "docs/prd",
		Explain: false,
		TopN:    3,
		JSON:    false,
	}

	cmd := &cobra.Command{
		Use:   "next-feature [prd-dir]",
		Short: "Analyze PRDs and recommend next feature to implement",
		Long: `Analyzes Product Requirement Documents (PRDs) to recommend which feature
should be implemented next based on dependencies and technical priorities.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Override PRDDir if positional arg provided
			if len(args) > 0 {
				opts.PRDDir = args[0]
			}
			return app.RunNextFeature(cmd.Context(), opts)
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&opts.Explain, "explain", false, "Show detailed reasoning")
	cmd.Flags().IntVar(&opts.TopN, "top", 3, "Number of recommendations to show")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")

	return cmd
}

// RunNextFeature executes the prioritization and displays results
func (a *App) RunNextFeature(ctx context.Context, opts NextFeatureOptions) error {
	// Validate PRD directory exists
	if _, err := os.Stat(opts.PRDDir); os.IsNotExist(err) {
		return fmt.Errorf("PRD directory not found: %s", opts.PRDDir)
	}

	// Check if directory is empty
	entries, err := os.ReadDir(opts.PRDDir)
	if err != nil {
		return fmt.Errorf("failed to read PRD directory: %w", err)
	}

	// Filter for .md files
	hasPRDs := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			hasPRDs = true
			break
		}
	}

	if !hasPRDs {
		return fmt.Errorf("No PRD files found in %s", opts.PRDDir)
	}

	// Create prioritizer
	// For now, we assume specs are in "specs" directory - this could be configurable later
	prioritizer := feature.NewPrioritizer(opts.PRDDir, "specs")

	// Set up prioritize options
	prioritizeOpts := feature.PrioritizeOptions{
		TopN:       opts.TopN,
		ShowReason: opts.Explain,
	}

	// Get the agent invoker from app configuration
	// The agent invoker must be set up during app initialization
	invoker := a.agentInvoker
	if invoker == nil {
		return fmt.Errorf("agent invoker not configured: choo next-feature requires Claude agent integration.\n" +
			"Run with ANTHROPIC_API_KEY environment variable set, or configure an agent invoker.")
	}

	// Run prioritization
	result, err := prioritizer.Prioritize(ctx, invoker, prioritizeOpts)
	if err != nil {
		return fmt.Errorf("failed to analyze PRDs: %w", err)
	}

	// Format and display output
	if opts.JSON {
		output, err := formatJSONOutput(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON output: %w", err)
		}
		fmt.Println(output)
	} else {
		output := formatStandardOutput(result, opts.Explain)
		fmt.Println(output)
	}

	return nil
}

// formatStandardOutput formats results for terminal display
func formatStandardOutput(result *feature.PriorityResult, explain bool) string {
	var sb strings.Builder

	sb.WriteString("Feature Implementation Recommendations\n")
	sb.WriteString("=====================================\n\n")

	for i, rec := range result.Recommendations {
		// Rank, ID, and Title
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, rec.PRDID, rec.Title))

		// Brief reasoning
		sb.WriteString(fmt.Sprintf("   Reasoning: %s\n", rec.Reasoning))

		// Dependencies
		if len(rec.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("   Depends On: %s\n", strings.Join(rec.DependsOn, ", ")))
		}

		// Enables
		if len(rec.EnablesFor) > 0 {
			sb.WriteString(fmt.Sprintf("   Enables: %s\n", strings.Join(rec.EnablesFor, ", ")))
		}

		// Detailed explanation if requested
		if explain {
			sb.WriteString(fmt.Sprintf("   Priority: %d\n", rec.Priority))
		}

		sb.WriteString("\n")
	}

	// Add dependency graph info
	if result.DependencyGraph != "" {
		sb.WriteString("Dependency Overview\n")
		sb.WriteString("-------------------\n")
		sb.WriteString(result.DependencyGraph)
		sb.WriteString("\n\n")
	}

	// Add detailed analysis if explain mode
	if explain && result.Analysis != "" {
		sb.WriteString("Detailed Analysis\n")
		sb.WriteString("-----------------\n")
		sb.WriteString(result.Analysis)
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatJSONOutput formats results as JSON
func formatJSONOutput(result *feature.PriorityResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
