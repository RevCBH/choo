package feature

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PRDFrontmatter represents optional YAML frontmatter in PRDs
type PRDFrontmatter struct {
	Title     string   `yaml:"title"`
	DependsOn []string `yaml:"depends_on"`
	Status    string   `yaml:"status"`   // draft, ready, in_progress, complete
	Priority  string   `yaml:"priority"` // hint: high, medium, low
}

// PRDForPrioritization represents a PRD loaded for prioritization
type PRDForPrioritization struct {
	ID        string   // filename without extension
	Path      string   // absolute path to file
	Title     string   // extracted from first H1 or frontmatter
	Content   string   // full markdown content
	DependsOn []string // from frontmatter (optional hints)
}

// LoadPRDs reads all PRD files from the given directory
func LoadPRDs(prdDir string) ([]*PRDForPrioritization, error) {
	// Find all markdown files
	pattern := filepath.Join(prdDir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for PRD files: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no markdown files found in %s", prdDir)
	}

	var prds []*PRDForPrioritization

	for _, path := range matches {
		// Skip README.md files (common in PRD directories)
		basename := filepath.Base(path)
		if strings.EqualFold(basename, "README.md") {
			continue
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			// Log warning and continue with other PRDs
			fmt.Fprintf(os.Stderr, "Warning: failed to read PRD %s: %v\n", path, err)
			continue
		}

		// Parse frontmatter if present
		frontmatter, err := ParsePRDFrontmatter(content)
		if err != nil {
			// Log warning and skip this PRD
			fmt.Fprintf(os.Stderr, "Warning: malformed frontmatter in %s: %v\n", path, err)
			continue
		}

		// Extract title
		title := ExtractPRDTitle(content)
		if title == "" {
			// Fallback to filename if no H1 heading found
			title = strings.TrimSuffix(basename, ".md")
		}

		// Override title if present in frontmatter
		if frontmatter != nil && frontmatter.Title != "" {
			title = frontmatter.Title
		}

		// Extract depends_on from frontmatter
		var dependsOn []string
		if frontmatter != nil {
			dependsOn = frontmatter.DependsOn
		}

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		prd := &PRDForPrioritization{
			ID:        strings.TrimSuffix(basename, filepath.Ext(basename)),
			Path:      absPath,
			Title:     title,
			Content:   string(content),
			DependsOn: dependsOn,
		}

		prds = append(prds, prd)
	}

	if len(prds) == 0 {
		return nil, fmt.Errorf("no valid PRD files found in %s (found %d markdown files but all were skipped)", prdDir, len(matches))
	}

	return prds, nil
}

// ParsePRDFrontmatter extracts optional frontmatter from PRD content
func ParsePRDFrontmatter(content []byte) (*PRDFrontmatter, error) {
	// Check if content starts with frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, nil // No frontmatter present (not an error)
	}

	// Find the closing delimiter
	var start int
	if bytes.HasPrefix(content, []byte("---\n")) {
		start = 4
	} else {
		start = 5
	}

	// Look for closing ---
	// Handle case where frontmatter is empty (---\n---\n)
	if bytes.HasPrefix(content[start:], []byte("---\n")) || bytes.HasPrefix(content[start:], []byte("---\r\n")) {
		return nil, nil // Empty frontmatter is valid
	}

	end := bytes.Index(content[start:], []byte("\n---\n"))
	if end == -1 {
		end = bytes.Index(content[start:], []byte("\n---\r\n"))
		if end == -1 {
			// No closing delimiter found
			return nil, fmt.Errorf("unterminated frontmatter: missing closing ---")
		}
	}

	// Extract frontmatter content
	frontmatterContent := content[start : start+end]

	// Handle empty frontmatter
	if len(bytes.TrimSpace(frontmatterContent)) == 0 {
		return nil, nil // Empty frontmatter is valid
	}

	// Parse YAML
	var fm PRDFrontmatter
	if err := yaml.Unmarshal(frontmatterContent, &fm); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return &fm, nil
}

// ExtractPRDTitle extracts the first H1 heading as title
func ExtractPRDTitle(content []byte) string {
	lines := bytes.Split(content, []byte("\n"))

	inFrontmatter := false
	frontmatterClosed := false

	for _, line := range lines {
		lineStr := string(bytes.TrimSpace(line))

		// Track frontmatter boundaries
		if lineStr == "---" {
			if !inFrontmatter && !frontmatterClosed {
				// Start of frontmatter
				inFrontmatter = true
				continue
			} else if inFrontmatter {
				// End of frontmatter
				inFrontmatter = false
				frontmatterClosed = true
				continue
			}
		}

		// Skip lines inside frontmatter
		if inFrontmatter {
			continue
		}

		// Look for H1 heading (starts with # followed by space)
		if strings.HasPrefix(lineStr, "# ") {
			title := strings.TrimSpace(lineStr[2:])
			return title
		}
	}

	return "" // No H1 heading found
}

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
	// Construct path: baseDir/<prd-id>.md
	path := s.prdPath(prdID)

	// Read entire file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read PRD file: %w", err)
	}

	// Split frontmatter from body
	frontmatterBytes, bodyBytes, err := parseFrontmatter(content)
	if err != nil {
		return nil, "", err
	}

	// Parse frontmatter as YAML into PRDMetadata
	var metadata PRDMetadata
	if err := yaml.Unmarshal(frontmatterBytes, &metadata); err != nil {
		return nil, "", fmt.Errorf("failed to parse frontmatter YAML: %w", err)
	}

	return &metadata, string(bodyBytes), nil
}

// UpdateStatus atomically updates only the feature_status field
func (s *PRDStore) UpdateStatus(prdID string, status FeatureStatus) error {
	// Load current file
	metadata, body, err := s.Load(prdID)
	if err != nil {
		return err
	}

	// Update only the status field
	metadata.FeatureStatus = status

	// Serialize frontmatter with YAML
	frontmatterStr, err := serializeFrontmatter(metadata)
	if err != nil {
		return err
	}

	// Write complete file (frontmatter + body)
	fullContent := frontmatterStr + body

	// Write back atomically (write-to-temp + rename)
	path := s.prdPath(prdID)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// UpdateState atomically updates full feature state in frontmatter
func (s *PRDStore) UpdateState(prdID string, state FeatureState) error {
	// Load current file (preserving extra fields)
	metadata, body, err := s.Load(prdID)
	if err != nil {
		return err
	}

	// Merge state fields into metadata
	metadata.FeatureStatus = state.Status
	metadata.Branch = state.Branch
	metadata.StartedAt = &state.StartedAt
	metadata.ReviewIterations = state.ReviewIterations
	metadata.MaxReviewIter = state.MaxReviewIter
	metadata.LastFeedback = state.LastFeedback
	metadata.SpecCount = state.SpecCount
	metadata.TaskCount = state.TaskCount

	// Serialize frontmatter with YAML
	frontmatterStr, err := serializeFrontmatter(metadata)
	if err != nil {
		return err
	}

	// Write complete file (frontmatter + body)
	fullContent := frontmatterStr + body

	// Write back atomically (write-to-temp + rename)
	path := s.prdPath(prdID)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
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

// serializeFrontmatter converts metadata back to YAML with --- markers
func serializeFrontmatter(meta *PRDMetadata) (string, error) {
	// Marshal metadata to YAML
	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata to YAML: %w", err)
	}

	// Wrap with --- markers
	return "---\n" + string(yamlBytes) + "---\n", nil
}
