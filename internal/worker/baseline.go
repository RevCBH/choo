package worker

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// BaselineCheckResult holds results for a single check
type BaselineCheckResult struct {
	Check  BaselineCheck
	Passed bool
	Output string
}

// RunBaselineChecks executes all baseline checks for the unit
// Returns (allPassed, combinedFailureOutput)
func RunBaselineChecks(ctx context.Context, checks []BaselineCheck, workdir string, timeout time.Duration) (bool, string) {
	// Handle empty checks
	if len(checks) == 0 {
		return true, ""
	}

	// Create timeout context for entire baseline check phase
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Iterate through checks and collect failures
	var failures []string
	allPassed := true

	for _, check := range checks {
		result := RunSingleBaselineCheck(timeoutCtx, check, workdir)
		if !result.Passed {
			allPassed = false
			// Format failure with check name header
			failures = append(failures, "=== "+check.Name+" ===\n"+result.Output)
		}
	}

	// Join multiple failures with double newlines for readability
	combinedOutput := strings.Join(failures, "\n\n")

	return allPassed, combinedOutput
}

// RunSingleBaselineCheck executes one baseline check and returns the result
func RunSingleBaselineCheck(ctx context.Context, check BaselineCheck, workdir string) BaselineCheckResult {
	// Execute check.Command via sh -c
	cmd := exec.CommandContext(ctx, "sh", "-c", check.Command)
	cmd.Dir = workdir

	// Capture both stdout and stderr
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run the command
	err := cmd.Run()

	// Return structured result
	return BaselineCheckResult{
		Check:  check,
		Passed: err == nil,
		Output: output.String(),
	}
}
