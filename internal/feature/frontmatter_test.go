package feature

import (
	"bytes"
	"testing"
)

// Valid frontmatter with body
var validWithBody = `---
prd_id: test-feature
title: Test Feature
status: draft
---

# Test Feature

This is the body.
`

// Valid frontmatter at end of file
var validNoBody = `---
prd_id: test-feature
title: Test Feature
status: draft
---`

// Missing opening delimiter
var noOpening = `prd_id: test-feature
title: Test Feature
---
# Body`

// Missing closing delimiter
var noClosing = `---
prd_id: test-feature
title: Test Feature
# Body without closing delimiter`

func TestParseFrontmatter_Valid(t *testing.T) {
	frontmatter, body, err := parseFrontmatter([]byte(validWithBody))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedFrontmatter := `prd_id: test-feature
title: Test Feature
status: draft`
	if string(frontmatter) != expectedFrontmatter {
		t.Errorf("frontmatter mismatch\nexpected: %q\ngot: %q", expectedFrontmatter, string(frontmatter))
	}

	expectedBody := `
# Test Feature

This is the body.
`
	if string(body) != expectedBody {
		t.Errorf("body mismatch\nexpected: %q\ngot: %q", expectedBody, string(body))
	}
}

func TestParseFrontmatter_NoOpeningDelimiter(t *testing.T) {
	_, _, err := parseFrontmatter([]byte(noOpening))
	if err == nil {
		t.Fatal("expected error for missing opening delimiter, got nil")
	}
	if err.Error() != "missing frontmatter delimiter" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseFrontmatter_NoClosingDelimiter(t *testing.T) {
	_, _, err := parseFrontmatter([]byte(noClosing))
	if err == nil {
		t.Fatal("expected error for missing closing delimiter, got nil")
	}
	if err.Error() != "missing closing frontmatter delimiter" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseFrontmatter_EmptyBody(t *testing.T) {
	frontmatter, body, err := parseFrontmatter([]byte(validNoBody))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedFrontmatter := `prd_id: test-feature
title: Test Feature
status: draft`
	if string(frontmatter) != expectedFrontmatter {
		t.Errorf("frontmatter mismatch\nexpected: %q\ngot: %q", expectedFrontmatter, string(frontmatter))
	}

	if len(body) != 0 {
		t.Errorf("expected empty body, got: %q", string(body))
	}
}

func TestParseFrontmatter_TrailingWhitespace(t *testing.T) {
	content := `---
prd_id: test-feature
title: Test Feature
status: draft
---


# Body with leading newlines`

	frontmatter, body, err := parseFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedFrontmatter := `prd_id: test-feature
title: Test Feature
status: draft`
	if string(frontmatter) != expectedFrontmatter {
		t.Errorf("frontmatter mismatch\nexpected: %q\ngot: %q", expectedFrontmatter, string(frontmatter))
	}

	expectedBody := `

# Body with leading newlines`
	if !bytes.Equal(body, []byte(expectedBody)) {
		t.Errorf("body mismatch\nexpected: %q\ngot: %q", expectedBody, string(body))
	}
}
