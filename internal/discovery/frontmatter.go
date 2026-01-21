package discovery

import (
	"bufio"
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// UnitFrontmatter represents the YAML frontmatter in IMPLEMENTATION_PLAN.md
type UnitFrontmatter struct {
	// Required fields
	Unit string `yaml:"unit"`

	// Optional dependency field
	DependsOn []string `yaml:"depends_on"`

	// Provider overrides the default provider for this unit's task execution
	// Valid values: "claude", "codex"
	// Empty means use the resolved default from CLI/env/config
	Provider string `yaml:"provider,omitempty"`

	// Orchestrator-managed fields (may not be present initially)
	OrchStatus      string `yaml:"orch_status"`
	OrchBranch      string `yaml:"orch_branch"`
	OrchWorktree    string `yaml:"orch_worktree"`
	OrchPRNumber    int    `yaml:"orch_pr_number"`
	OrchStartedAt   string `yaml:"orch_started_at"`
	OrchCompletedAt string `yaml:"orch_completed_at"`
}

// TaskFrontmatter represents the YAML frontmatter in task files
type TaskFrontmatter struct {
	// Required fields
	Task         int    `yaml:"task"`
	Status       string `yaml:"status"`
	Backpressure string `yaml:"backpressure"`

	// Optional dependency field
	DependsOn []int `yaml:"depends_on"`
}

// ParseFrontmatter extracts YAML frontmatter from markdown content
// Returns the frontmatter string and the remaining content
// Frontmatter is delimited by --- on its own line at start and end
func ParseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error) {
	// Frontmatter must start at the very beginning
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return nil, content, nil
	}

	// Find the closing delimiter
	// Start searching after the opening "---\n"
	remaining := content[4:]
	closingIdx := bytes.Index(remaining, []byte("\n---\n"))
	if closingIdx == -1 {
		return nil, nil, fmt.Errorf("unclosed frontmatter: missing closing '---'")
	}

	// Extract frontmatter (between the delimiters, excluding the delimiters themselves)
	frontmatter = remaining[:closingIdx]

	// Extract body (everything after the closing "---\n")
	bodyStart := 4 + closingIdx + 5 // len("---\n") + closingIdx + len("\n---\n")
	if bodyStart < len(content) {
		body = content[bodyStart:]
	}

	return frontmatter, body, nil
}

// ParseUnitFrontmatter parses IMPLEMENTATION_PLAN.md frontmatter
func ParseUnitFrontmatter(data []byte) (*UnitFrontmatter, error) {
	var uf UnitFrontmatter
	if err := yaml.Unmarshal(data, &uf); err != nil {
		return nil, fmt.Errorf("failed to parse unit frontmatter: %w", err)
	}
	return &uf, nil
}

// ParseTaskFrontmatter parses task file frontmatter
func ParseTaskFrontmatter(data []byte) (*TaskFrontmatter, error) {
	var tf TaskFrontmatter
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse task frontmatter: %w", err)
	}
	return &tf, nil
}

// extractTitle extracts the first H1 heading from markdown body
func extractTitle(body []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			return line[2:]
		}
	}
	return ""
}
