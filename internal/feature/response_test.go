package feature

import (
	"testing"
)

func TestParsePriorityResponse_RawJSON(t *testing.T) {
	rawJSON := `{
		"recommendations": [
			{
				"prd_id": "feature-001",
				"title": "User Authentication",
				"priority": 1,
				"reasoning": "Foundation feature",
				"depends_on": [],
				"enables_for": ["feature-002"]
			}
		],
		"dependency_graph": "feature-001 -> feature-002",
		"analysis": "Authentication is critical"
	}`

	result, err := ParsePriorityResponse(rawJSON)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.Recommendations))
	}

	rec := result.Recommendations[0]
	if rec.PRDID != "feature-001" {
		t.Errorf("expected PRDID 'feature-001', got '%s'", rec.PRDID)
	}
	if rec.Title != "User Authentication" {
		t.Errorf("expected title 'User Authentication', got '%s'", rec.Title)
	}
	if rec.Priority != 1 {
		t.Errorf("expected priority 1, got %d", rec.Priority)
	}
	if result.DependencyGraph != "feature-001 -> feature-002" {
		t.Errorf("expected dependency_graph, got '%s'", result.DependencyGraph)
	}
	if result.Analysis != "Authentication is critical" {
		t.Errorf("expected analysis, got '%s'", result.Analysis)
	}
}

func TestParsePriorityResponse_MarkdownWrapped(t *testing.T) {
	response := "Here's the analysis:\n\n```json\n" +
		`{
		"recommendations": [
			{
				"prd_id": "feature-002",
				"title": "Dashboard",
				"priority": 2,
				"reasoning": "Secondary feature",
				"depends_on": ["feature-001"],
				"enables_for": []
			}
		],
		"dependency_graph": "feature-001 -> feature-002"
	}` + "\n```\n\nThis is the result."

	result, err := ParsePriorityResponse(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.Recommendations))
	}

	rec := result.Recommendations[0]
	if rec.PRDID != "feature-002" {
		t.Errorf("expected PRDID 'feature-002', got '%s'", rec.PRDID)
	}
	if rec.Title != "Dashboard" {
		t.Errorf("expected title 'Dashboard', got '%s'", rec.Title)
	}
	if rec.Priority != 2 {
		t.Errorf("expected priority 2, got %d", rec.Priority)
	}
}

func TestParsePriorityResponse_PlainCodeBlock(t *testing.T) {
	response := "Analysis result:\n\n```\n" +
		`{
		"recommendations": [
			{
				"prd_id": "feature-003",
				"title": "Reports",
				"priority": 3,
				"reasoning": "Nice to have",
				"depends_on": ["feature-002"],
				"enables_for": []
			}
		],
		"dependency_graph": "feature-002 -> feature-003"
	}` + "\n```"

	result, err := ParsePriorityResponse(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.Recommendations))
	}

	rec := result.Recommendations[0]
	if rec.PRDID != "feature-003" {
		t.Errorf("expected PRDID 'feature-003', got '%s'", rec.PRDID)
	}
}

func TestParsePriorityResponse_WithPreamble(t *testing.T) {
	response := `I've analyzed the PRDs. Here are my recommendations:

	Some text before the JSON...

	{
		"recommendations": [
			{
				"prd_id": "feature-004",
				"title": "Settings",
				"priority": 1,
				"reasoning": "Core functionality",
				"depends_on": [],
				"enables_for": []
			}
		],
		"dependency_graph": "standalone"
	}

	Additional notes here.`

	result, err := ParsePriorityResponse(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.Recommendations))
	}

	rec := result.Recommendations[0]
	if rec.PRDID != "feature-004" {
		t.Errorf("expected PRDID 'feature-004', got '%s'", rec.PRDID)
	}
}

func TestParsePriorityResponse_EmptyRecommendations(t *testing.T) {
	response := `{
		"recommendations": [],
		"dependency_graph": "none"
	}`

	_, err := ParsePriorityResponse(response)
	if err == nil {
		t.Fatal("expected error for empty recommendations, got none")
	}

	expectedMsg := "no recommendations in response"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestParsePriorityResponse_MissingPRDID(t *testing.T) {
	response := `{
		"recommendations": [
			{
				"prd_id": "",
				"title": "Invalid Feature",
				"priority": 1,
				"reasoning": "Missing ID",
				"depends_on": [],
				"enables_for": []
			}
		],
		"dependency_graph": "none"
	}`

	_, err := ParsePriorityResponse(response)
	if err == nil {
		t.Fatal("expected error for missing PRDID, got none")
	}

	expectedMsg := "recommendation 0 missing prd_id"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestParsePriorityResponse_InvalidPriority(t *testing.T) {
	response := `{
		"recommendations": [
			{
				"prd_id": "feature-005",
				"title": "Invalid Priority",
				"priority": 0,
				"reasoning": "Priority is zero",
				"depends_on": [],
				"enables_for": []
			}
		],
		"dependency_graph": "none"
	}`

	_, err := ParsePriorityResponse(response)
	if err == nil {
		t.Fatal("expected error for invalid priority, got none")
	}

	expectedMsg := "recommendation 0 (feature-005) has invalid priority: 0"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	response := "This is just plain text with no JSON content"

	_, err := extractJSON(response)
	if err == nil {
		t.Fatal("expected error when no JSON found, got none")
	}

	expectedMsg := "no JSON content found in response"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestExtractJSON_RawJSON(t *testing.T) {
	jsonStr := `{"test": "value"}`

	result, err := extractJSON(jsonStr)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result != jsonStr {
		t.Errorf("expected '%s', got '%s'", jsonStr, result)
	}
}

func TestExtractJSON_MarkdownJSON(t *testing.T) {
	response := "Some text\n```json\n{\"test\": \"value\"}\n```\nMore text"

	result, err := extractJSON(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := `{"test": "value"}`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestExtractJSON_PlainCodeBlock(t *testing.T) {
	response := "Some text\n```\n{\"test\": \"value\"}\n```\nMore text"

	result, err := extractJSON(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := `{"test": "value"}`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	response := "Text before {\"outer\": {\"inner\": \"value\"}} text after"

	result, err := extractJSON(response)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := `{"outer": {"inner": "value"}}`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestValidateRecommendations_Empty(t *testing.T) {
	var recs []Recommendation

	err := validateRecommendations(recs)
	if err == nil {
		t.Fatal("expected error for empty recommendations, got none")
	}

	expectedMsg := "no recommendations in response"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestValidateRecommendations_MissingPRDID(t *testing.T) {
	recs := []Recommendation{
		{PRDID: "valid-1", Priority: 1},
		{PRDID: "", Priority: 2},
	}

	err := validateRecommendations(recs)
	if err == nil {
		t.Fatal("expected error for missing PRDID, got none")
	}

	expectedMsg := "recommendation 1 missing prd_id"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestValidateRecommendations_InvalidPriority(t *testing.T) {
	recs := []Recommendation{
		{PRDID: "valid-1", Priority: 1},
		{PRDID: "invalid-2", Priority: -1},
	}

	err := validateRecommendations(recs)
	if err == nil {
		t.Fatal("expected error for invalid priority, got none")
	}

	expectedMsg := "recommendation 1 (invalid-2) has invalid priority: -1"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestValidateRecommendations_Valid(t *testing.T) {
	recs := []Recommendation{
		{PRDID: "valid-1", Priority: 1},
		{PRDID: "valid-2", Priority: 2},
	}

	err := validateRecommendations(recs)
	if err != nil {
		t.Fatalf("expected no error for valid recommendations, got: %v", err)
	}
}
