package feature

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PRDStore handles PRD file operations
type PRDStore struct {
	baseDir string
}

// PRDMetadata represents parsed PRD frontmatter
type PRDMetadata struct {
	Title            string                 `yaml:"title"`
	FeatureStatus    FeatureStatus          `yaml:"feature_status,omitempty"`
	Branch           string                 `yaml:"branch,omitempty"`
	StartedAt        *time.Time             `yaml:"started_at,omitempty"`
	ReviewIterations int                    `yaml:"review_iterations,omitempty"`
	MaxReviewIter    int                    `yaml:"max_review_iter,omitempty"`
	LastFeedback     string                 `yaml:"last_feedback,omitempty"`
	SpecCount        int                    `yaml:"spec_count,omitempty"`
	TaskCount        int                    `yaml:"task_count,omitempty"`
	Extra            map[string]interface{} `yaml:",inline"`
}

// NewPRDStore creates a PRD store for the given directory
func NewPRDStore(baseDir string) *PRDStore {
	return &PRDStore{
		baseDir: baseDir,
	}
}

// Load reads and parses a PRD file, returning metadata and body separately
func (s *PRDStore) Load(prdID string) (*PRDMetadata, string, error) {
	path := s.prdPath(prdID)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read PRD file: %w", err)
	}

	frontmatter, body, err := parseFrontmatter(string(content))
	if err != nil {
		return nil, "", err
	}

	var meta PRDMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, "", fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return &meta, body, nil
}

// UpdateStatus atomically updates only the feature_status field
func (s *PRDStore) UpdateStatus(prdID string, status FeatureStatus) error {
	meta, body, err := s.Load(prdID)
	if err != nil {
		return err
	}

	meta.FeatureStatus = status

	return s.writeFile(prdID, meta, body)
}

// UpdateState atomically updates full feature state in frontmatter
func (s *PRDStore) UpdateState(prdID string, state FeatureState) error {
	meta, body, err := s.Load(prdID)
	if err != nil {
		return err
	}

	// Merge state fields into metadata
	meta.FeatureStatus = state.Status
	meta.Branch = state.Branch
	meta.StartedAt = &state.StartedAt
	meta.ReviewIterations = state.ReviewIterations
	meta.MaxReviewIter = state.MaxReviewIter
	meta.LastFeedback = state.LastFeedback
	meta.SpecCount = state.SpecCount
	meta.TaskCount = state.TaskCount

	return s.writeFile(prdID, meta, body)
}

// Exists checks if a PRD file exists
func (s *PRDStore) Exists(prdID string) bool {
	path := s.prdPath(prdID)
	_, err := os.Stat(path)
	return err == nil
}

// prdPath returns the full path for a PRD ID
func (s *PRDStore) prdPath(prdID string) string {
	return filepath.Join(s.baseDir, prdID+".md")
}

// writeFile atomically writes PRD file with frontmatter and body
func (s *PRDStore) writeFile(prdID string, meta *PRDMetadata, body string) error {
	frontmatter, err := serializeFrontmatter(meta)
	if err != nil {
		return err
	}

	content := frontmatter + body
	path := s.prdPath(prdID)

	// Write to temp file first for atomicity
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// parseFrontmatter extracts YAML frontmatter from file content
func parseFrontmatter(content string) (frontmatter string, body string, err error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("PRD file missing frontmatter: must start with '---'")
	}

	// Skip opening ---
	rest := content[4:]

	// Find closing ---
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		return "", "", fmt.Errorf("PRD file missing closing frontmatter marker: '---'")
	}

	frontmatter = rest[:endIdx]
	body = rest[endIdx+5:] // Skip \n---\n

	return frontmatter, body, nil
}

// serializeFrontmatter converts metadata back to YAML with --- markers
func serializeFrontmatter(meta *PRDMetadata) (string, error) {
	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	return "---\n" + string(yamlBytes) + "---\n", nil
}
