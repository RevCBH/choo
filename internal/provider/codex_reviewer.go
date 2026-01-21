package provider

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// issuePattern matches common issue output formats:
// file.go:10: error: message
// file.go:10:5: warning: message
// path/to/file.go:42: suggestion: use constants
// test.go:1: info: consider renaming
var issuePattern = regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?:\s*(\w+):\s*(.+)$`)

// CodexReviewer implements Reviewer using Codex CLI.
type CodexReviewer struct {
	// command is the path to codex CLI, empty for system PATH
	command string
}

// NewCodexReviewer creates a CodexReviewer with optional command override.
// If command is empty, defaults to "codex" resolved via PATH.
func NewCodexReviewer(command string) *CodexReviewer {
	return &CodexReviewer{command: command}
}

// Name returns ProviderCodex to identify this reviewer.
func (r *CodexReviewer) Name() ProviderType {
	return ProviderCodex
}

// Review executes codex review and returns structured results.
// The review compares changes in workdir against baseBranch.
//
// Exit code handling:
// - 0: No issues found (Passed = true)
// - 1: Issues found (Passed = false, parse output for issues)
// - 2+: Execution error (return error)
func (r *CodexReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
	cmdPath := r.command
	if cmdPath == "" {
		cmdPath = "codex"
	}

	// Invoke: codex review --base <baseBranch>
	cmd := exec.CommandContext(ctx, cmdPath, "review", "--base", baseBranch)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			// Exit code 1 = issues found (not an error)
			if exitCode == 1 {
				return r.parseOutput(string(output), exitCode)
			}
			// Exit code 2+ = actual execution error
			return nil, fmt.Errorf("codex review error (exit %d): %s",
				exitCode, string(output))
		}
		// Command not found or other exec error
		return nil, fmt.Errorf("codex review failed: %w", err)
	}

	return r.parseOutput(string(output), 0)
}

// parseOutput converts codex output into structured ReviewResult.
func (r *CodexReviewer) parseOutput(output string, exitCode int) (*ReviewResult, error) {
	result := &ReviewResult{
		RawOutput: output,
		Passed:    exitCode == 0,
		Issues:    []ReviewIssue{},
	}

	// Parse codex review output format
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if issue := r.parseLine(line); issue != nil {
			result.Issues = append(result.Issues, *issue)
		}
	}

	if len(result.Issues) > 0 {
		result.Passed = false
		result.Summary = fmt.Sprintf("Found %d issues", len(result.Issues))
	} else if exitCode == 0 {
		result.Summary = "No issues found"
	} else {
		result.Summary = fmt.Sprintf("Review completed with exit code %d", exitCode)
	}

	return result, nil
}

// parseLine attempts to parse a single line as an issue.
// Returns nil if the line is not an issue.
func (r *CodexReviewer) parseLine(line string) *ReviewIssue {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	matches := issuePattern.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	lineNum, _ := strconv.Atoi(matches[2])

	return &ReviewIssue{
		File:     matches[1],
		Line:     lineNum,
		Severity: strings.ToLower(matches[3]),
		Message:  matches[4],
	}
}

// Compile-time check that CodexReviewer implements Reviewer interface
var _ Reviewer = (*CodexReviewer)(nil)
