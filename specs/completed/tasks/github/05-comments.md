---
task: 5
status: complete
backpressure: "go test ./internal/github/... -run TestComments"
depends_on: [1, 2]
---

# PR Comments

**Parent spec**: `/specs/GITHUB.md`
**Task**: #5 of 5 in implementation plan

## Objective

Implement PR comment fetching: retrieve all review comments on a PR and filter for unaddressed comments since a given timestamp.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRComment`)
- Task #2 must be complete (provides: `PRClient`, `doRequest`)

### Package Dependencies
- `context` (standard library)
- `encoding/json` (standard library)
- `fmt` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── github/
    ├── comments.go       # MODIFY: Add comment fetching methods
    └── comments_test.go  # CREATE: Tests for comment operations
```

### Functions to Implement

```go
// internal/github/comments.go

// ghReviewComment is the GitHub API response for a PR review comment
type ghReviewComment struct {
    ID        int64     `json:"id"`
    Path      string    `json:"path"`
    Line      int       `json:"line"`
    Body      string    `json:"body"`
    User      ghUser    `json:"user"`
    CreatedAt time.Time `json:"created_at"`
}

type ghUser struct {
    Login string `json:"login"`
}

// GetPRComments fetches all review comments on a PR
func (c *PRClient) GetPRComments(ctx context.Context, prNumber int) ([]PRComment, error) {
    // GET /repos/{owner}/{repo}/pulls/{pull_number}/comments
    // Parse response and convert to []PRComment
}

// GetUnaddressedComments returns comments since the given timestamp
// Used to find comments that need to be addressed after a push
func (c *PRClient) GetUnaddressedComments(ctx context.Context, prNumber int, since time.Time) ([]PRComment, error) {
    // 1. Fetch all comments via GetPRComments
    // 2. Filter to comments where CreatedAt > since
    // 3. Return filtered list
}
```

### GitHub API Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| List review comments | GET | `/repos/{owner}/{repo}/pulls/{pull_number}/comments` |

### API Response Structure

```go
// GitHub API response (array of)
type ghReviewComment struct {
    ID                  int64     `json:"id"`
    Path                string    `json:"path"`
    Line                int       `json:"line"`                   // null if general comment
    OriginalLine        int       `json:"original_line"`
    Body                string    `json:"body"`
    User                ghUser    `json:"user"`
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
    HTMLURL             string    `json:"html_url"`
    PullRequestReviewID int64     `json:"pull_request_review_id"`
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/github/... -run TestComments -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGetPRComments_Empty` | Returns empty slice for PR with no comments |
| `TestGetPRComments_Multiple` | Returns all comments with correct fields |
| `TestGetPRComments_ParsesPath` | Path field correctly extracted |
| `TestGetPRComments_ParsesLine` | Line field correctly extracted (0 if null) |
| `TestGetPRComments_ParsesAuthor` | Author login correctly extracted |
| `TestGetUnaddressedComments_FiltersBySince` | Only returns comments after `since` time |
| `TestGetUnaddressedComments_AllNew` | Returns all comments if all after `since` |
| `TestGetUnaddressedComments_NoneNew` | Returns empty if all before `since` |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock comments JSON | In test code | Various comment scenarios |

### Mock Response Example

```json
[
  {
    "id": 1234,
    "path": "internal/github/client.go",
    "line": 42,
    "body": "Consider adding error handling here",
    "user": {"login": "reviewer-bot"},
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "id": 1235,
    "path": "internal/github/pr.go",
    "line": null,
    "body": "General feedback: nice work!",
    "user": {"login": "human-reviewer"},
    "created_at": "2024-01-15T11:00:00Z"
  }
]
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses httptest)
- [x] Runs in <60 seconds

## Implementation Notes

### Pagination

GitHub API paginates results. For MVP, assume <100 comments per PR. Future enhancement: implement pagination with `Link` header parsing.

### Line Number Handling

The `line` field can be null in the API response (for general comments not attached to a specific line). Handle this:

```go
// Line may be null in JSON
line := 0
if comment.Line != nil {
    line = *comment.Line
}
```

Or use a pointer type during parsing:

```go
type ghReviewComment struct {
    Line *int `json:"line"`
    // ...
}
```

### Filtering Implementation

```go
func (c *PRClient) GetUnaddressedComments(ctx context.Context, prNumber int, since time.Time) ([]PRComment, error) {
    all, err := c.GetPRComments(ctx, prNumber)
    if err != nil {
        return nil, err
    }

    var unaddressed []PRComment
    for _, comment := range all {
        if comment.CreatedAt.After(since) {
            unaddressed = append(unaddressed, comment)
        }
    }
    return unaddressed, nil
}
```

### Integration with Review Polling

Task #3 (review polling) stubs `GetPRComments` to return empty slice. Once this task is complete, the integration will work automatically since Task #3 calls `GetPRComments`.

## NOT In Scope

- Issue comments (different API endpoint)
- Review comment replies
- Resolving/unresolving comments
- Creating comments programmatically
- Pagination beyond 100 comments
