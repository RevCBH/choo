package feature

import (
	"bytes"
	"fmt"
)

// parseFrontmatter splits content into frontmatter YAML and body markdown
// Content must start with "---\n" followed by YAML, then "\n---\n" delimiter
// Returns the frontmatter bytes (without delimiters) and body bytes
func parseFrontmatter(content []byte) (frontmatter []byte, body []byte, err error) {
	// Content must start with "---\n"
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return nil, nil, fmt.Errorf("missing frontmatter delimiter")
	}

	// Find closing delimiter
	rest := content[4:] // Skip opening "---\n"
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx == -1 {
		// Try "---" at end of file (no trailing newline)
		idx = bytes.Index(rest, []byte("\n---"))
		if idx == -1 || idx+4 != len(rest) {
			return nil, nil, fmt.Errorf("missing closing frontmatter delimiter")
		}
		frontmatter = rest[:idx]
		body = nil
		return frontmatter, body, nil
	}

	frontmatter = rest[:idx]
	body = rest[idx+5:] // Skip "\n---\n"

	return frontmatter, body, nil
}
