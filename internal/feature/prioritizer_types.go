package feature

import "fmt"

// PriorityResult holds the analysis result from Claude
type PriorityResult struct {
	Recommendations []Recommendation `json:"recommendations"`
	DependencyGraph string           `json:"dependency_graph"`
	Analysis        string           `json:"analysis,omitempty"`
}

// Recommendation represents a single PRD recommendation
type Recommendation struct {
	PRDID      string   `json:"prd_id"`
	Title      string   `json:"title"`
	Priority   int      `json:"priority"` // 1 = highest
	Reasoning  string   `json:"reasoning"`
	DependsOn  []string `json:"depends_on"`
	EnablesFor []string `json:"enables_for"` // PRDs that depend on this
}

// PrioritizeOptions controls the prioritization behavior
type PrioritizeOptions struct {
	TopN       int  // Return top N recommendations (default: 3)
	ShowReason bool // Include detailed reasoning in output
}

// DefaultPrioritizeOptions returns options with sensible defaults
func DefaultPrioritizeOptions() PrioritizeOptions {
	return PrioritizeOptions{
		TopN:       3,
		ShowReason: false,
	}
}

// Validate checks that a PriorityResult is well-formed
func (r *PriorityResult) Validate() error {
	// Must have at least one recommendation
	if len(r.Recommendations) == 0 {
		return fmt.Errorf("priority result must have at least one recommendation")
	}

	// Each recommendation must have valid PRDID and Priority
	for i, rec := range r.Recommendations {
		if rec.PRDID == "" {
			return fmt.Errorf("recommendation at index %d has empty PRDID", i)
		}
		if rec.Priority <= 0 {
			return fmt.Errorf("recommendation at index %d has invalid priority %d (must be > 0)", i, rec.Priority)
		}
	}

	return nil
}

// Truncate limits recommendations to the specified count
func (r *PriorityResult) Truncate(n int) {
	// Limit Recommendations slice to first n entries
	if n < len(r.Recommendations) {
		r.Recommendations = r.Recommendations[:n]
	}
}
