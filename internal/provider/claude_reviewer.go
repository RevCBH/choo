package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeReviewer implements Reviewer using Claude with diff-based prompts.
// It retrieves the git diff, builds a prompt requesting JSON output,
// and parses the structured response into ReviewIssue types.
type ClaudeReviewer struct {
	command string // Path to claude CLI executable
}

// NewClaudeReviewer creates a ClaudeReviewer with optional command override.
// If command is empty, defaults to "claude" (resolved via PATH).
func NewClaudeReviewer(command string) *ClaudeReviewer {
	if command == "" {
		command = "claude"
	}
	return &ClaudeReviewer{command: command}
}

// Name returns ProviderClaude to identify this reviewer.
func (r *ClaudeReviewer) Name() ProviderType {
	return ProviderClaude
}

// Review performs code review by getting the diff, invoking Claude,
// and parsing the structured JSON response.
func (r *ClaudeReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
	// Get diff for review
	diff, err := r.getDiff(ctx, workdir, baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	if diff == "" {
		return &ReviewResult{
			Passed:  true,
			Summary: "No changes to review",
		}, nil
	}

	// Build review prompt requesting JSON output
	prompt := BuildClaudeReviewPrompt(diff)

	// Invoke Claude with non-interactive flags to prevent hangs:
	// --dangerously-skip-permissions: bypass interactive permission prompts
	// --print: output to stdout instead of interactive mode
	// -p: provide the prompt
	cmd := exec.CommandContext(ctx, r.command,
		"--dangerously-skip-permissions",
		"--print",
		"-p", prompt,
	)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude review failed: %w", err)
	}

	return r.parseOutput(string(output))
}

// getDiff retrieves the git diff between baseBranch and HEAD.
func (r *ClaudeReviewer) getDiff(ctx context.Context, workdir, baseBranch string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", baseBranch+"...HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// parseOutput extracts and parses JSON from Claude's response.
// Returns an error if parsing fails - the caller handles this gracefully
// by emitting CodeReviewFailed and proceeding to merge.
func (r *ClaudeReviewer) parseOutput(output string) (*ReviewResult, error) {
	// Extract JSON from Claude's response
	jsonStr := extractJSON(output)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in review output")
	}

	var parsed struct {
		Passed  bool          `json:"passed"`
		Summary string        `json:"summary"`
		Issues  []ReviewIssue `json:"issues"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse review JSON: %w", err)
	}

	return &ReviewResult{
		Passed:    parsed.Passed,
		Summary:   parsed.Summary,
		Issues:    parsed.Issues,
		RawOutput: output,
	}, nil
}

// extractJSON finds and returns the first JSON object in the output.
// Handles JSON in markdown code fences and bare JSON.
// Returns empty string if no valid JSON found.
func extractJSON(output string) string {
	// First, try to extract JSON from markdown code fence
	if jsonStr := extractJSONFromCodeFence(output); jsonStr != "" {
		return jsonStr
	}

	// Fall back to finding bare JSON by brace matching
	return extractJSONByBraces(output)
}

// extractJSONFromCodeFence extracts JSON from markdown code fences.
// Looks for ```json or ``` followed by JSON content.
func extractJSONFromCodeFence(output string) string {
	markers := []string{"```json\n", "```\n"}
	for _, marker := range markers {
		start := strings.Index(output, marker)
		if start == -1 {
			continue
		}
		contentStart := start + len(marker)
		// Find the closing ```
		end := strings.Index(output[contentStart:], "```")
		if end == -1 {
			continue
		}
		content := strings.TrimSpace(output[contentStart : contentStart+end])
		if strings.HasPrefix(content, "{") {
			return content
		}
	}
	return ""
}

// extractJSONByBraces finds JSON by matching braces.
// Scans for first { and tracks depth until matching } found.
func extractJSONByBraces(output string) string {
	start := -1
	depth := 0

	for i, ch := range output {
		if ch == '{' {
			if start == -1 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && start != -1 {
				return output[start : i+1]
			}
		}
	}

	return ""
}
