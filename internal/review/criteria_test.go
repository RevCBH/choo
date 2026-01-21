package review

import "testing"

func TestDefaultCriteria_Count(t *testing.T) {
	criteria := DefaultCriteria()
	if len(criteria) != 4 {
		t.Errorf("expected 4 criteria, got: %d", len(criteria))
	}
}

func TestDefaultCriteria_Names(t *testing.T) {
	criteria := DefaultCriteria()
	expectedNames := map[string]bool{
		"completeness": false,
		"consistency":  false,
		"testability":  false,
		"architecture": false,
	}

	for _, c := range criteria {
		if _, exists := expectedNames[c.Name]; exists {
			expectedNames[c.Name] = true
		} else {
			t.Errorf("unexpected criterion name: %s", c.Name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing expected criterion: %s", name)
		}
	}
}

func TestDefaultCriteria_MinScores(t *testing.T) {
	criteria := DefaultCriteria()
	for _, c := range criteria {
		if c.MinScore != 70 {
			t.Errorf("expected MinScore 70 for %s, got: %d", c.Name, c.MinScore)
		}
	}
}

func TestGetCriterion_Found(t *testing.T) {
	criterion := GetCriterion("completeness")
	if criterion == nil {
		t.Fatal("expected criterion to be found, got nil")
	}

	if criterion.Name != "completeness" {
		t.Errorf("expected name 'completeness', got: %s", criterion.Name)
	}

	if criterion.Description != "All PRD requirements have corresponding spec sections" {
		t.Errorf("unexpected description: %s", criterion.Description)
	}

	if criterion.MinScore != 70 {
		t.Errorf("expected MinScore 70, got: %d", criterion.MinScore)
	}
}

func TestGetCriterion_NotFound(t *testing.T) {
	criterion := GetCriterion("nonexistent")
	if criterion != nil {
		t.Errorf("expected nil for unknown criterion, got: %+v", criterion)
	}
}

func TestCriteriaNames(t *testing.T) {
	names := CriteriaNames()
	if len(names) != 4 {
		t.Errorf("expected 4 names, got: %d", len(names))
	}

	expectedNames := map[string]bool{
		"completeness": false,
		"consistency":  false,
		"testability":  false,
		"architecture": false,
	}

	for _, name := range names {
		if _, exists := expectedNames[name]; exists {
			expectedNames[name] = true
		} else {
			t.Errorf("unexpected name: %s", name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing expected name: %s", name)
		}
	}
}

func TestIsPassing_AllPass(t *testing.T) {
	scores := map[string]int{
		"completeness": 75,
		"consistency":  80,
		"testability":  85,
		"architecture": 90,
	}

	if !IsPassing(scores) {
		t.Error("expected IsPassing to return true when all scores >= 70")
	}
}

func TestIsPassing_OneFails(t *testing.T) {
	scores := map[string]int{
		"completeness": 75,
		"consistency":  80,
		"testability":  65, // Below threshold
		"architecture": 90,
	}

	if IsPassing(scores) {
		t.Error("expected IsPassing to return false when any score < 70")
	}
}

func TestIsPassing_BoundaryScore(t *testing.T) {
	scores := map[string]int{
		"completeness": 70,
		"consistency":  70,
		"testability":  70,
		"architecture": 70,
	}

	if !IsPassing(scores) {
		t.Error("expected IsPassing to return true when score == 70 exactly")
	}
}
