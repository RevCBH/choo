package feature

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParsePriorityResponse parses Claude's response into a PriorityResult
// Handles both raw JSON and markdown-wrapped JSON (```json ... ```)
func ParsePriorityResponse(response string) (*PriorityResult, error) {
	// Extract JSON from the response
	jsonStr, err := extractJSON(response)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JSON: %w", err)
	}

	// Parse the JSON into PriorityResult
	var result PriorityResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate the recommendations
	if err := validateRecommendations(result.Recommendations); err != nil {
		return nil, err
	}

	return &result, nil
}

// extractJSON extracts JSON content from a response string
// Handles: raw JSON, ```json ... ``` wrapper, ``` ... ``` wrapper
func extractJSON(response string) (string, error) {
	response = strings.TrimSpace(response)

	// Case 1: Check if response starts with { (raw JSON)
	if strings.HasPrefix(response, "{") {
		return response, nil
	}

	// Case 2: Look for ```json marker
	jsonMarker := "```json"
	if idx := strings.Index(response, jsonMarker); idx != -1 {
		start := idx + len(jsonMarker)
		// Find the closing ```
		if endIdx := strings.Index(response[start:], "```"); endIdx != -1 {
			jsonContent := response[start : start+endIdx]
			return strings.TrimSpace(jsonContent), nil
		}
	}

	// Case 3: Look for plain ``` marker
	codeMarker := "```"
	if idx := strings.Index(response, codeMarker); idx != -1 {
		start := idx + len(codeMarker)
		// Find the closing ```
		if endIdx := strings.Index(response[start:], codeMarker); endIdx != -1 {
			jsonContent := response[start : start+endIdx]
			return strings.TrimSpace(jsonContent), nil
		}
	}

	// Case 4: Find first { and match closing } by counting depth
	firstBrace := strings.Index(response, "{")
	if firstBrace == -1 {
		return "", fmt.Errorf("no JSON content found in response")
	}

	depth := 0
	for i := firstBrace; i < len(response); i++ {
		switch response[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return response[firstBrace : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("no valid JSON object found in response")
}

// validateRecommendations checks that all recommendations are valid
func validateRecommendations(recs []Recommendation) error {
	if len(recs) == 0 {
		return fmt.Errorf("no recommendations in response")
	}
	for i, rec := range recs {
		if rec.PRDID == "" {
			return fmt.Errorf("recommendation %d missing prd_id", i)
		}
		if rec.Priority <= 0 {
			return fmt.Errorf("recommendation %d (%s) has invalid priority: %d", i, rec.PRDID, rec.Priority)
		}
	}
	return nil
}
