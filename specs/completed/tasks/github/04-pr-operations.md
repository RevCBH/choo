---
task: 4
status: complete
backpressure: "go test ./internal/github/... -run TestPR"
depends_on: [1, 2]
---

# PR Operations

**Parent spec**: `/specs/GITHUB.md`
**Task**: #4 of 5 in implementation plan

## Objective

Implement PR lifecycle operations: CreatePR (delegation stub for Claude/gh CLI), GetPR, UpdatePR, Merge (squash), and ClosePR.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRInfo`, `MergeResult`)
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
    ├── pr.go       # MODIFY: Add PR operation methods
    └── pr_test.go  # CREATE: Tests for PR operations
```

### Functions to Implement

```go
// internal/github/pr.go

// CreatePR is a placeholder - PR creation is delegated to Claude via gh CLI.
// This returns an error indicating the operation should be performed externally.
// The orchestrator will instruct Claude to run: gh pr create --title "..." --body "..."
func (c *PRClient) CreatePR(ctx context.Context, title, body, branch string) (*PRInfo, error) {
    // Return error explaining PR creation is delegated to Claude
    return nil, fmt.Errorf("PR creation is delegated to Claude via 'gh pr create'")
}

// GetPR fetches current PR information by number
func (c *PRClient) GetPR(ctx context.Context, prNumber int) (*PRInfo, error) {
    // GET /repos/{owner}/{repo}/pulls/{pull_number}
    // Parse response into PRInfo
}

// UpdatePR updates an existing PR's title or body
func (c *PRClient) UpdatePR(ctx context.Context, prNumber int, title, body string) error {
    // PATCH /repos/{owner}/{repo}/pulls/{pull_number}
    // Send title and body in request body
}

// Merge executes a squash merge on the PR
func (c *PRClient) Merge(ctx context.Context, prNumber int) (*MergeResult, error) {
    // PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge
    // merge_method: "squash"
}

// ClosePR closes a PR without merging
func (c *PRClient) ClosePR(ctx context.Context, prNumber int) error {
    // PATCH /repos/{owner}/{repo}/pulls/{pull_number}
    // state: "closed"
}
```

### GitHub API Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Get PR | GET | `/repos/{owner}/{repo}/pulls/{pull_number}` |
| Update PR | PATCH | `/repos/{owner}/{repo}/pulls/{pull_number}` |
| Merge PR | PUT | `/repos/{owner}/{repo}/pulls/{pull_number}/merge` |
| Close PR | PATCH | `/repos/{owner}/{repo}/pulls/{pull_number}` |

### API Response Structures

```go
// GitHub API response for PR
type ghPullRequest struct {
    Number    int       `json:"number"`
    HTMLURL   string    `json:"html_url"`
    Head      ghRef     `json:"head"`
    Base      ghRef     `json:"base"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"`
}

type ghRef struct {
    Ref string `json:"ref"`
}

// GitHub API response for merge
type ghMergeResponse struct {
    SHA     string `json:"sha"`
    Merged  bool   `json:"merged"`
    Message string `json:"message"`
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/github/... -run TestPR -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGetPR_Success` | Returns PRInfo with correct number, URL, branches |
| `TestGetPR_NotFound` | Returns error for 404 response |
| `TestUpdatePR_Success` | Returns nil error, request body contains title/body |
| `TestMerge_SquashMethod` | Request body contains `merge_method: "squash"` |
| `TestMerge_Success` | Returns MergeResult with Merged=true, SHA set |
| `TestMerge_Conflict` | Returns error for 409 conflict response |
| `TestClosePR_Success` | Request body contains `state: "closed"` |
| `TestCreatePR_DelegationError` | Returns error indicating Claude delegation |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock PR JSON | In test code | GetPR response parsing |
| Mock merge response | In test code | Merge result parsing |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses httptest)
- [x] Runs in <60 seconds

## Implementation Notes

### Merge Request Body

```go
body := map[string]string{
    "merge_method": "squash",
}
```

### Close Request Body

```go
body := map[string]string{
    "state": "closed",
}
```

### Error Handling

| Status | Meaning | Action |
|--------|---------|--------|
| 200 | Success | Return result |
| 404 | PR not found | Return error |
| 405 | Method not allowed | Return error |
| 409 | Merge conflict | Return error |
| 422 | Validation failed | Return error |

### PR Creation Note

PR creation is intentionally not implemented via API. The design spec states:
> "PR creation is delegated to Claude, which uses `gh pr create`"

This allows Claude to generate appropriate PR titles/descriptions with context. The `CreatePR` method exists as a stub to make the API surface complete but returns an error directing callers to use Claude.

## NOT In Scope

- PR creation implementation (delegated to Claude/gh CLI)
- Review polling (Task #3)
- Comment fetching (Task #5)
- Rebase before merge (future enhancement)
- Required checks waiting (future enhancement)
