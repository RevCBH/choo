package review

// Criterion defines a review criterion with its evaluation parameters
type Criterion struct {
	Name        string
	Description string
	MinScore    int // Minimum acceptable score (default: 70)
}

// DefaultCriteria returns the standard review criteria
func DefaultCriteria() []Criterion {
	return []Criterion{
		{
			Name:        "completeness",
			Description: "All PRD requirements have corresponding spec sections",
			MinScore:    70,
		},
		{
			Name:        "consistency",
			Description: "Types, interfaces, and naming are consistent throughout",
			MinScore:    70,
		},
		{
			Name:        "testability",
			Description: "Backpressure commands are specific and executable",
			MinScore:    70,
		},
		{
			Name:        "architecture",
			Description: "Follows existing patterns in codebase",
			MinScore:    70,
		},
	}
}

// GetCriterion returns a criterion by name, or nil if not found
func GetCriterion(name string) *Criterion {
	criteria := DefaultCriteria()
	for i := range criteria {
		if criteria[i].Name == name {
			return &criteria[i]
		}
	}
	return nil
}

// CriteriaNames returns the list of criterion names
func CriteriaNames() []string {
	criteria := DefaultCriteria()
	names := make([]string, len(criteria))
	for i, c := range criteria {
		names[i] = c.Name
	}
	return names
}

// IsPassing checks if all scores meet minimum thresholds
func IsPassing(scores map[string]int) bool {
	criteria := DefaultCriteria()
	for _, c := range criteria {
		score, exists := scores[c.Name]
		if !exists || score < c.MinScore {
			return false
		}
	}
	return true
}
