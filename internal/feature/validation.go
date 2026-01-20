package feature

import (
	"fmt"
	"regexp"
)

// validPRDID matches lowercase alphanumeric with hyphens
// Must start and end with alphanumeric, 2-50 characters
var validPRDID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// validatePRDID checks that the ID is valid for use in git branch names
func validatePRDID(id string) error {
	if len(id) < 2 {
		return ValidationError{
			Field:   "prd_id",
			Message: "too short (minimum 2 characters)",
		}
	}
	if len(id) > 50 {
		return ValidationError{
			Field:   "prd_id",
			Message: "too long (maximum 50 characters)",
		}
	}
	if !validPRDID.MatchString(id) {
		return ValidationError{
			Field:   "prd_id",
			Message: "must be lowercase alphanumeric with hyphens, no leading/trailing hyphens",
		}
	}
	return nil
}

// ValidatePRD checks that all required fields are present and valid
// Returns nil if valid, ValidationError if invalid
func ValidatePRD(prd *PRD) error {
	if prd.ID == "" {
		return ValidationError{Field: "prd_id", Message: "required"}
	}
	if err := validatePRDID(prd.ID); err != nil {
		return err
	}
	if prd.Title == "" {
		return ValidationError{Field: "title", Message: "required"}
	}
	if prd.Status == "" {
		return ValidationError{Field: "status", Message: "required"}
	}
	if !IsValidPRDStatus(prd.Status) {
		return ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("invalid value %q, must be one of: draft, approved, in_progress, complete, archived", prd.Status),
		}
	}
	return nil
}
