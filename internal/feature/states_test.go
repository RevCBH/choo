package feature

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     FeatureStatus
		to       FeatureStatus
		expected bool
	}{
		{"pending to generating_specs", StatusPending, StatusGeneratingSpecs, true},
		{"pending to complete", StatusPending, StatusComplete, false},
		{"review_blocked to reviewing_specs", StatusReviewBlocked, StatusReviewingSpecs, true},
		{"complete to pending", StatusComplete, StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("CanTransition(%v, %v) = %v, expected %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestParseFeatureStatus(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    FeatureStatus
		shouldError bool
	}{
		{"valid pending", "pending", StatusPending, false},
		{"invalid status", "invalid", "", true},
		{"empty string defaults to pending", "", StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFeatureStatus(tt.input)
			if tt.shouldError && err == nil {
				t.Errorf("ParseFeatureStatus(%q) expected error, got nil", tt.input)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ParseFeatureStatus(%q) unexpected error: %v", tt.input, err)
			}
			if !tt.shouldError && result != tt.expected {
				t.Errorf("ParseFeatureStatus(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   FeatureStatus
		expected bool
	}{
		{"complete is terminal", StatusComplete, true},
		{"failed is terminal", StatusFailed, true},
		{"in_progress is not terminal", StatusInProgress, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsTerminal()
			if result != tt.expected {
				t.Errorf("%v.IsTerminal() = %v, expected %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestCanResume(t *testing.T) {
	tests := []struct {
		name     string
		status   FeatureStatus
		expected bool
	}{
		{"review_blocked can resume", StatusReviewBlocked, true},
		{"in_progress cannot resume", StatusInProgress, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.CanResume()
			if result != tt.expected {
				t.Errorf("%v.CanResume() = %v, expected %v", tt.status, result, tt.expected)
			}
		})
	}
}
