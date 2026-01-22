---
task: 3
status: complete
backpressure: "go test ./internal/provider/... -run TestExtract -v"
depends_on: [1, 2]
---

# JSON Extraction and Output Parsing

**Parent spec**: `/specs/CLAUDE-REVIEWER.md`
**Task**: #3 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the JSON extraction functions and parseOutput method that extract structured review data from Claude's output, handling both markdown code fences and bare JSON.

## Dependencies

### External Specs (must be implemented)
- REVIEWER-INTERFACE - provides `ReviewResult`, `ReviewIssue` types

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `ClaudeReviewer` struct)
- Task #2 must be complete (provides: `BuildClaudeReviewPrompt` function)

### Package Dependencies
- `encoding/json` (standard library)
- `strings` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/provider/
└── claude_reviewer.go    # MODIFY: Add parseOutput and extraction functions
```

### Functions to Implement

```go
// internal/provider/claude_reviewer.go (additions)

// parseOutput extracts and parses JSON from Claude's response.
// Returns graceful degradation (passed=true) if parsing fails.
func (r *ClaudeReviewer) parseOutput(output string) (*ReviewResult, error) {
    // Extract JSON from Claude's response
    jsonStr := extractJSON(output)
    if jsonStr == "" {
        return &ReviewResult{
            Passed:    true,
            Summary:   "No structured review output",
            RawOutput: output,
        }, nil
    }

    var parsed struct {
        Passed  bool          `json:"passed"`
        Summary string        `json:"summary"`
        Issues  []ReviewIssue `json:"issues"`
    }

    if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
        return &ReviewResult{
            Passed:    true,
            Summary:   "Failed to parse review output",
            RawOutput: output,
        }, nil
    }

    return &ReviewResult{
        Passed:    parsed.Passed,
        Summary:   parsed.Summary,
        Issues:    parsed.Issues,
        RawOutput: output,
    }, nil
}

// extractJSON finds and returns the first JSON object in the output.
// Handles JSON in markdown code fences and bare JSON.
// Returns empty string if no valid JSON found.
func extractJSON(output string) string {
    // First, try to extract JSON from markdown code fence
    if jsonStr := extractJSONFromCodeFence(output); jsonStr != "" {
        return jsonStr
    }

    // Fall back to finding bare JSON by brace matching
    return extractJSONByBraces(output)
}

// extractJSONFromCodeFence extracts JSON from markdown code fences.
// Looks for ```json or ``` followed by JSON content.
func extractJSONFromCodeFence(output string) string {
    markers := []string{"```json\n", "```\n"}
    for _, marker := range markers {
        start := strings.Index(output, marker)
        if start == -1 {
            continue
        }
        contentStart := start + len(marker)
        // Find the closing ```
        end := strings.Index(output[contentStart:], "```")
        if end == -1 {
            continue
        }
        content := strings.TrimSpace(output[contentStart : contentStart+end])
        if strings.HasPrefix(content, "{") {
            return content
        }
    }
    return ""
}

// extractJSONByBraces finds JSON by matching braces.
// Scans for first { and tracks depth until matching } found.
func extractJSONByBraces(output string) string {
    start := -1
    depth := 0

    for i, ch := range output {
        if ch == '{' {
            if start == -1 {
                start = i
            }
            depth++
        } else if ch == '}' {
            depth--
            if depth == 0 && start != -1 {
                return output[start : i+1]
            }
        }
    }

    return ""
}
```

## Implementation Notes

### JSON Extraction Strategy

Claude's output may include explanatory text before and after the JSON. The extraction handles:

1. **Markdown code fences**: Checks for ` ```json ` or ` ``` ` blocks first (common Claude behavior)
2. **Bare JSON**: Falls back to brace matching to find the outermost JSON object

### Graceful Degradation

When JSON parsing fails, return `passed=true` to avoid blocking development:

| Scenario | Result |
|----------|--------|
| No JSON in output | `passed=true`, summary="No structured review output" |
| Malformed JSON | `passed=true`, summary="Failed to parse review output" |
| Valid JSON, no issues | `passed=true`, issues=[] |
| Valid JSON, issues found | `passed=false`, issues populated |

### Brace Matching Algorithm

1. Scan for first `{` character
2. Track brace depth (increment on `{`, decrement on `}`)
3. Return substring when depth returns to 0
4. Return empty string if no valid JSON found

This handles nested JSON objects correctly.

## Backpressure

### Validation Command

```bash
go test ./internal/provider/... -run TestExtract -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestExtractJSON_PlainJSON` | Extracts bare JSON object |
| `TestExtractJSON_JSONWithPrefixText` | Extracts JSON after explanatory text |
| `TestExtractJSON_JSONWithSuffixText` | Extracts JSON before trailing text |
| `TestExtractJSON_NestedJSON` | Correctly handles nested objects |
| `TestExtractJSON_MarkdownCodeFence` | Extracts from ` ```json ` block |
| `TestExtractJSON_PlainCodeFence` | Extracts from ` ``` ` block |
| `TestExtractJSON_NoJSON` | Returns empty string for text without JSON |
| `TestExtractJSON_IncompleteJSON` | Returns empty string for incomplete braces |
| `TestClaudeReviewer_ParseOutput_ValidJSON` | Parses valid JSON into ReviewResult |
| `TestClaudeReviewer_ParseOutput_NoJSON` | Returns graceful degradation for no JSON |
| `TestClaudeReviewer_ParseOutput_MalformedJSON` | Returns graceful degradation for invalid JSON |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## NOT In Scope

- Integration tests with actual Claude CLI
- Structured output API (not available in Claude CLI)
- Review caching or incremental review
- Custom JSON schema validation
