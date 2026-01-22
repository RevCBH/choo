package skills

import (
	"fmt"
	"os"
	"path/filepath"
)

// SkillName represents a known skill identifier.
type SkillName string

const (
	// SkillSpec is the technical specification writing skill.
	SkillSpec SkillName = "spec"
	// SkillSpecValidate is the specification validation skill.
	SkillSpecValidate SkillName = "spec-validate"
	// SkillRalphPrep is the ralph preparation (task decomposition) skill.
	SkillRalphPrep SkillName = "ralph-prep"
)

// Source indicates where a skill was loaded from.
type Source string

const (
	// SourceUser indicates the skill was loaded from user overrides (~/.choo/skills/).
	SourceUser Source = "user"
	// SourceEmbedded indicates the skill was loaded from embedded defaults.
	SourceEmbedded Source = "embedded"
)

// Skill represents a loaded skill with its content and metadata.
type Skill struct {
	// Name is the skill identifier.
	Name SkillName
	// Content is the raw markdown content of the skill.
	Content string
	// Source indicates where the skill was loaded from.
	Source Source
}

// Load loads a skill by name, checking user overrides first then falling back to embedded.
//
// User overrides are checked at ~/.choo/skills/{name}.md
// If not found, falls back to embedded skills from the binary.
func Load(name SkillName) (*Skill, error) {
	return LoadWithDir(name, "")
}

// LoadWithDir loads a skill with a custom skills directory for user overrides.
// If skillsDir is empty, it defaults to ~/.choo/skills/.
func LoadWithDir(name SkillName, skillsDir string) (*Skill, error) {
	// Determine user skills directory
	if skillsDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fall through to embedded if we can't get home dir
			return loadEmbedded(name)
		}
		skillsDir = filepath.Join(homeDir, ".choo", "skills")
	}

	// Expand ~ in path if present
	if len(skillsDir) > 0 && skillsDir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			skillsDir = filepath.Join(homeDir, skillsDir[1:])
		}
	}

	// Check for user override
	userPath := filepath.Join(skillsDir, string(name)+".md")
	if content, err := os.ReadFile(userPath); err == nil {
		return &Skill{
			Name:    name,
			Content: string(content),
			Source:  SourceUser,
		}, nil
	}

	// Fall back to embedded
	return loadEmbedded(name)
}

// loadEmbedded loads a skill from embedded defaults.
func loadEmbedded(name SkillName) (*Skill, error) {
	content, err := skillsFS.ReadFile("skills/" + string(name) + ".md")
	if err != nil {
		return nil, fmt.Errorf("skill %s not found: %w", name, err)
	}
	return &Skill{
		Name:    name,
		Content: string(content),
		Source:  SourceEmbedded,
	}, nil
}

// LoadAll loads all known skills, returning a map of skill name to skill.
func LoadAll() (map[SkillName]*Skill, error) {
	return LoadAllWithDir("")
}

// LoadAllWithDir loads all known skills with a custom skills directory.
func LoadAllWithDir(skillsDir string) (map[SkillName]*Skill, error) {
	skills := make(map[SkillName]*Skill)

	names := []SkillName{SkillSpec, SkillSpecValidate, SkillRalphPrep}
	for _, name := range names {
		skill, err := LoadWithDir(name, skillsDir)
		if err != nil {
			return nil, err
		}
		skills[name] = skill
	}

	return skills, nil
}

// MustLoad loads a skill and panics if it fails.
// Use only when skill must exist (e.g., embedded skills during init).
func MustLoad(name SkillName) *Skill {
	skill, err := Load(name)
	if err != nil {
		panic(fmt.Sprintf("failed to load required skill %s: %v", name, err))
	}
	return skill
}
