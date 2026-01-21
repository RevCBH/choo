package feature

import (
	"strings"
	"testing"
)

func TestValidatePRD_Valid(t *testing.T) {
	prd := &PRD{
		ID:     "test-feature",
		Title:  "Test Feature",
		Status: PRDStatusDraft,
	}
	err := ValidatePRD(prd)
	if err != nil {
		t.Errorf("ValidatePRD() returned error for valid PRD: %v", err)
	}
}

func TestValidatePRD_MissingID(t *testing.T) {
	prd := &PRD{
		Title:  "Test Feature",
		Status: PRDStatusDraft,
	}
	err := ValidatePRD(prd)
	if err == nil {
		t.Fatal("ValidatePRD() expected error for missing ID, got nil")
	}
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "prd_id" {
		t.Errorf("expected Field to be 'prd_id', got %q", validationErr.Field)
	}
}

func TestValidatePRD_MissingTitle(t *testing.T) {
	prd := &PRD{
		ID:     "test-feature",
		Status: PRDStatusDraft,
	}
	err := ValidatePRD(prd)
	if err == nil {
		t.Fatal("ValidatePRD() expected error for missing Title, got nil")
	}
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "title" {
		t.Errorf("expected Field to be 'title', got %q", validationErr.Field)
	}
}

func TestValidatePRD_MissingStatus(t *testing.T) {
	prd := &PRD{
		ID:    "test-feature",
		Title: "Test Feature",
	}
	err := ValidatePRD(prd)
	if err == nil {
		t.Fatal("ValidatePRD() expected error for missing Status, got nil")
	}
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "status" {
		t.Errorf("expected Field to be 'status', got %q", validationErr.Field)
	}
}

func TestValidatePRD_InvalidStatus(t *testing.T) {
	prd := &PRD{
		ID:     "test-feature",
		Title:  "Test Feature",
		Status: "unknown",
	}
	err := ValidatePRD(prd)
	if err == nil {
		t.Fatal("ValidatePRD() expected error for invalid Status, got nil")
	}
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "status" {
		t.Errorf("expected Field to be 'status', got %q", validationErr.Field)
	}
}

func TestValidatePRDID_TooShort(t *testing.T) {
	err := validatePRDID("a")
	if err == nil {
		t.Fatal("validatePRDID() expected error for single character ID, got nil")
	}
}

func TestValidatePRDID_TooLong(t *testing.T) {
	longID := strings.Repeat("a", 51)
	err := validatePRDID(longID)
	if err == nil {
		t.Fatal("validatePRDID() expected error for ID over 50 characters, got nil")
	}
}

func TestValidatePRDID_InvalidChars(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{"uppercase", "Test-Feature"},
		{"special chars", "test_feature"},
		{"spaces", "test feature"},
		{"exclamation", "test-feature!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePRDID(tc.id)
			if err == nil {
				t.Errorf("validatePRDID(%q) expected error for invalid characters, got nil", tc.id)
			}
		})
	}
}

func TestValidatePRDID_LeadingHyphen(t *testing.T) {
	err := validatePRDID("-test-feature")
	if err == nil {
		t.Fatal("validatePRDID() expected error for leading hyphen, got nil")
	}
}

func TestValidatePRDID_TrailingHyphen(t *testing.T) {
	err := validatePRDID("test-feature-")
	if err == nil {
		t.Fatal("validatePRDID() expected error for trailing hyphen, got nil")
	}
}

func TestValidatePRDID_Valid(t *testing.T) {
	err := validatePRDID("test-feature-01")
	if err != nil {
		t.Errorf("validatePRDID() returned error for valid ID: %v", err)
	}
}
