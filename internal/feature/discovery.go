package feature

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsePRD reads a PRD file and parses its frontmatter and body
func ParsePRD(filePath string) (*PRD, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open PRD file: %w", err)
	}
	defer f.Close()

	return ParsePRDFromReader(f, filePath)
}

// ParsePRDFromReader parses a PRD from an io.Reader (for testing)
func ParsePRDFromReader(r io.Reader, filePath string) (*PRD, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read PRD: %w", err)
	}

	frontmatter, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	var prd PRD
	if err := yaml.Unmarshal(frontmatter, &prd); err != nil {
		return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
	}

	prd.FilePath = filePath
	prd.Body = string(body)
	prd.BodyHash = ComputeBodyHash(prd.Body)

	return &prd, nil
}

// ComputeBodyHash returns SHA-256 hash of the PRD body content
func ComputeBodyHash(body string) string {
	h := sha256.New()
	h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

// DiscoverPRDs finds all PRD files in the given directory recursively
// Skips README.md files and logs warnings for invalid PRDs
func DiscoverPRDs(baseDir string) ([]*PRD, error) {
	var prds []*PRD

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		// Skip README files
		if strings.ToLower(d.Name()) == "readme.md" {
			return nil
		}

		prd, err := ParsePRD(path)
		if err != nil {
			// Log warning but continue discovery
			slog.Warn("failed to parse PRD", "path", path, "error", err)
			return nil
		}

		if err := ValidatePRD(prd); err != nil {
			slog.Warn("invalid PRD", "path", path, "error", err)
			return nil
		}

		prds = append(prds, prd)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	return prds, nil
}

// DiscoverPRDsWithFilter finds PRDs matching the given status filter
// Empty filter returns all PRDs
func DiscoverPRDsWithFilter(baseDir string, statusFilter []string) ([]*PRD, error) {
	prds, err := DiscoverPRDs(baseDir)
	if err != nil {
		return nil, err
	}

	if len(statusFilter) == 0 {
		return prds, nil
	}

	filterSet := make(map[string]bool)
	for _, s := range statusFilter {
		filterSet[s] = true
	}

	var filtered []*PRD
	for _, prd := range prds {
		if filterSet[prd.Status] {
			filtered = append(filtered, prd)
		}
	}

	return filtered, nil
}
