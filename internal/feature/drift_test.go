package feature

import (
	"context"
	"testing"
	"time"
)

// mockClaudeClient implements ClaudeClient for testing
type mockClaudeClient struct {
	assessDriftFunc func(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error)
}

func (m *mockClaudeClient) AssessDrift(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error) {
	if m.assessDriftFunc != nil {
		return m.assessDriftFunc(ctx, req)
	}
	return &DriftAssessment{
		Significant:    false,
		AffectedUnits:  []string{},
		Recommendation: "No action needed",
	}, nil
}

func TestDriftDetector_NoDrift(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{}
	detector := NewDriftDetector(prd, mockClaude)

	result, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}

	if result.HasDrift {
		t.Errorf("CheckDrift() HasDrift = true, expected false when body unchanged")
	}
}

func TestDriftDetector_DetectsDrift(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{
		assessDriftFunc: func(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error) {
			return &DriftAssessment{
				Significant:    false,
				AffectedUnits:  []string{},
				Recommendation: "Minor change",
			}, nil
		},
	}

	detector := NewDriftDetector(prd, mockClaude)

	// Change the PRD body
	prd.Body = "Updated PRD content"

	result, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}

	if !result.HasDrift {
		t.Errorf("CheckDrift() HasDrift = false, expected true when body changed")
	}
}

func TestDriftDetector_HashComparison(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content that is reasonably long to simulate a real PRD document with multiple sections and details",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{}
	detector := NewDriftDetector(prd, mockClaude)

	start := time.Now()
	_, err := detector.CheckDrift(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}

	if elapsed > 50*time.Millisecond {
		t.Errorf("Hash comparison took %v, expected < 50ms", elapsed)
	}
}

func TestDriftDetector_SignificantDrift(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
			{Name: "unit2", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{
		assessDriftFunc: func(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error) {
			return &DriftAssessment{
				Significant:    true,
				AffectedUnits:  []string{"unit1", "unit2"},
				Recommendation: "Major changes detected. Review and update affected units.",
			}, nil
		},
	}

	detector := NewDriftDetector(prd, mockClaude)

	// Change the PRD body significantly
	prd.Body = "Completely rewritten PRD with different requirements and scope"

	result, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}

	if !result.Significant {
		t.Errorf("CheckDrift() Significant = false, expected true for significant changes")
	}

	if len(result.AffectedUnits) != 2 {
		t.Errorf("CheckDrift() AffectedUnits count = %d, expected 2", len(result.AffectedUnits))
	}
}

func TestDriftDetector_MinorDrift(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{
		assessDriftFunc: func(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error) {
			return &DriftAssessment{
				Significant:    false,
				AffectedUnits:  []string{},
				Recommendation: "Minor typo fixes. No action needed.",
			}, nil
		},
	}

	detector := NewDriftDetector(prd, mockClaude)

	// Minor change to PRD body
	prd.Body = "Original PRD content with minor typo fix"

	result, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}

	if result.Significant {
		t.Errorf("CheckDrift() Significant = true, expected false for minor changes")
	}

	if len(result.AffectedUnits) != 0 {
		t.Errorf("CheckDrift() AffectedUnits count = %d, expected 0 for minor changes", len(result.AffectedUnits))
	}
}

func TestDriftDetector_UpdateBaseline(t *testing.T) {
	prd := &DriftPRD{
		Body: "Original PRD content",
		Units: []DriftUnit{
			{Name: "unit1", Status: "in_progress"},
		},
	}

	mockClaude := &mockClaudeClient{}
	detector := NewDriftDetector(prd, mockClaude)

	// Change the PRD body
	prd.Body = "Updated PRD content"

	// First check should detect drift
	result1, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}
	if !result1.HasDrift {
		t.Errorf("First CheckDrift() HasDrift = false, expected true")
	}

	// Update baseline
	detector.UpdateBaseline()

	// Second check should not detect drift
	result2, err := detector.CheckDrift(context.Background())
	if err != nil {
		t.Fatalf("CheckDrift() error = %v", err)
	}
	if result2.HasDrift {
		t.Errorf("CheckDrift() after UpdateBaseline() HasDrift = true, expected false")
	}
}

func TestHashBody_Deterministic(t *testing.T) {
	input := "Test PRD content"

	hash1 := hashBody(input)
	hash2 := hashBody(input)

	if hash1 != hash2 {
		t.Errorf("hashBody() produced different hashes for same input: %s != %s", hash1, hash2)
	}

	// Different input should produce different hash
	hash3 := hashBody("Different content")
	if hash1 == hash3 {
		t.Errorf("hashBody() produced same hash for different inputs")
	}
}

func TestComputeDiff_EmptyBodies(t *testing.T) {
	tests := []struct {
		name     string
		oldBody  string
		newBody  string
		expected string
	}{
		{
			name:     "both empty",
			oldBody:  "",
			newBody:  "",
			expected: "No changes",
		},
		{
			name:     "old empty, new has content",
			oldBody:  "",
			newBody:  "New content",
			expected: "Added: 11 characters",
		},
		{
			name:     "new empty, old has content",
			oldBody:  "Old content",
			newBody:  "",
			expected: "Removed: 11 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeDiff(tt.oldBody, tt.newBody)
			if result != tt.expected {
				t.Errorf("computeDiff() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
