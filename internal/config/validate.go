package config

import (
	"errors"
	"fmt"
	"time"
)

// ValidationError contains details about what failed validation.
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config.%s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// validateConfig checks all config values for validity.
// Returns nil if valid, or joined errors for all validation failures.
func validateConfig(cfg *Config) error {
	var errs []error

	// Parallelism must be >= 1
	if cfg.Parallelism < 1 {
		errs = append(errs, &ValidationError{
			Field:   "parallelism",
			Value:   cfg.Parallelism,
			Message: "must be at least 1",
		})
	}

	// GitHub.Owner must not be empty or "auto" after detection
	if cfg.GitHub.Owner == "" || cfg.GitHub.Owner == "auto" {
		errs = append(errs, &ValidationError{
			Field:   "github.owner",
			Value:   cfg.GitHub.Owner,
			Message: "must be set or auto-detectable",
		})
	}

	// GitHub.Repo must not be empty or "auto" after detection
	if cfg.GitHub.Repo == "" || cfg.GitHub.Repo == "auto" {
		errs = append(errs, &ValidationError{
			Field:   "github.repo",
			Value:   cfg.GitHub.Repo,
			Message: "must be set or auto-detectable",
		})
	}

	// Claude.Command must not be empty
	if cfg.Claude.Command == "" {
		errs = append(errs, &ValidationError{
			Field:   "claude.command",
			Value:   cfg.Claude.Command,
			Message: "must not be empty",
		})
	}

	// Claude.MaxTurns must be >= 0 (0 = unlimited)
	if cfg.Claude.MaxTurns < 0 {
		errs = append(errs, &ValidationError{
			Field:   "claude.max_turns",
			Value:   cfg.Claude.MaxTurns,
			Message: "must be non-negative (0 = unlimited)",
		})
	}

	// Merge.MaxConflictRetries must be >= 1
	if cfg.Merge.MaxConflictRetries < 1 {
		errs = append(errs, &ValidationError{
			Field:   "merge.max_conflict_retries",
			Value:   cfg.Merge.MaxConflictRetries,
			Message: "must be at least 1",
		})
	}

	// Review.Timeout must be valid Go duration string
	if _, err := time.ParseDuration(cfg.Review.Timeout); err != nil {
		errs = append(errs, &ValidationError{
			Field:   "review.timeout",
			Value:   cfg.Review.Timeout,
			Message: fmt.Sprintf("invalid duration: %v", err),
		})
	}

	// Review.PollInterval must be valid Go duration string
	if _, err := time.ParseDuration(cfg.Review.PollInterval); err != nil {
		errs = append(errs, &ValidationError{
			Field:   "review.poll_interval",
			Value:   cfg.Review.PollInterval,
			Message: fmt.Sprintf("invalid duration: %v", err),
		})
	}

	// CodeReview validation
	if err := cfg.CodeReview.Validate(); err != nil {
		errs = append(errs, &ValidationError{
			Field:   "code_review",
			Value:   cfg.CodeReview.Provider,
			Message: err.Error(),
		})
	}

	// LogLevel must be one of: debug, info, warn, error (case-sensitive)
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[cfg.LogLevel] {
		errs = append(errs, &ValidationError{
			Field:   "log_level",
			Value:   cfg.LogLevel,
			Message: "must be one of: debug, info, warn, error",
		})
	}

	// BaselineChecks[].Name must not be empty
	// BaselineChecks[].Command must not be empty
	for i, check := range cfg.BaselineChecks {
		if check.Name == "" {
			errs = append(errs, &ValidationError{
				Field:   fmt.Sprintf("baseline_checks[%d].name", i),
				Value:   check.Name,
				Message: "must not be empty",
			})
		}
		if check.Command == "" {
			errs = append(errs, &ValidationError{
				Field:   fmt.Sprintf("baseline_checks[%d].command", i),
				Value:   check.Command,
				Message: "must not be empty",
			})
		}
	}

	// Worktree.SetupCommands[].Command must not be empty
	for i, cmd := range cfg.Worktree.SetupCommands {
		if cmd.Command == "" {
			errs = append(errs, &ValidationError{
				Field:   fmt.Sprintf("worktree.setup[%d].command", i),
				Value:   cmd.Command,
				Message: "must not be empty",
			})
		}
	}

	// Worktree.TeardownCommands[].Command must not be empty
	for i, cmd := range cfg.Worktree.TeardownCommands {
		if cmd.Command == "" {
			errs = append(errs, &ValidationError{
				Field:   fmt.Sprintf("worktree.teardown[%d].command", i),
				Value:   cmd.Command,
				Message: "must not be empty",
			})
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
