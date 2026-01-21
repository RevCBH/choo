package feature

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Prioritizer analyzes PRDs and recommends implementation order
type Prioritizer struct {
	prdDir   string
	specsDir string
}

// AgentInvoker abstracts the Claude agent invocation for testing
type AgentInvoker interface {
	Invoke(ctx context.Context, prompt string) (string, error)
}

// NewPrioritizer creates a new prioritizer for the given directories
func NewPrioritizer(prdDir, specsDir string) *Prioritizer {
	return &Prioritizer{
		prdDir:   prdDir,
		specsDir: specsDir,
	}
}

// Prioritize analyzes PRDs and returns ranked recommendations
func (p *Prioritizer) Prioritize(ctx context.Context, invoker AgentInvoker, opts PrioritizeOptions) (*PriorityResult, error) {
	// Load PRDs from directory
	prds, err := LoadPRDs(p.prdDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load PRDs: %w", err)
	}

	if len(prds) == 0 {
		return nil, fmt.Errorf("no PRDs found in %s", p.prdDir)
	}

	// Load existing spec files for context
	specs, err := p.loadExistingSpecs()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing specs: %w", err)
	}

	// Build the prompt
	prompt := p.buildPrompt(prds, specs, opts)

	// Invoke the agent
	response, err := invoker.Invoke(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent invocation failed: %w", err)
	}

	// Parse the response
	result, err := ParsePriorityResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w", err)
	}

	// Truncate to TopN if specified
	if opts.TopN > 0 {
		result.Truncate(opts.TopN)
	}

	return result, nil
}

// buildPrompt constructs the Claude prompt with PRD content and context
func (p *Prioritizer) buildPrompt(prds []*PRDForPrioritization, specs []string, opts PrioritizeOptions) string {
	var sb strings.Builder

	// System context and role
	sb.WriteString("You are a technical product manager analyzing PRDs to recommend implementation order.\n\n")
	sb.WriteString("Your goal is to analyze dependencies, technical foundations, and feature complexity to suggest optimal implementation order.\n\n")

	// Analysis criteria
	sb.WriteString("Analysis Criteria:\n")
	sb.WriteString("1. Foundation features - features that others depend on\n")
	sb.WriteString("2. Refactoring enablers - changes that simplify future work\n")
	sb.WriteString("3. Technical debt fixes - improvements to code quality\n")
	sb.WriteString("4. Independent features - can be parallelized\n\n")

	// PRD list
	sb.WriteString("PRDs to Analyze:\n\n")
	for _, prd := range prds {
		sb.WriteString(fmt.Sprintf("ID: %s\n", prd.ID))
		sb.WriteString(fmt.Sprintf("Title: %s\n", prd.Title))
		if len(prd.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("Depends On: %s\n", strings.Join(prd.DependsOn, ", ")))
		}
		sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", prd.Content))
		sb.WriteString("---\n\n")
	}

	// Existing specs context
	if len(specs) > 0 {
		sb.WriteString("Existing Completed Specs:\n")
		for _, spec := range specs {
			sb.WriteString(fmt.Sprintf("- %s\n", spec))
		}
		sb.WriteString("\n")
	}

	// Output format instructions
	sb.WriteString("Output Format:\n")
	sb.WriteString("Return a JSON object with the following structure:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"recommendations\": [\n")
	sb.WriteString("    {\n")
	sb.WriteString("      \"prd_id\": \"string\",\n")
	sb.WriteString("      \"title\": \"string\",\n")
	sb.WriteString("      \"priority\": 1,\n")
	sb.WriteString("      \"reasoning\": \"string\",\n")
	sb.WriteString("      \"depends_on\": [\"string\"],\n")
	sb.WriteString("      \"enables_for\": [\"string\"]\n")
	sb.WriteString("    }\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"dependency_graph\": \"string describing the overall dependency structure\",\n")
	if opts.ShowReason {
		sb.WriteString("  \"analysis\": \"string with detailed analysis of the prioritization\"\n")
	}
	sb.WriteString("}\n\n")

	// Explain flag
	if opts.ShowReason {
		sb.WriteString("Include detailed analysis in the 'analysis' field explaining your prioritization decisions.\n")
	} else {
		sb.WriteString("Keep reasoning concise in each recommendation.\n")
	}

	return sb.String()
}

// loadExistingSpecs finds completed spec files for context
func (p *Prioritizer) loadExistingSpecs() ([]string, error) {
	// Check if specs directory exists
	if _, err := os.Stat(p.specsDir); os.IsNotExist(err) {
		// No specs directory is fine - return empty list
		return []string{}, nil
	}

	// Look for completed specs
	completedDir := filepath.Join(p.specsDir, "completed")
	if _, err := os.Stat(completedDir); os.IsNotExist(err) {
		// No completed directory yet - return empty list
		return []string{}, nil
	}

	// Find all markdown files in completed directory
	pattern := filepath.Join(completedDir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for spec files: %w", err)
	}

	// Extract basenames
	var specs []string
	for _, path := range matches {
		basename := filepath.Base(path)
		specs = append(specs, basename)
	}

	return specs, nil
}
