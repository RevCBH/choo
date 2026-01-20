package review

import "github.com/RevCBH/choo/internal/events"

// Review event types
const (
	SpecReviewStarted   events.EventType = "spec.review.started"
	SpecReviewFeedback  events.EventType = "spec.review.feedback"
	SpecReviewPassed    events.EventType = "spec.review.passed"
	SpecReviewBlocked   events.EventType = "spec.review.blocked"
	SpecReviewIteration events.EventType = "spec.review.iteration"
	SpecReviewMalformed events.EventType = "spec.review.malformed"
)

// ReviewStartedPayload contains data for review started events
type ReviewStartedPayload struct {
	Feature   string `json:"feature"`
	PRDPath   string `json:"prd_path"`
	SpecsPath string `json:"specs_path"`
}

// ReviewFeedbackPayload contains data for feedback events
type ReviewFeedbackPayload struct {
	Feature   string           `json:"feature"`
	Iteration int              `json:"iteration"`
	Feedback  []ReviewFeedback `json:"feedback"`
	Scores    map[string]int   `json:"scores"`
}

// ReviewBlockedPayload contains data for blocked events
type ReviewBlockedPayload struct {
	Feature      string             `json:"feature"`
	Reason       string             `json:"reason"`
	Iterations   []IterationHistory `json:"iterations"`
	Recovery     []string           `json:"recovery_actions"`
	CurrentSpecs string             `json:"current_specs_path"`
}

// ReviewMalformedPayload contains data for malformed output events
type ReviewMalformedPayload struct {
	Feature     string `json:"feature"`
	RawOutput   string `json:"raw_output"`
	ParseError  string `json:"parse_error"`
	RetryNumber int    `json:"retry_number"`
}
