package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/feature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNextFeatureCmd_Defaults(t *testing.T) {
	app := New()
	cmd := NewNextFeatureCmd(app)

	require.NotNil(t, cmd)
	assert.Equal(t, "next-feature", cmd.Use[:12])

	// Check default flag values
	prdDirFlag := cmd.Flags().Lookup("top")
	require.NotNil(t, prdDirFlag)
	assert.Equal(t, "3", prdDirFlag.DefValue)

	explainFlag := cmd.Flags().Lookup("explain")
	require.NotNil(t, explainFlag)
	assert.Equal(t, "false", explainFlag.DefValue)

	jsonFlag := cmd.Flags().Lookup("json")
	require.NotNil(t, jsonFlag)
	assert.Equal(t, "false", jsonFlag.DefValue)
}

func TestNewNextFeatureCmd_Flags(t *testing.T) {
	app := New()
	cmd := NewNextFeatureCmd(app)

	// Test that all required flags exist
	flags := []string{"explain", "top", "json"}
	for _, flag := range flags {
		f := cmd.Flags().Lookup(flag)
		require.NotNil(t, f, "flag %s should exist", flag)
	}

	// Test that flags can be parsed
	err := cmd.Flags().Set("explain", "true")
	assert.NoError(t, err)

	err = cmd.Flags().Set("top", "5")
	assert.NoError(t, err)

	err = cmd.Flags().Set("json", "true")
	assert.NoError(t, err)
}

func TestFormatStandardOutput_Basic(t *testing.T) {
	result := &feature.PriorityResult{
		Recommendations: []feature.Recommendation{
			{
				PRDID:      "PRD-001",
				Title:      "User Authentication",
				Priority:   1,
				Reasoning:  "Foundation feature required by other features",
				DependsOn:  []string{},
				EnablesFor: []string{"PRD-002", "PRD-003"},
			},
			{
				PRDID:      "PRD-002",
				Title:      "User Profile",
				Priority:   2,
				Reasoning:  "Depends on authentication",
				DependsOn:  []string{"PRD-001"},
				EnablesFor: []string{},
			},
		},
		DependencyGraph: "PRD-001 -> PRD-002",
	}

	output := formatStandardOutput(result, false)

	// Check that rank, ID, and title are present
	assert.Contains(t, output, "1. [PRD-001] User Authentication")
	assert.Contains(t, output, "2. [PRD-002] User Profile")

	// Check that reasoning is present
	assert.Contains(t, output, "Reasoning: Foundation feature required by other features")
	assert.Contains(t, output, "Reasoning: Depends on authentication")

	// Check that dependencies are shown
	assert.Contains(t, output, "Depends On: PRD-001")

	// Check that enables are shown
	assert.Contains(t, output, "Enables: PRD-002, PRD-003")
}

func TestFormatStandardOutput_Explain(t *testing.T) {
	result := &feature.PriorityResult{
		Recommendations: []feature.Recommendation{
			{
				PRDID:     "PRD-001",
				Title:     "User Authentication",
				Priority:  1,
				Reasoning: "Foundation feature",
			},
		},
		Analysis: "Detailed analysis of prioritization decisions",
	}

	output := formatStandardOutput(result, true)

	// Check that priority is shown in explain mode
	assert.Contains(t, output, "Priority: 1")

	// Check that full reasoning text is included
	assert.Contains(t, output, "Detailed Analysis")
	assert.Contains(t, output, "Detailed analysis of prioritization decisions")
}

func TestFormatJSONOutput_Valid(t *testing.T) {
	result := &feature.PriorityResult{
		Recommendations: []feature.Recommendation{
			{
				PRDID:      "PRD-001",
				Title:      "User Authentication",
				Priority:   1,
				Reasoning:  "Foundation feature",
				DependsOn:  []string{},
				EnablesFor: []string{"PRD-002"},
			},
		},
		DependencyGraph: "PRD-001 -> PRD-002",
		Analysis:        "Detailed analysis",
	}

	output, err := formatJSONOutput(result)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed feature.PriorityResult
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)

	// Verify content
	assert.Equal(t, result.Recommendations[0].PRDID, parsed.Recommendations[0].PRDID)
	assert.Equal(t, result.DependencyGraph, parsed.DependencyGraph)
	assert.Equal(t, result.Analysis, parsed.Analysis)
}

func TestRunNextFeature_NoPRDDir(t *testing.T) {
	app := New()
	opts := NextFeatureOptions{
		PRDDir:  "/nonexistent/directory/that/does/not/exist",
		TopN:    3,
		Explain: false,
		JSON:    false,
	}

	err := app.RunNextFeature(context.Background(), opts)
	require.Error(t, err)

	// Check for helpful error message
	assert.Contains(t, err.Error(), "PRD directory not found")
	assert.Contains(t, err.Error(), opts.PRDDir)
}

func TestRunNextFeature_EmptyDirectory(t *testing.T) {
	// Create a temporary empty directory
	tmpDir, err := os.MkdirTemp("", "prd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	app := New()
	opts := NextFeatureOptions{
		PRDDir:  tmpDir,
		TopN:    3,
		Explain: false,
		JSON:    false,
	}

	err = app.RunNextFeature(context.Background(), opts)
	require.Error(t, err)

	// Check for helpful error message
	assert.Contains(t, err.Error(), "No PRD files found in")
}

func TestRunNextFeature_DirectoryWithNonMarkdownFiles(t *testing.T) {
	// Create a temporary directory with a non-markdown file
	tmpDir, err := os.MkdirTemp("", "prd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a non-markdown file
	txtFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(txtFile, []byte("not a markdown file"), 0644)
	require.NoError(t, err)

	app := New()
	opts := NextFeatureOptions{
		PRDDir:  tmpDir,
		TopN:    3,
		Explain: false,
		JSON:    false,
	}

	err = app.RunNextFeature(context.Background(), opts)
	require.Error(t, err)

	// Check for helpful error message about no PRD files
	assert.Contains(t, err.Error(), "No PRD files found in")
}

func TestFormatStandardOutput_NoDependencies(t *testing.T) {
	result := &feature.PriorityResult{
		Recommendations: []feature.Recommendation{
			{
				PRDID:      "PRD-001",
				Title:      "Independent Feature",
				Priority:   1,
				Reasoning:  "Can be implemented independently",
				DependsOn:  []string{},
				EnablesFor: []string{},
			},
		},
	}

	output := formatStandardOutput(result, false)

	// Should not contain "Depends On" or "Enables" sections
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		assert.NotContains(t, line, "Depends On:")
		assert.NotContains(t, line, "Enables:")
	}
}

func TestFormatStandardOutput_MultipleDependencies(t *testing.T) {
	result := &feature.PriorityResult{
		Recommendations: []feature.Recommendation{
			{
				PRDID:      "PRD-003",
				Title:      "Complex Feature",
				Priority:   3,
				Reasoning:  "Requires multiple foundations",
				DependsOn:  []string{"PRD-001", "PRD-002"},
				EnablesFor: []string{"PRD-004", "PRD-005", "PRD-006"},
			},
		},
	}

	output := formatStandardOutput(result, false)

	// Check that all dependencies are shown
	assert.Contains(t, output, "Depends On: PRD-001, PRD-002")
	assert.Contains(t, output, "Enables: PRD-004, PRD-005, PRD-006")
}
