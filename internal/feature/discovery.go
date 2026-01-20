package feature

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

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
