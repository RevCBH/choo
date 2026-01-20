package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var validPRDContent = `---
prd_id: test-feature
title: "Test Feature"
status: draft
depends_on:
  - other-feature
estimated_units: 3
estimated_tasks: 12
---

# Test Feature

This is the PRD body content.
`

var minimalPRDContent = `---
prd_id: minimal
title: Minimal PRD
status: approved
---

Body here.
`

var invalidYAMLContent = `---
prd_id: [invalid yaml
---

Body.
`

func TestParsePRD_ValidComplete(t *testing.T) {
	r := strings.NewReader(validPRDContent)
	prd, err := ParsePRDFromReader(r, "test.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	// Verify all frontmatter fields
	if prd.ID != "test-feature" {
		t.Errorf("ID = %q, want %q", prd.ID, "test-feature")
	}
	if prd.Title != "Test Feature" {
		t.Errorf("Title = %q, want %q", prd.Title, "Test Feature")
	}
	if prd.Status != "draft" {
		t.Errorf("Status = %q, want %q", prd.Status, "draft")
	}
	if len(prd.DependsOn) != 1 || prd.DependsOn[0] != "other-feature" {
		t.Errorf("DependsOn = %v, want [other-feature]", prd.DependsOn)
	}
	if prd.EstimatedUnits != 3 {
		t.Errorf("EstimatedUnits = %d, want 3", prd.EstimatedUnits)
	}
	if prd.EstimatedTasks != 12 {
		t.Errorf("EstimatedTasks = %d, want 12", prd.EstimatedTasks)
	}
	if prd.FilePath != "test.md" {
		t.Errorf("FilePath = %q, want %q", prd.FilePath, "test.md")
	}
}

func TestParsePRD_MinimalFields(t *testing.T) {
	r := strings.NewReader(minimalPRDContent)
	prd, err := ParsePRDFromReader(r, "minimal.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	// Verify required fields
	if prd.ID != "minimal" {
		t.Errorf("ID = %q, want %q", prd.ID, "minimal")
	}
	if prd.Title != "Minimal PRD" {
		t.Errorf("Title = %q, want %q", prd.Title, "Minimal PRD")
	}
	if prd.Status != "approved" {
		t.Errorf("Status = %q, want %q", prd.Status, "approved")
	}

	// Verify optional fields have zero values
	if len(prd.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty", prd.DependsOn)
	}
	if prd.EstimatedUnits != 0 {
		t.Errorf("EstimatedUnits = %d, want 0", prd.EstimatedUnits)
	}
	if prd.EstimatedTasks != 0 {
		t.Errorf("EstimatedTasks = %d, want 0", prd.EstimatedTasks)
	}
}

func TestParsePRD_WithDependsOn(t *testing.T) {
	content := `---
prd_id: depends-test
title: Depends Test
status: draft
depends_on:
  - feature-a
  - feature-b
  - feature-c
---

Body.
`
	r := strings.NewReader(content)
	prd, err := ParsePRDFromReader(r, "depends.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	if len(prd.DependsOn) != 3 {
		t.Fatalf("len(DependsOn) = %d, want 3", len(prd.DependsOn))
	}
	expected := []string{"feature-a", "feature-b", "feature-c"}
	for i, dep := range prd.DependsOn {
		if dep != expected[i] {
			t.Errorf("DependsOn[%d] = %q, want %q", i, dep, expected[i])
		}
	}
}

func TestParsePRD_WithEstimates(t *testing.T) {
	content := `---
prd_id: estimates-test
title: Estimates Test
status: draft
estimated_units: 5
estimated_tasks: 20
---

Body.
`
	r := strings.NewReader(content)
	prd, err := ParsePRDFromReader(r, "estimates.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	if prd.EstimatedUnits != 5 {
		t.Errorf("EstimatedUnits = %d, want 5", prd.EstimatedUnits)
	}
	if prd.EstimatedTasks != 20 {
		t.Errorf("EstimatedTasks = %d, want 20", prd.EstimatedTasks)
	}
}

func TestParsePRD_WithOrchestratorFields(t *testing.T) {
	content := `---
prd_id: orchestrator-test
title: Orchestrator Test
status: in_progress
feature_branch: "feat/test"
feature_status: "in_progress"
feature_started_at: "2025-01-15T10:00:00Z"
feature_completed_at: "2025-01-20T15:30:00Z"
spec_review_iterations: 2
last_spec_review: "2025-01-14T09:00:00Z"
---

Body.
`
	r := strings.NewReader(content)
	prd, err := ParsePRDFromReader(r, "orchestrator.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	if prd.FeatureBranch != "feat/test" {
		t.Errorf("FeatureBranch = %q, want %q", prd.FeatureBranch, "feat/test")
	}
	if prd.FeatureStatus != "in_progress" {
		t.Errorf("FeatureStatus = %q, want %q", prd.FeatureStatus, "in_progress")
	}
	if prd.SpecReviewIterations != 2 {
		t.Errorf("SpecReviewIterations = %d, want 2", prd.SpecReviewIterations)
	}
	if prd.FeatureStartedAt == nil {
		t.Error("FeatureStartedAt is nil, want non-nil")
	}
	if prd.FeatureCompletedAt == nil {
		t.Error("FeatureCompletedAt is nil, want non-nil")
	}
	if prd.LastSpecReview == nil {
		t.Error("LastSpecReview is nil, want non-nil")
	}
}

func TestParsePRD_BodyContent(t *testing.T) {
	r := strings.NewReader(validPRDContent)
	prd, err := ParsePRDFromReader(r, "test.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	expectedBody := "\n# Test Feature\n\nThis is the PRD body content.\n"
	if prd.Body != expectedBody {
		t.Errorf("Body = %q, want %q", prd.Body, expectedBody)
	}
}

func TestParsePRD_BodyHashComputed(t *testing.T) {
	r := strings.NewReader(validPRDContent)
	prd, err := ParsePRDFromReader(r, "test.md")
	if err != nil {
		t.Fatalf("ParsePRDFromReader failed: %v", err)
	}

	if prd.BodyHash == "" {
		t.Error("BodyHash is empty, want non-empty")
	}
	if len(prd.BodyHash) != 64 {
		t.Errorf("len(BodyHash) = %d, want 64 (SHA-256 hex)", len(prd.BodyHash))
	}
	// Verify it's hex
	for _, c := range prd.BodyHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("BodyHash contains non-hex character: %c", c)
		}
	}
}

func TestParsePRDFromReader_Error(t *testing.T) {
	r := strings.NewReader(invalidYAMLContent)
	_, err := ParsePRDFromReader(r, "invalid.md")
	if err == nil {
		t.Fatal("ParsePRDFromReader succeeded, want error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error = %q, want error containing 'unmarshal'", err.Error())
	}
}

func TestComputeBodyHash_Deterministic(t *testing.T) {
	body := "Same content"
	hash1 := ComputeBodyHash(body)
	hash2 := ComputeBodyHash(body)

	if hash1 != hash2 {
		t.Errorf("hash1 = %s, hash2 = %s, want same hash for same content", hash1, hash2)
	}
}

func TestComputeBodyHash_Different(t *testing.T) {
	body1 := "Content A"
	body2 := "Content B"
	hash1 := ComputeBodyHash(body1)
	hash2 := ComputeBodyHash(body2)

	if hash1 == hash2 {
		t.Errorf("hash1 = hash2 = %s, want different hashes for different content", hash1)
	}
}

var featureAPRD = `---
prd_id: feature-a
title: Feature A
status: draft
---

# Feature A
`

var featureBPRD = `---
prd_id: feature-b
title: Feature B
status: approved
---

# Feature B
`

var featureCPRD = `---
prd_id: feature-c
title: Feature C
status: draft
---

# Feature C
`

func TestDiscoverPRDs_Empty(t *testing.T) {
	dir := t.TempDir()
	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	if len(prds) != 0 {
		t.Errorf("len(prds) = %d, want 0", len(prds))
	}
}

func TestDiscoverPRDs_SingleFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	if len(prds) != 1 {
		t.Fatalf("len(prds) = %d, want 1", len(prds))
	}
	if prds[0].ID != "feature-a" {
		t.Errorf("prds[0].ID = %q, want %q", prds[0].ID, "feature-a")
	}
}

func TestDiscoverPRDs_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-b.md"), []byte(featureBPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	if len(prds) != 2 {
		t.Fatalf("len(prds) = %d, want 2", len(prds))
	}

	ids := make(map[string]bool)
	for _, prd := range prds {
		ids[prd.ID] = true
	}
	if !ids["feature-a"] || !ids["feature-b"] {
		t.Errorf("prds IDs = %v, want feature-a and feature-b", ids)
	}
}

func TestDiscoverPRDs_Recursive(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	nestedDir := filepath.Join(dir, "nested")
	err = os.Mkdir(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(nestedDir, "feature-c.md"), []byte(featureCPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	if len(prds) != 2 {
		t.Fatalf("len(prds) = %d, want 2", len(prds))
	}

	ids := make(map[string]bool)
	for _, prd := range prds {
		ids[prd.ID] = true
	}
	if !ids["feature-a"] || !ids["feature-c"] {
		t.Errorf("prds IDs = %v, want feature-a and feature-c", ids)
	}
}

func TestDiscoverPRDs_SkipsReadme(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# README\n"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	if len(prds) != 1 {
		t.Fatalf("len(prds) = %d, want 1", len(prds))
	}
	if prds[0].ID != "feature-a" {
		t.Errorf("prds[0].ID = %q, want %q", prds[0].ID, "feature-a")
	}
}

func TestDiscoverPRDs_SkipsInvalid(t *testing.T) {
	dir := t.TempDir()
	invalidPRD := `---
prd_id: [invalid yaml
---

# Invalid
`
	err := os.WriteFile(filepath.Join(dir, "invalid.md"), []byte(invalidPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDs(dir)
	if err != nil {
		t.Fatalf("DiscoverPRDs failed: %v", err)
	}
	// Should only include feature-a, invalid.md should be skipped with a warning
	if len(prds) != 1 {
		t.Fatalf("len(prds) = %d, want 1", len(prds))
	}
	if prds[0].ID != "feature-a" {
		t.Errorf("prds[0].ID = %q, want %q", prds[0].ID, "feature-a")
	}
}

func TestDiscoverPRDs_NonExistentDir(t *testing.T) {
	dir := "/nonexistent/directory/path"
	_, err := DiscoverPRDs(dir)
	if err == nil {
		t.Fatal("DiscoverPRDs succeeded, want error for non-existent directory")
	}
}

func TestDiscoverPRDsWithFilter_NoFilter(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-b.md"), []byte(featureBPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDsWithFilter(dir, []string{})
	if err != nil {
		t.Fatalf("DiscoverPRDsWithFilter failed: %v", err)
	}
	if len(prds) != 2 {
		t.Fatalf("len(prds) = %d, want 2", len(prds))
	}
}

func TestDiscoverPRDsWithFilter_SingleStatus(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-b.md"), []byte(featureBPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDsWithFilter(dir, []string{"draft"})
	if err != nil {
		t.Fatalf("DiscoverPRDsWithFilter failed: %v", err)
	}
	if len(prds) != 1 {
		t.Fatalf("len(prds) = %d, want 1", len(prds))
	}
	if prds[0].ID != "feature-a" {
		t.Errorf("prds[0].ID = %q, want %q", prds[0].ID, "feature-a")
	}
	if prds[0].Status != "draft" {
		t.Errorf("prds[0].Status = %q, want %q", prds[0].Status, "draft")
	}
}

func TestDiscoverPRDsWithFilter_MultipleStatuses(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-b.md"), []byte(featureBPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "feature-c.md"), []byte(featureCPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDsWithFilter(dir, []string{"draft", "approved"})
	if err != nil {
		t.Fatalf("DiscoverPRDsWithFilter failed: %v", err)
	}
	if len(prds) != 3 {
		t.Fatalf("len(prds) = %d, want 3", len(prds))
	}

	statuses := make(map[string]int)
	for _, prd := range prds {
		statuses[prd.Status]++
	}
	if statuses["draft"] != 2 || statuses["approved"] != 1 {
		t.Errorf("statuses = %v, want 2 draft and 1 approved", statuses)
	}
}

func TestDiscoverPRDsWithFilter_NoMatches(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "feature-a.md"), []byte(featureAPRD), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	prds, err := DiscoverPRDsWithFilter(dir, []string{"complete"})
	if err != nil {
		t.Fatalf("DiscoverPRDsWithFilter failed: %v", err)
	}
	if len(prds) != 0 {
		t.Errorf("len(prds) = %d, want 0", len(prds))
	}
}
