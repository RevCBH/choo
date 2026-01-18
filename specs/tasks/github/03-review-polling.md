---
task: 3
status: complete
backpressure: "go test ./internal/github/... -run TestReview"
depends_on: [1, 2]
---

# Review Polling

**Parent spec**: `/specs/GITHUB.md`
**Task**: #3 of 5 in implementation plan

## Objective

Implement the emoji-based review state machine: polling PR reactions for eyes/thumbs-up emojis, determining review status, and waiting for approval with timeout support.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `ReviewStatus`, `ReviewState`, `PollResult`)
- Task #2 must be complete (provides: `PRClient`, `doRequest`)

### Package Dependencies
- `context` (standard library)
- `time` (standard library)
- `encoding/json` (standard library)
- `fmt` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── github/
    ├── review.go       # MODIFY: Add polling methods
    └── review_test.go  # CREATE: Tests for review polling
```

### Functions to Implement

```go
// internal/github/review.go

// Reaction represents a GitHub reaction on a PR/issue
type Reaction struct {
    ID        int64     `json:"id"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}

// getReactions fetches reactions on a PR (issue endpoint)
func (c *PRClient) getReactions(ctx context.Context, prNumber int) ([]Reaction, error) {
    // GET /repos/{owner}/{repo}/issues/{issue_number}/reactions
}

// GetReviewStatus fetches the current review status from reactions
func (c *PRClient) GetReviewStatus(ctx context.Context, prNumber int) (*ReviewState, error) {
    // 1. Fetch reactions via getReactions
    // 2. Check for "eyes" and "+1" (thumbs up) reactions
    // 3. Fetch comment count via GetPRComments (stub until Task #5)
    // 4. Determine status with precedence: approved > in_progress > changes_requested > pending
}

// PollReview polls the PR for review status changes
// Returns when approval received, feedback found, or timeout
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error) {
    // 1. Start ticker with pollInterval
    // 2. On each tick: GetReviewStatus
    // 3. Track state changes, emit events (when events package exists)
    // 4. Check timeout condition
    // 5. Return on terminal conditions: approved, changes_requested, timeout
}

// WaitForApproval blocks until PR is approved or timeout
// Emits events on state changes
func (c *PRClient) WaitForApproval(ctx context.Context, prNumber int, prStartTime time.Time) error {
    // Wrapper around PollReview with error handling
}
```

### Emoji State Machine

```
Precedence (highest to lowest):
1. HasThumbsUp (+1) → ReviewApproved
2. HasEyes → ReviewInProgress
3. CommentCount > 0 → ReviewChangesRequested
4. Default → ReviewPending
```

### GitHub API Endpoints

| Operation | Endpoint |
|-----------|----------|
| Get reactions | `GET /repos/{owner}/{repo}/issues/{issue_number}/reactions` |

Note: PRs are issues in GitHub's API, so reactions use the issues endpoint.

## Backpressure

### Validation Command

```bash
go test ./internal/github/... -run TestReview -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGetReviewStatus_NoReactions` | Returns `ReviewPending` |
| `TestGetReviewStatus_EyesOnly` | Returns `ReviewInProgress` |
| `TestGetReviewStatus_ThumbsUp` | Returns `ReviewApproved` |
| `TestGetReviewStatus_ThumbsUpBeatsEyes` | Thumbs up present with eyes returns `ReviewApproved` |
| `TestGetReviewStatus_CommentsWithoutEmoji` | Returns `ReviewChangesRequested` |
| `TestGetReviewStatus_EyesBeatsComments` | Eyes present with comments returns `ReviewInProgress` |
| `TestPollReview_Timeout` | Returns `TimedOut: true` after reviewTimeout |
| `TestPollReview_ApprovalStopsPolling` | Returns `ShouldMerge: true` when thumbs up added |
| `TestPollReview_ContextCancellation` | Returns `ctx.Err()` when context cancelled |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Mock reactions JSON | In test code | Various reaction combinations |

### Mock Setup

```go
func newMockClient(reactions []Reaction, comments []PRComment) *PRClient {
    // Setup httptest.Server returning canned responses
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (uses httptest)
- [x] Runs in <60 seconds (short poll intervals in tests)

## Implementation Notes

### Reaction Content Values

GitHub reaction content strings:
- `+1` = thumbs up
- `-1` = thumbs down
- `eyes` = eyes
- `laugh` = laughing
- `confused` = confused
- `heart` = heart
- `hooray` = party
- `rocket` = rocket

### Polling Loop Pattern

```go
ticker := time.NewTicker(c.pollInterval)
defer ticker.Stop()

for {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-ticker.C:
        // Poll and check conditions
    }
}
```

### Test Timing

Use short intervals for tests:
- `pollInterval: 10 * time.Millisecond`
- `reviewTimeout: 50 * time.Millisecond`

## NOT In Scope

- PR creation/merge (Task #4)
- Full comment fetching (Task #5) - stub GetPRComments returns empty for now
- Event emission (deferred until events package)
