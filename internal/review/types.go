package review

import "time"

// ReviewConfig configures the review loop behavior
type ReviewConfig struct {
	MaxIterations    int      // Maximum review iterations before blocking (default: 3)
	Criteria         []string // Review criteria to evaluate
	RetryOnMalformed int      // Retry attempts on malformed output (default: 1)
}

// DefaultReviewConfig returns sensible defaults
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		MaxIterations:    3,
		Criteria:         []string{"completeness", "consistency", "testability", "architecture"},
		RetryOnMalformed: 1,
	}
}

// ReviewResult represents the outcome of a single review
type ReviewResult struct {
	Verdict   string            `json:"verdict"`   // "pass" or "needs_revision"
	Score     map[string]int    `json:"score"`     // criteria -> score (0-100)
	Feedback  []ReviewFeedback  `json:"feedback"`  // Required when needs_revision
	RawOutput string            `json:"-"`         // For debugging malformed output
}

// ReviewFeedback represents a single piece of actionable feedback
type ReviewFeedback struct {
	Section    string `json:"section"`    // Spec section with issue
	Issue      string `json:"issue"`      // Description of the problem
	Suggestion string `json:"suggestion"` // How to fix it
}

// IterationHistory tracks review attempts for debugging
type IterationHistory struct {
	Iteration int           `json:"iteration"`
	Result    *ReviewResult `json:"result"`
	Timestamp time.Time     `json:"timestamp"`
}

// ReviewSession tracks the full review loop state
type ReviewSession struct {
	Feature      string             `json:"feature"`
	PRDPath      string             `json:"prd_path"`
	SpecsPath    string             `json:"specs_path"`
	Config       ReviewConfig       `json:"config"`
	Iterations   []IterationHistory `json:"iterations"`
	FinalVerdict string             `json:"final_verdict"` // "pass", "blocked", or ""
	BlockReason  string             `json:"block_reason"`  // Set when blocked
}
