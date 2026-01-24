package discovery

import (
	"strings"
	"testing"
)

func TestParseTaskMetadata_MetadataBlock(t *testing.T) {
	content := []byte("# Intro\n\n## Metadata\n```yaml\ntask: 1\nbackpressure: go test ./...\n```\n\n# Task Title\n")

	tf, body, source, err := ParseTaskMetadata(content)
	if err != nil {
		t.Fatalf("ParseTaskMetadata failed: %v", err)
	}
	if source != MetadataSourceMetadataBlock {
		t.Fatalf("expected metadata source %q, got %q", MetadataSourceMetadataBlock, source)
	}
	if tf.Task != 1 {
		t.Fatalf("expected task 1, got %d", tf.Task)
	}
	if strings.Contains(string(body), "## Metadata") {
		t.Fatalf("body should not contain metadata section")
	}
	if strings.Contains(string(body), "```yaml") {
		t.Fatalf("body should not contain metadata fence")
	}
}

func TestParseTaskMetadata_FrontmatterPrecedence(t *testing.T) {
	content := []byte("---\ntask: 2\nbackpressure: go test ./...\n---\n\n# Task Title\n\n## Metadata\n```yaml\ntask: 1\nbackpressure: nope\n```\n")

	tf, _, source, err := ParseTaskMetadata(content)
	if err != nil {
		t.Fatalf("ParseTaskMetadata failed: %v", err)
	}
	if source != MetadataSourceFrontmatter {
		t.Fatalf("expected metadata source %q, got %q", MetadataSourceFrontmatter, source)
	}
	if tf.Task != 2 {
		t.Fatalf("expected task 2, got %d", tf.Task)
	}
}

func TestParseTaskMetadata_InvalidMetadataBlock(t *testing.T) {
	content := []byte("# Task\n\n## Metadata\n```yaml\ntask: 1\nbackpressure: go test ./...\n")

	_, _, _, err := ParseTaskMetadata(content)
	if err == nil {
		t.Fatal("expected error for invalid metadata block, got nil")
	}
	if !strings.Contains(err.Error(), "missing closing fence") {
		t.Fatalf("expected missing closing fence error, got: %v", err)
	}
}

func TestParseTaskMetadata_MissingMetadata(t *testing.T) {
	content := []byte("# Task\n\nNo metadata here.\n")

	_, _, _, err := ParseTaskMetadata(content)
	if err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
	if !strings.Contains(err.Error(), "missing metadata") {
		t.Fatalf("expected missing metadata error, got: %v", err)
	}
}
