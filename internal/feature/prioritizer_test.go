package feature

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mock agent invoker for testing
type mockInvoker struct {
	response string
	err      error
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

func TestNewPrioritizer(t *testing.T) {
	prdDir := "/path/to/prds"
	specsDir := "/path/to/specs"

	p := NewPrioritizer(prdDir, specsDir)

	if p.prdDir != prdDir {
		t.Errorf("Expected prdDir %s, got %s", prdDir, p.prdDir)
	}
	if p.specsDir != specsDir {
		t.Errorf("Expected specsDir %s, got %s", specsDir, p.specsDir)
	}
}

func TestPrioritizer_BuildPrompt_IncludesPRDs(t *testing.T) {
	p := NewPrioritizer("", "")

	prds := []*PRDForPrioritization{
		{
			ID:      "auth",
			Title:   "Authentication System",
			Content: "# Authentication System\n\nImplement OAuth2 authentication.",
		},
		{
			ID:      "dashboard",
			Title:   "User Dashboard",
			Content: "# User Dashboard\n\nCreate a dashboard for users.",
		},
	}

	opts := DefaultPrioritizeOptions()
	prompt := p.buildPrompt(prds, []string{}, opts)

	// Check that prompt contains PRD IDs
	if !strings.Contains(prompt, "auth") {
		t.Error("Prompt should contain PRD ID 'auth'")
	}
	if !strings.Contains(prompt, "dashboard") {
		t.Error("Prompt should contain PRD ID 'dashboard'")
	}

	// Check that prompt contains PRD titles
	if !strings.Contains(prompt, "Authentication System") {
		t.Error("Prompt should contain PRD title 'Authentication System'")
	}
	if !strings.Contains(prompt, "User Dashboard") {
		t.Error("Prompt should contain PRD title 'User Dashboard'")
	}

	// Check that prompt contains PRD content
	if !strings.Contains(prompt, "OAuth2") {
		t.Error("Prompt should contain PRD content 'OAuth2'")
	}
}

func TestPrioritizer_BuildPrompt_IncludesSpecs(t *testing.T) {
	p := NewPrioritizer("", "")

	prds := []*PRDForPrioritization{
		{
			ID:      "feature",
			Title:   "New Feature",
			Content: "# New Feature\n\nSome content.",
		},
	}

	specs := []string{"AUTHENTICATION.md", "DATABASE.md"}
	opts := DefaultPrioritizeOptions()
	prompt := p.buildPrompt(prds, specs, opts)

	// Check that prompt contains spec names
	if !strings.Contains(prompt, "AUTHENTICATION.md") {
		t.Error("Prompt should contain spec 'AUTHENTICATION.md'")
	}
	if !strings.Contains(prompt, "DATABASE.md") {
		t.Error("Prompt should contain spec 'DATABASE.md'")
	}

	// Check that there's a section for existing specs
	if !strings.Contains(prompt, "Existing Completed Specs") {
		t.Error("Prompt should have 'Existing Completed Specs' section")
	}
}

func TestPrioritizer_Prioritize_Success(t *testing.T) {
	// Create temp directory for test PRDs
	tmpDir := t.TempDir()

	// Write a test PRD
	prdPath := filepath.Join(tmpDir, "test-feature.md")
	prdContent := "# Test Feature\n\nThis is a test feature."
	if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	// Create prioritizer
	p := NewPrioritizer(tmpDir, "")

	// Mock response
	mockResponse := `{
		"recommendations": [
			{
				"prd_id": "test-feature",
				"title": "Test Feature",
				"priority": 1,
				"reasoning": "Foundation feature",
				"depends_on": [],
				"enables_for": ["other-feature"]
			}
		],
		"dependency_graph": "test-feature is a foundation"
	}`

	invoker := &mockInvoker{response: mockResponse}
	opts := DefaultPrioritizeOptions()

	result, err := p.Prioritize(context.Background(), invoker, opts)
	if err != nil {
		t.Fatalf("Prioritize failed: %v", err)
	}

	// Verify result
	if len(result.Recommendations) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(result.Recommendations))
	}

	if result.Recommendations[0].PRDID != "test-feature" {
		t.Errorf("Expected PRDID 'test-feature', got %s", result.Recommendations[0].PRDID)
	}

	if result.DependencyGraph != "test-feature is a foundation" {
		t.Errorf("Expected dependency graph to match, got %s", result.DependencyGraph)
	}
}

func TestPrioritizer_Prioritize_TopN(t *testing.T) {
	// Create temp directory for test PRDs
	tmpDir := t.TempDir()

	// Write test PRDs
	for i := 1; i <= 3; i++ {
		prdPath := filepath.Join(tmpDir, "feature"+string(rune('0'+i))+".md")
		prdContent := "# Feature " + string(rune('0'+i))
		if err := os.WriteFile(prdPath, []byte(prdContent), 0644); err != nil {
			t.Fatalf("Failed to write test PRD: %v", err)
		}
	}

	p := NewPrioritizer(tmpDir, "")

	// Mock response with 5 recommendations
	mockResponse := `{
		"recommendations": [
			{"prd_id": "feature1", "title": "Feature 1", "priority": 1, "reasoning": "First", "depends_on": [], "enables_for": []},
			{"prd_id": "feature2", "title": "Feature 2", "priority": 2, "reasoning": "Second", "depends_on": [], "enables_for": []},
			{"prd_id": "feature3", "title": "Feature 3", "priority": 3, "reasoning": "Third", "depends_on": [], "enables_for": []},
			{"prd_id": "feature4", "title": "Feature 4", "priority": 4, "reasoning": "Fourth", "depends_on": [], "enables_for": []},
			{"prd_id": "feature5", "title": "Feature 5", "priority": 5, "reasoning": "Fifth", "depends_on": [], "enables_for": []}
		],
		"dependency_graph": "linear"
	}`

	invoker := &mockInvoker{response: mockResponse}
	opts := PrioritizeOptions{TopN: 2}

	result, err := p.Prioritize(context.Background(), invoker, opts)
	if err != nil {
		t.Fatalf("Prioritize failed: %v", err)
	}

	// Verify only TopN recommendations are returned
	if len(result.Recommendations) != 2 {
		t.Errorf("Expected 2 recommendations (TopN=2), got %d", len(result.Recommendations))
	}

	// Verify correct recommendations were kept
	if result.Recommendations[0].PRDID != "feature1" {
		t.Errorf("Expected first recommendation to be 'feature1', got %s", result.Recommendations[0].PRDID)
	}
	if result.Recommendations[1].PRDID != "feature2" {
		t.Errorf("Expected second recommendation to be 'feature2', got %s", result.Recommendations[1].PRDID)
	}
}

func TestPrioritizer_Prioritize_NoPRDs(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	p := NewPrioritizer(tmpDir, "")
	invoker := &mockInvoker{response: "{}"}
	opts := DefaultPrioritizeOptions()

	_, err := p.Prioritize(context.Background(), invoker, opts)
	if err == nil {
		t.Error("Expected error when no PRDs found, got nil")
	}

	// Error should mention no PRDs found
	if !strings.Contains(err.Error(), "no") || !strings.Contains(err.Error(), "found") {
		t.Errorf("Error should mention no PRDs found, got: %v", err)
	}
}

func TestPrioritizer_Prioritize_InvokerError(t *testing.T) {
	// Create temp directory with a PRD
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(prdPath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write test PRD: %v", err)
	}

	p := NewPrioritizer(tmpDir, "")

	// Mock invoker that returns an error
	invoker := &mockInvoker{err: os.ErrPermission}
	opts := DefaultPrioritizeOptions()

	_, err := p.Prioritize(context.Background(), invoker, opts)
	if err == nil {
		t.Error("Expected error from invoker to be propagated, got nil")
	}

	// Error should wrap the original error
	if !strings.Contains(err.Error(), "agent invocation failed") {
		t.Errorf("Error should mention agent invocation failure, got: %v", err)
	}
}
