package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaError represents a validation failure
type SchemaError struct {
	Field   string
	Message string
}

func (e SchemaError) Error() string {
	return fmt.Sprintf("schema validation failed: %s - %s", e.Field, e.Message)
}

// ValidVerdicts defines acceptable verdict values
var ValidVerdicts = []string{"pass", "needs_revision"}

// RequiredScoreCriteria defines the criteria that must have scores
var RequiredScoreCriteria = []string{"completeness", "consistency", "testability", "architecture"}

// ParseAndValidate parses JSON output and validates against required schema
func ParseAndValidate(output string) (*ReviewResult, error) {
	// Extract JSON from output
	jsonStr := extractJSON(output)
	if jsonStr == "" {
		return nil, SchemaError{Field: "json", Message: "no JSON object found in output"}
	}

	// Parse JSON
	var result ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, SchemaError{Field: "json", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	// Store raw output for debugging
	result.RawOutput = output

	// Validate verdict
	if !isValidVerdict(result.Verdict) {
		return nil, SchemaError{Field: "verdict", Message: fmt.Sprintf("must be one of %v, got: %s", ValidVerdicts, result.Verdict)}
	}

	// Validate score object exists
	if result.Score == nil {
		return nil, SchemaError{Field: "score", Message: "score object is required"}
	}

	// Validate all required criteria are present
	for _, criterion := range RequiredScoreCriteria {
		score, exists := result.Score[criterion]
		if !exists {
			return nil, SchemaError{Field: "score." + criterion, Message: "required criterion missing"}
		}

		// Validate score range
		if score < 0 || score > 100 {
			return nil, SchemaError{Field: "score." + criterion, Message: fmt.Sprintf("score must be between 0 and 100, got: %d", score)}
		}
	}

	// Validate feedback when needs_revision
	if result.Verdict == "needs_revision" {
		if len(result.Feedback) == 0 {
			return nil, SchemaError{Field: "feedback", Message: "at least one feedback item required when verdict is needs_revision"}
		}

		// Validate feedback structure
		for i, fb := range result.Feedback {
			if fb.Section == "" {
				return nil, SchemaError{Field: fmt.Sprintf("feedback[%d].section", i), Message: "section cannot be empty"}
			}
			if fb.Issue == "" {
				return nil, SchemaError{Field: fmt.Sprintf("feedback[%d].issue", i), Message: "issue cannot be empty"}
			}
			if fb.Suggestion == "" {
				return nil, SchemaError{Field: fmt.Sprintf("feedback[%d].suggestion", i), Message: "suggestion cannot be empty"}
			}
		}
	}

	return &result, nil
}

// extractJSON finds and extracts JSON object from surrounding text
func extractJSON(output string) string {
	// Find first { and last }
	firstBrace := strings.Index(output, "{")
	lastBrace := strings.LastIndex(output, "}")

	if firstBrace == -1 || lastBrace == -1 || firstBrace >= lastBrace {
		return ""
	}

	return output[firstBrace : lastBrace+1]
}

// isValidVerdict checks if verdict is in ValidVerdicts
func isValidVerdict(verdict string) bool {
	for _, valid := range ValidVerdicts {
		if verdict == valid {
			return true
		}
	}
	return false
}
