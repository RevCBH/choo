package review

import (
	"strings"
	"testing"
)

func TestParseAndValidate_ValidPass(t *testing.T) {
	input := `{
		"verdict": "pass",
		"score": {
			"completeness": 85,
			"consistency": 90,
			"testability": 80,
			"architecture": 88
		},
		"feedback": []
	}`

	result, err := ParseAndValidate(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Verdict != "pass" {
		t.Errorf("expected verdict 'pass', got: %s", result.Verdict)
	}

	if result.Score["completeness"] != 85 {
		t.Errorf("expected completeness score 85, got: %d", result.Score["completeness"])
	}
}

func TestParseAndValidate_ValidNeedsRevision(t *testing.T) {
	input := `{
		"verdict": "needs_revision",
		"score": {
			"completeness": 60,
			"consistency": 70,
			"testability": 65,
			"architecture": 75
		},
		"feedback": [
			{
				"section": "Dependencies",
				"issue": "Missing external API dependencies",
				"suggestion": "Add GitHub API client dependency"
			}
		]
	}`

	result, err := ParseAndValidate(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Verdict != "needs_revision" {
		t.Errorf("expected verdict 'needs_revision', got: %s", result.Verdict)
	}

	if len(result.Feedback) != 1 {
		t.Errorf("expected 1 feedback item, got: %d", len(result.Feedback))
	}

	if result.Feedback[0].Section != "Dependencies" {
		t.Errorf("expected section 'Dependencies', got: %s", result.Feedback[0].Section)
	}
}

func TestParseAndValidate_InvalidVerdict(t *testing.T) {
	input := `{
		"verdict": "maybe",
		"score": {
			"completeness": 85,
			"consistency": 90,
			"testability": 80,
			"architecture": 88
		},
		"feedback": []
	}`

	_, err := ParseAndValidate(input)
	if err == nil {
		t.Fatal("expected error for invalid verdict, got none")
	}

	schemaErr, ok := err.(SchemaError)
	if !ok {
		t.Fatalf("expected SchemaError, got: %T", err)
	}

	if schemaErr.Field != "verdict" {
		t.Errorf("expected field 'verdict', got: %s", schemaErr.Field)
	}
}

func TestParseAndValidate_MissingScore(t *testing.T) {
	input := `{
		"verdict": "pass",
		"feedback": []
	}`

	_, err := ParseAndValidate(input)
	if err == nil {
		t.Fatal("expected error for missing score, got none")
	}

	schemaErr, ok := err.(SchemaError)
	if !ok {
		t.Fatalf("expected SchemaError, got: %T", err)
	}

	if schemaErr.Field != "score" {
		t.Errorf("expected field 'score', got: %s", schemaErr.Field)
	}
}

func TestParseAndValidate_NeedsRevisionWithoutFeedback(t *testing.T) {
	input := `{
		"verdict": "needs_revision",
		"score": {
			"completeness": 60,
			"consistency": 70,
			"testability": 65,
			"architecture": 75
		},
		"feedback": []
	}`

	_, err := ParseAndValidate(input)
	if err == nil {
		t.Fatal("expected error for needs_revision without feedback, got none")
	}

	schemaErr, ok := err.(SchemaError)
	if !ok {
		t.Fatalf("expected SchemaError, got: %T", err)
	}

	if schemaErr.Field != "feedback" {
		t.Errorf("expected field 'feedback', got: %s", schemaErr.Field)
	}
}

func TestParseAndValidate_ExtractsJSONFromText(t *testing.T) {
	input := `Here is my review of the specification:

	{
		"verdict": "pass",
		"score": {
			"completeness": 85,
			"consistency": 90,
			"testability": 80,
			"architecture": 88
		},
		"feedback": []
	}

	The spec looks good overall.`

	result, err := ParseAndValidate(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Verdict != "pass" {
		t.Errorf("expected verdict 'pass', got: %s", result.Verdict)
	}
}

func TestParseAndValidate_ScoreOutOfRange(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name: "score too high",
			input: `{
				"verdict": "pass",
				"score": {
					"completeness": 150,
					"consistency": 90,
					"testability": 80,
					"architecture": 88
				},
				"feedback": []
			}`,
		},
		{
			name: "score too low",
			input: `{
				"verdict": "pass",
				"score": {
					"completeness": -10,
					"consistency": 90,
					"testability": 80,
					"architecture": 88
				},
				"feedback": []
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseAndValidate(tc.input)
			if err == nil {
				t.Fatal("expected error for score out of range, got none")
			}

			schemaErr, ok := err.(SchemaError)
			if !ok {
				t.Fatalf("expected SchemaError, got: %T", err)
			}

			if !strings.HasPrefix(schemaErr.Field, "score.") {
				t.Errorf("expected field to start with 'score.', got: %s", schemaErr.Field)
			}
		})
	}
}

func TestParseAndValidate_MissingCriterion(t *testing.T) {
	input := `{
		"verdict": "pass",
		"score": {
			"completeness": 85,
			"consistency": 90,
			"testability": 80
		},
		"feedback": []
	}`

	_, err := ParseAndValidate(input)
	if err == nil {
		t.Fatal("expected error for missing criterion, got none")
	}

	schemaErr, ok := err.(SchemaError)
	if !ok {
		t.Fatalf("expected SchemaError, got: %T", err)
	}

	if schemaErr.Field != "score.architecture" {
		t.Errorf("expected field 'score.architecture', got: %s", schemaErr.Field)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "This is just plain text without any JSON"

	result := extractJSON(input)
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}

func TestExtractJSON_ValidJSON(t *testing.T) {
	input := `Some text before {"key": "value", "nested": {"data": 123}} and text after`

	result := extractJSON(input)
	expected := `{"key": "value", "nested": {"data": 123}}`

	if result != expected {
		t.Errorf("expected %s, got: %s", expected, result)
	}
}
