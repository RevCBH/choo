package discovery

import (
	"bytes"
	"fmt"
	"strings"
)

// MetadataSource identifies where metadata was parsed from.
type MetadataSource string

const (
	MetadataSourceFrontmatter   MetadataSource = "frontmatter"
	MetadataSourceMetadataBlock MetadataSource = "metadata_block"
	MetadataSourceNone          MetadataSource = "none"
)

// MetadataBlock represents a parsed ## Metadata section with a fenced YAML block.
type MetadataBlock struct {
	YAML           []byte
	HeaderLine     int
	FenceStartLine int
	FenceEndLine   int
}

// FindMetadataBlock locates a ## Metadata section followed by a fenced yaml/yml block.
// Returns nil, nil if no metadata block is found.
func FindMetadataBlock(content []byte) (*MetadataBlock, error) {
	lines := strings.Split(string(content), "\n")

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "## Metadata" {
			continue
		}

		headerLine := i
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) {
			return nil, fmt.Errorf("metadata block missing fenced yaml block")
		}

		fenceLine := strings.TrimSpace(lines[j])
		if fenceLine != "```yaml" && fenceLine != "```yml" {
			return nil, fmt.Errorf("metadata block missing fenced yaml block")
		}
		fenceStart := j

		k := j + 1
		for k < len(lines) && strings.TrimSpace(lines[k]) != "```" {
			k++
		}
		if k >= len(lines) {
			return nil, fmt.Errorf("metadata block missing closing fence")
		}
		fenceEnd := k

		yamlContent := strings.Join(lines[fenceStart+1:fenceEnd], "\n")
		return &MetadataBlock{
			YAML:           []byte(yamlContent),
			HeaderLine:     headerLine,
			FenceStartLine: fenceStart,
			FenceEndLine:   fenceEnd,
		}, nil
	}

	return nil, nil
}

// RemoveMetadataBlock removes the metadata block and surrounding blank lines.
func RemoveMetadataBlock(content []byte, block *MetadataBlock) []byte {
	if block == nil {
		return content
	}

	lines := strings.Split(string(content), "\n")
	start := block.HeaderLine
	end := block.FenceEndLine

	for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	for end+1 < len(lines) && strings.TrimSpace(lines[end+1]) == "" {
		end++
	}

	filtered := append([]string{}, lines[:start]...)
	filtered = append(filtered, lines[end+1:]...)

	return []byte(strings.Join(filtered, "\n"))
}

// ParseMetadataBlock extracts YAML from a metadata block and returns the body without the block.
func ParseMetadataBlock(content []byte) (metadata []byte, body []byte, found bool, err error) {
	block, err := FindMetadataBlock(content)
	if err != nil {
		return nil, nil, false, err
	}
	if block == nil {
		return nil, content, false, nil
	}

	body = RemoveMetadataBlock(content, block)
	return block.YAML, body, true, nil
}

// ParseUnitMetadata parses unit metadata from frontmatter or metadata block.
func ParseUnitMetadata(content []byte) (*UnitFrontmatter, []byte, MetadataSource, error) {
	if bytes.HasPrefix(content, []byte("---\n")) {
		frontmatter, body, err := ParseFrontmatter(content)
		if err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		unitFrontmatter, err := ParseUnitFrontmatter(frontmatter)
		if err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		if err := validateUnitFrontmatter(unitFrontmatter); err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		return unitFrontmatter, body, MetadataSourceFrontmatter, nil
	}

	metadata, body, found, err := ParseMetadataBlock(content)
	if err != nil {
		return nil, nil, MetadataSourceNone, err
	}
	if !found {
		return nil, content, MetadataSourceNone, fmt.Errorf("missing metadata")
	}

	unitFrontmatter, err := ParseUnitFrontmatter(metadata)
	if err != nil {
		return nil, nil, MetadataSourceNone, err
	}
	if err := validateUnitFrontmatter(unitFrontmatter); err != nil {
		return nil, nil, MetadataSourceNone, err
	}

	return unitFrontmatter, body, MetadataSourceMetadataBlock, nil
}

// ParseTaskMetadata parses task metadata from frontmatter or metadata block.
func ParseTaskMetadata(content []byte) (*TaskFrontmatter, []byte, MetadataSource, error) {
	if bytes.HasPrefix(content, []byte("---\n")) {
		frontmatter, body, err := ParseFrontmatter(content)
		if err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		taskFrontmatter, err := ParseTaskFrontmatter(frontmatter)
		if err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		if err := validateTaskFrontmatter(taskFrontmatter); err != nil {
			return nil, nil, MetadataSourceNone, err
		}
		return taskFrontmatter, body, MetadataSourceFrontmatter, nil
	}

	metadata, body, found, err := ParseMetadataBlock(content)
	if err != nil {
		return nil, nil, MetadataSourceNone, err
	}
	if !found {
		return nil, content, MetadataSourceNone, fmt.Errorf("missing metadata")
	}

	taskFrontmatter, err := ParseTaskFrontmatter(metadata)
	if err != nil {
		return nil, nil, MetadataSourceNone, err
	}
	if err := validateTaskFrontmatter(taskFrontmatter); err != nil {
		return nil, nil, MetadataSourceNone, err
	}

	return taskFrontmatter, body, MetadataSourceMetadataBlock, nil
}

func validateTaskFrontmatter(tf *TaskFrontmatter) error {
	if tf.Task < 1 {
		return fmt.Errorf("task must be >= 1")
	}
	if strings.TrimSpace(tf.Backpressure) == "" {
		return fmt.Errorf("backpressure must be set")
	}
	if _, err := parseTaskStatus(tf.Status); err != nil {
		return err
	}
	return nil
}

func validateUnitFrontmatter(uf *UnitFrontmatter) error {
	if strings.TrimSpace(uf.Unit) == "" {
		return fmt.Errorf("unit must be set")
	}
	if uf.OrchStatus != "" {
		if _, err := parseUnitStatus(uf.OrchStatus); err != nil {
			return err
		}
	}
	return nil
}
