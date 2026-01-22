package skills

import (
	"fmt"
	"os"
	"strings"
)

// PromptBuilder constructs prompts for spec workflow phases.
type PromptBuilder struct {
	skills map[SkillName]*Skill
}

// NewPromptBuilder creates a new prompt builder with loaded skills.
func NewPromptBuilder() (*PromptBuilder, error) {
	return NewPromptBuilderWithDir("")
}

// NewPromptBuilderWithDir creates a new prompt builder with a custom skills directory.
func NewPromptBuilderWithDir(skillsDir string) (*PromptBuilder, error) {
	skills, err := LoadAllWithDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}
	return &PromptBuilder{skills: skills}, nil
}

// BuildSpecGenerationPrompt builds the prompt for spec generation phase.
// It embeds the spec skill along with the PRD content and context about existing specs.
func (pb *PromptBuilder) BuildSpecGenerationPrompt(prdContent string, specsDir string, existingSpecs []string) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Task: Generate Technical Specifications from PRD\n\n")

	// Embed the spec skill
	specSkill := pb.skills[SkillSpec]
	sb.WriteString("## Skill Instructions\n\n")
	sb.WriteString(specSkill.Content)
	sb.WriteString("\n\n")

	// PRD content
	sb.WriteString("---\n\n")
	sb.WriteString("## PRD (Product Requirements Document)\n\n")
	sb.WriteString(prdContent)
	sb.WriteString("\n\n")

	// Context about existing specs
	if len(existingSpecs) > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString("## Existing Specs\n\n")
		sb.WriteString("The following specs already exist in the repository:\n\n")
		for _, spec := range existingSpecs {
			sb.WriteString(fmt.Sprintf("- `%s`\n", spec))
		}
		sb.WriteString("\n")
	}

	// Output directory instruction
	sb.WriteString("---\n\n")
	sb.WriteString("## Output Directory\n\n")
	sb.WriteString(fmt.Sprintf("Write all generated specs to: `%s/`\n\n", specsDir))

	// Final instruction
	sb.WriteString("---\n\n")
	sb.WriteString("## Your Task\n\n")
	sb.WriteString("1. Analyze the PRD and identify the components that need specs\n")
	sb.WriteString("2. Create technical specifications following the skill instructions\n")
	sb.WriteString("3. Write each spec to the output directory\n")
	sb.WriteString("4. Update specs/README.md with the new specs table and dependency graph\n")

	return sb.String()
}

// BuildValidationPrompt builds the prompt for spec validation phase.
// It embeds the spec-validate skill and points to the specs directory.
func (pb *PromptBuilder) BuildValidationPrompt(specsDir string, prdPath string) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Task: Validate Generated Specifications\n\n")

	// Embed the spec-validate skill
	validateSkill := pb.skills[SkillSpecValidate]
	sb.WriteString("## Skill Instructions\n\n")
	sb.WriteString(validateSkill.Content)
	sb.WriteString("\n\n")

	// Context
	sb.WriteString("---\n\n")
	sb.WriteString("## Context\n\n")
	sb.WriteString(fmt.Sprintf("- **PRD**: `%s`\n", prdPath))
	sb.WriteString(fmt.Sprintf("- **Specs Directory**: `%s/`\n\n", specsDir))

	// Instructions
	sb.WriteString("---\n\n")
	sb.WriteString("## Your Task\n\n")
	sb.WriteString("1. Read all specs in the specs directory\n")
	sb.WriteString("2. Validate type consistency, interface alignment, and dependencies\n")
	sb.WriteString("3. Apply auto-fixes where possible (mode: fix)\n")
	sb.WriteString("4. Report any issues that require manual resolution\n")
	sb.WriteString("5. Output a validation report summarizing findings\n")

	return sb.String()
}

// BuildRalphPrepPrompt builds the prompt for ralph-prep phase (task generation).
// It embeds the ralph-prep skill and points to the specs directory.
func (pb *PromptBuilder) BuildRalphPrepPrompt(specsDir string) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Task: Generate Ralph-Executable Task Specs\n\n")

	// Embed the ralph-prep skill
	ralphPrepSkill := pb.skills[SkillRalphPrep]
	sb.WriteString("## Skill Instructions\n\n")
	sb.WriteString(ralphPrepSkill.Content)
	sb.WriteString("\n\n")

	// Context
	sb.WriteString("---\n\n")
	sb.WriteString("## Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Specs Directory**: `%s/`\n", specsDir))
	sb.WriteString(fmt.Sprintf("- **Tasks Output**: `%s/tasks/`\n\n", specsDir))

	// Instructions
	sb.WriteString("---\n\n")
	sb.WriteString("## Your Task\n\n")
	sb.WriteString("1. Read each spec in the specs directory\n")
	sb.WriteString("2. For each spec, decompose into atomic, Ralph-executable task specs\n")
	sb.WriteString("3. Create implementation plan (IMPLEMENTATION_PLAN.md) for each unit\n")
	sb.WriteString("4. Write atomic task specs with proper frontmatter, backpressure, and dependencies\n")
	sb.WriteString("5. Ensure all tasks have explicit, executable backpressure commands\n")

	return sb.String()
}

// GetExistingSpecs scans a directory for existing spec files.
// Only returns top-level specs; subdirectories (completed/, tasks/, etc.) are ignored.
func GetExistingSpecs(specsDir string) ([]string, error) {
	var specs []string

	// Read only top-level directory entries (non-recursive)
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return specs, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		// Skip directories entirely
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".md") && name != "README.md" {
			specs = append(specs, name)
		}
	}

	return specs, nil
}

// SkillSources returns a map of skill names to their sources for debugging.
func (pb *PromptBuilder) SkillSources() map[SkillName]Source {
	sources := make(map[SkillName]Source)
	for name, skill := range pb.skills {
		sources[name] = skill.Source
	}
	return sources
}
