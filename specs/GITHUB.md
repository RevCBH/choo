# GITHUB — GitHub PR Lifecycle for Ralph Orchestrator

## Overview

The GITHUB package manages the complete PR lifecycle for choo: creating pull requests, polling for review status via emoji reactions (Codex convention), addressing feedback, and executing squash merges. PR creation is delegated to Claude via the `gh` CLI, while review polling and merge operations are handled programmatically.

The review process uses an emoji-based state machine on the PR body (Codex workflow):
- No reaction: pending review
- Eyes reaction: review in progress
- Thumbs up reaction: approved, ready to merge

This bot-only review workflow enables fully autonomous PR lifecycle management without human intervention, while preserving the option to add human reviewers later.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           PR Lifecycle                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐         │
│   │  Create  │───▶│  Review  │───▶│ Feedback │───▶│  Merge   │         │
│   │    PR    │    │  Polling │    │   Loop   │    │          │         │
│   └──────────┘    └──────────┘    └──────────┘    └──────────┘         │
│        │               │               │               │                │
│        ▼               ▼               ▼               ▼                │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐         │
│   │ gh pr    │    │ Emoji    │    │ Claude   │    │ Squash   │         │
│   │ create   │    │  State   │    │ Feedback │    │  Merge   │         │
│   │ (Claude) │    │ Machine  │    │  Prompt  │    │   API    │         │
│   └──────────┘    └──────────┘    └──────────┘    └──────────┘         │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Delegate PR creation to Claude via `gh pr create` command
2. Poll PR reactions to detect review state changes (eyes/thumbs up emojis)
3. Support 2-hour review timeout with configurable escalation
4. Fetch PR review comments for feedback addressing
5. Execute squash merge via GitHub API after approval
6. Support bot-only review workflow (Codex convention)
7. Emit events for all PR state transitions
8. Handle GitHub API rate limiting gracefully

### Performance Requirements

| Metric | Target |
|--------|--------|
| Poll interval | 30 seconds |
| Review timeout | 2 hours (default) |
| API retry backoff | Exponential (1s, 2s, 4s, 8s, 16s) |
| Max concurrent API calls | 10 per unit |

### Constraints

- Uses `gh` CLI for PR creation (delegated to Claude)
- Uses GitHub REST API for polling and merge operations
- Authentication via `GITHUB_TOKEN` or `gh auth token`
- Depends on: `discovery.Unit` for PR metadata

## Design

### Module Structure

```
internal/github/
├── client.go       # GitHub API client, authentication
├── pr.go           # PR creation delegation, update, merge
├── review.go       # Review status polling, emoji state machine
└── comments.go     # PR comment fetching for feedback
```

### Core Types

```go
// internal/github/client.go

// PRClient manages GitHub PR operations
type PRClient struct {
    // httpClient is the authenticated HTTP client
    httpClient *http.Client

    // owner is the repository owner (org or user)
    owner string

    // repo is the repository name
    repo string

    // pollInterval is the duration between review polls
    pollInterval time.Duration

    // reviewTimeout is the max wait time before escalation
    reviewTimeout time.Duration

    // events is the event bus for emitting PR events
    events *events.Bus
}

// PRClientConfig holds configuration for the PR client
type PRClientConfig struct {
    // Owner is the repository owner (auto-detected if empty)
    Owner string

    // Repo is the repository name (auto-detected if empty)
    Repo string

    // PollInterval is the duration between review polls (default: 30s)
    PollInterval time.Duration

    // ReviewTimeout is the max wait time before escalation (default: 2h)
    ReviewTimeout time.Duration
}
```

```go
// internal/github/review.go

// ReviewStatus represents the current state of PR review
type ReviewStatus string

const (
    // ReviewPending indicates no review activity yet
    ReviewPending ReviewStatus = "pending"

    // ReviewInProgress indicates reviewer is actively reviewing (eyes emoji)
    ReviewInProgress ReviewStatus = "in_progress"

    // ReviewApproved indicates PR is approved (thumbs up emoji)
    ReviewApproved ReviewStatus = "approved"

    // ReviewChangesRequested indicates feedback exists without approval
    ReviewChangesRequested ReviewStatus = "changes_requested"

    // ReviewTimeout indicates review timeout was exceeded
    ReviewTimeout ReviewStatus = "timeout"
)

// ReviewState holds the parsed review state from PR reactions
type ReviewState struct {
    // Status is the current review status
    Status ReviewStatus

    // HasEyes is true if eyes reaction present
    HasEyes bool

    // HasThumbsUp is true if thumbs up reaction present
    HasThumbsUp bool

    // CommentCount is the number of review comments
    CommentCount int

    // LastActivity is the timestamp of last reaction/comment
    LastActivity time.Time
}

// PollResult represents the result of a single poll iteration
type PollResult struct {
    // State is the current review state
    State ReviewState

    // Changed is true if state changed since last poll
    Changed bool

    // ShouldMerge is true if PR is approved and ready
    ShouldMerge bool

    // HasFeedback is true if there are comments to address
    HasFeedback bool

    // TimedOut is true if review timeout exceeded
    TimedOut bool
}
```

```go
// internal/github/pr.go

// PRInfo holds information about a created PR
type PRInfo struct {
    // Number is the PR number
    Number int

    // URL is the web URL of the PR
    URL string

    // Branch is the source branch name
    Branch string

    // TargetBranch is the base branch for the PR
    TargetBranch string

    // Title is the PR title
    Title string

    // CreatedAt is when the PR was created
    CreatedAt time.Time
}

// MergeResult holds the result of a merge operation
type MergeResult struct {
    // Merged is true if merge succeeded
    Merged bool

    // SHA is the merge commit SHA
    SHA string

    // Message is the merge commit message
    Message string
}
```

```go
// internal/github/comments.go

// PRComment represents a review comment on a PR
type PRComment struct {
    // ID is the comment ID
    ID int64

    // Path is the file path the comment is on
    Path string

    // Line is the line number (0 if general comment)
    Line int

    // Body is the comment text
    Body string

    // Author is the comment author's login
    Author string

    // CreatedAt is when the comment was created
    CreatedAt time.Time
}
```

### API Surface

```go
// internal/github/client.go

// NewPRClient creates a new GitHub PR client
func NewPRClient(cfg PRClientConfig, events *events.Bus) (*PRClient, error)

// getToken retrieves the GitHub token from env or gh CLI
func getToken() (string, error)

// detectOwnerRepo parses owner/repo from git remote
func detectOwnerRepo() (owner, repo string, err error)
```

```go
// internal/github/pr.go

// CreatePR instructs Claude to create a PR via gh CLI
// Returns the PR info after Claude creates it
func (c *PRClient) CreatePR(ctx context.Context, unit *discovery.Unit) (*PRInfo, error)

// GetPR fetches current PR information by number
func (c *PRClient) GetPR(ctx context.Context, prNumber int) (*PRInfo, error)

// UpdatePR updates an existing PR's title or body
func (c *PRClient) UpdatePR(ctx context.Context, prNumber int, title, body string) error

// Merge executes a squash merge on the PR
func (c *PRClient) Merge(ctx context.Context, prNumber int) (*MergeResult, error)

// ClosePR closes a PR without merging
func (c *PRClient) ClosePR(ctx context.Context, prNumber int) error
```

```go
// internal/github/review.go

// PollReview polls the PR for review status changes
// Returns when approval received, feedback found, or timeout
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error)

// GetReviewStatus fetches the current review status from reactions
func (c *PRClient) GetReviewStatus(ctx context.Context, prNumber int) (*ReviewState, error)

// WaitForApproval blocks until PR is approved or timeout
// Emits events on state changes
func (c *PRClient) WaitForApproval(ctx context.Context, prNumber int, prStartTime time.Time) error
```

```go
// internal/github/comments.go

// GetPRComments fetches all review comments on a PR
func (c *PRClient) GetPRComments(ctx context.Context, prNumber int) ([]PRComment, error)

// GetUnaddressedComments returns comments since last push
func (c *PRClient) GetUnaddressedComments(ctx context.Context, prNumber int, since time.Time) ([]PRComment, error)
```

### Emoji State Machine

The review state machine uses GitHub reactions on the PR body as signals:

```
                     ┌──────────────────┐
                     │                  │
                     ▼                  │
              ┌─────────────┐           │
              │   Pending   │───────────┤
              │ (no emoji)  │           │
              └──────┬──────┘           │
                     │                  │
                     │ +eyes            │
                     ▼                  │
              ┌─────────────┐           │
         ┌───▶│ In Progress │           │
         │    │   (eyes)    │───────────┤
         │    └──────┬──────┘           │
         │           │                  │
  -eyes  │           │ +thumbs_up      │ timeout
  +eyes  │           ▼                  │
         │    ┌─────────────┐           │
         │    │  Approved   │           │
         └────│(thumbs_up)  │           │
              └──────┬──────┘           │
                     │                  │
                     │ merge            │
                     ▼                  ▼
              ┌─────────────┐    ┌─────────────┐
              │   Merged    │    │  Escalate   │
              └─────────────┘    └─────────────┘
```

### PR Creation Flow

PR creation is delegated to Claude, which uses `gh pr create`:

```
1. Worker completes all tasks
2. Worker pushes branch to remote
3. Worker invokes Claude with PR creation prompt
4. Claude runs: gh pr create --title "..." --body "..."
5. Worker parses PR number from Claude output or git state
6. Worker stores PR number in unit frontmatter
7. Worker emits PRCreated event
```

### Review Polling Flow

```
Input: PR number, start time

Loop (every 30s):
  1. GET /repos/{owner}/{repo}/issues/{pr}/reactions
  2. Parse reactions:
     - Look for "eyes" (+1 = in progress)
     - Look for "+1" (thumbs up = approved)
  3. Check for comments via GET /repos/{owner}/{repo}/pulls/{pr}/comments
  4. Determine state:
     - If thumbs_up present → Approved
     - If eyes present → InProgress
     - If comments exist → ChangesRequested
     - Otherwise → Pending
  5. Emit event if state changed
  6. Check timeout (now - start_time > 2h):
     - If timeout → emit ReviewTimeout, return
  7. If Approved → return (ready to merge)
  8. If ChangesRequested → return (need feedback loop)
  9. Sleep poll interval
  10. Continue loop
```

### Merge Flow

```
Input: PR number, unit

1. Emit PRMergeQueued event
2. Acquire merge lock (FCFS queue)
3. Fetch latest target branch
4. Check if rebase needed (target moved)
   - If yes: rebase, handle conflicts, force push
5. PUT /repos/{owner}/{repo}/pulls/{pr}/merge
   - merge_method: "squash"
   - commit_title: "feat(unit): summary"
   - commit_message: PR body
6. Emit PRMerged event
7. Release merge lock
8. Return MergeResult
```

## Implementation Notes

### Authentication

```go
func getToken() (string, error) {
    // 1. Check environment
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        return token, nil
    }

    // 2. Try gh CLI
    cmd := exec.Command("gh", "auth", "token")
    out, err := cmd.Output()
    if err == nil {
        return strings.TrimSpace(string(out)), nil
    }

    return "", fmt.Errorf("no GitHub token found: set GITHUB_TOKEN or run 'gh auth login'")
}
```

### Review Status Detection

```go
func (c *PRClient) GetReviewStatus(ctx context.Context, prNumber int) (*ReviewState, error) {
    // GET /repos/{owner}/{repo}/issues/{issue_number}/reactions
    reactions, err := c.getReactions(ctx, prNumber)
    if err != nil {
        return nil, err
    }

    state := &ReviewState{
        Status: ReviewPending,
    }

    for _, r := range reactions {
        switch r.Content {
        case "eyes":
            state.HasEyes = true
        case "+1":
            state.HasThumbsUp = true
        }
        if r.CreatedAt.After(state.LastActivity) {
            state.LastActivity = r.CreatedAt
        }
    }

    // Check for comments
    comments, err := c.GetPRComments(ctx, prNumber)
    if err != nil {
        return nil, err
    }
    state.CommentCount = len(comments)

    // Determine status (precedence: approved > in_progress > changes > pending)
    if state.HasThumbsUp {
        state.Status = ReviewApproved
    } else if state.HasEyes {
        state.Status = ReviewInProgress
    } else if state.CommentCount > 0 {
        state.Status = ReviewChangesRequested
    }

    return state, nil
}
```

### Polling Implementation

```go
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error) {
    var lastStatus ReviewStatus
    startTime := time.Now()

    ticker := time.NewTicker(c.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()

        case <-ticker.C:
            state, err := c.GetReviewStatus(ctx, prNumber)
            if err != nil {
                // Log error but continue polling
                continue
            }

            changed := state.Status != lastStatus
            lastStatus = state.Status

            // Emit event on state change
            if changed {
                c.emitReviewEvent(prNumber, state.Status)
            }

            // Check timeout
            if time.Since(startTime) > c.reviewTimeout {
                return &PollResult{
                    State:    *state,
                    Changed:  changed,
                    TimedOut: true,
                }, nil
            }

            // Check terminal conditions
            if state.Status == ReviewApproved {
                return &PollResult{
                    State:       *state,
                    Changed:     changed,
                    ShouldMerge: true,
                }, nil
            }

            if state.Status == ReviewChangesRequested {
                return &PollResult{
                    State:       *state,
                    Changed:     changed,
                    HasFeedback: true,
                }, nil
            }
        }
    }
}
```

### Squash Merge

```go
func (c *PRClient) Merge(ctx context.Context, prNumber int) (*MergeResult, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/merge",
        c.owner, c.repo, prNumber)

    body := map[string]string{
        "merge_method": "squash",
    }

    resp, err := c.doRequest(ctx, "PUT", url, body)
    if err != nil {
        return nil, fmt.Errorf("merge failed: %w", err)
    }

    var result struct {
        SHA     string `json:"sha"`
        Merged  bool   `json:"merged"`
        Message string `json:"message"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return &MergeResult{
        Merged:  result.Merged,
        SHA:     result.SHA,
        Message: result.Message,
    }, nil
}
```

### Rate Limiting

```go
func (c *PRClient) doRequest(ctx context.Context, method, url string, body any) (*http.Response, error) {
    var lastErr error
    backoff := time.Second

    for attempt := 0; attempt < 5; attempt++ {
        resp, err := c.doRequestOnce(ctx, method, url, body)
        if err != nil {
            lastErr = err
            time.Sleep(backoff)
            backoff *= 2
            continue
        }

        // Check rate limit
        if resp.StatusCode == 403 || resp.StatusCode == 429 {
            retryAfter := resp.Header.Get("Retry-After")
            if d, err := time.ParseDuration(retryAfter + "s"); err == nil {
                time.Sleep(d)
            } else {
                time.Sleep(backoff)
                backoff *= 2
            }
            lastErr = fmt.Errorf("rate limited")
            continue
        }

        // 4xx errors don't retry (except rate limit)
        if resp.StatusCode >= 400 && resp.StatusCode < 500 {
            return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
        }

        // 5xx errors retry
        if resp.StatusCode >= 500 {
            lastErr = fmt.Errorf("GitHub server error: %d", resp.StatusCode)
            time.Sleep(backoff)
            backoff *= 2
            continue
        }

        return resp, nil
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

## Testing Strategy

### Unit Tests

```go
// internal/github/review_test.go

func TestGetReviewStatus(t *testing.T) {
    tests := []struct {
        name      string
        reactions []Reaction
        comments  []PRComment
        want      ReviewStatus
    }{
        {
            name:      "no reactions or comments",
            reactions: nil,
            comments:  nil,
            want:      ReviewPending,
        },
        {
            name:      "eyes only",
            reactions: []Reaction{{Content: "eyes"}},
            comments:  nil,
            want:      ReviewInProgress,
        },
        {
            name:      "thumbs up",
            reactions: []Reaction{{Content: "+1"}},
            comments:  nil,
            want:      ReviewApproved,
        },
        {
            name:      "thumbs up beats eyes",
            reactions: []Reaction{{Content: "eyes"}, {Content: "+1"}},
            comments:  nil,
            want:      ReviewApproved,
        },
        {
            name:      "comments without emoji",
            reactions: nil,
            comments:  []PRComment{{Body: "fix this"}},
            want:      ReviewChangesRequested,
        },
        {
            name:      "eyes beats comments",
            reactions: []Reaction{{Content: "eyes"}},
            comments:  []PRComment{{Body: "looks good so far"}},
            want:      ReviewInProgress,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client := newMockClient(tt.reactions, tt.comments)
            state, err := client.GetReviewStatus(context.Background(), 1)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if state.Status != tt.want {
                t.Errorf("GetReviewStatus() = %v, want %v", state.Status, tt.want)
            }
        })
    }
}

func TestPollReview_Timeout(t *testing.T) {
    client := &PRClient{
        pollInterval:  10 * time.Millisecond,
        reviewTimeout: 50 * time.Millisecond,
    }
    client.setMockReactions(nil) // No reactions = pending

    result, err := client.PollReview(context.Background(), 1)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !result.TimedOut {
        t.Error("expected timeout")
    }
}

func TestPollReview_ApprovalStopsPolling(t *testing.T) {
    client := &PRClient{
        pollInterval:  10 * time.Millisecond,
        reviewTimeout: time.Hour, // Long timeout
    }

    // Add thumbs up after 2 polls
    go func() {
        time.Sleep(25 * time.Millisecond)
        client.setMockReactions([]Reaction{{Content: "+1"}})
    }()

    start := time.Now()
    result, err := client.PollReview(context.Background(), 1)
    elapsed := time.Since(start)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !result.ShouldMerge {
        t.Error("expected ShouldMerge = true")
    }
    if elapsed > 100*time.Millisecond {
        t.Errorf("polling took too long: %v", elapsed)
    }
}
```

```go
// internal/github/pr_test.go

func TestMerge_SquashMethod(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify merge method
        var body map[string]string
        json.NewDecoder(r.Body).Decode(&body)

        if body["merge_method"] != "squash" {
            t.Errorf("merge_method = %v, want squash", body["merge_method"])
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]any{
            "sha":     "abc123",
            "merged":  true,
            "message": "Pull Request successfully merged",
        })
    }))
    defer server.Close()

    client := newTestClient(server.URL)
    result, err := client.Merge(context.Background(), 1)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !result.Merged {
        t.Error("expected Merged = true")
    }
    if result.SHA != "abc123" {
        t.Errorf("SHA = %v, want abc123", result.SHA)
    }
}
```

```go
// internal/github/client_test.go

func TestDoRequest_RateLimitRetry(t *testing.T) {
    attempts := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts < 3 {
            w.Header().Set("Retry-After", "0")
            w.WriteHeader(http.StatusTooManyRequests)
            return
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    client := newTestClient(server.URL)
    _, err := client.doRequest(context.Background(), "GET", server.URL, nil)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if attempts != 3 {
        t.Errorf("attempts = %d, want 3", attempts)
    }
}

func TestGetToken_EnvVariable(t *testing.T) {
    t.Setenv("GITHUB_TOKEN", "test-token-123")

    token, err := getToken()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if token != "test-token-123" {
        t.Errorf("token = %v, want test-token-123", token)
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| PR creation via Claude | Mock Claude CLI, verify gh command invoked |
| Review polling cycle | Mock API server with staged reactions |
| Feedback detection | Mock API with comments, verify ChangesRequested |
| Squash merge | Mock API, verify merge_method=squash |
| Rate limit handling | Mock 429 responses, verify retry with backoff |
| Timeout escalation | Short timeout, verify event emitted |

### Manual Testing

- [ ] Create PR via Claude/gh CLI integration works
- [ ] Eyes emoji triggers InProgress state
- [ ] Thumbs up emoji triggers Approved state
- [ ] Comments without emoji trigger ChangesRequested
- [ ] 2-hour timeout triggers escalation event
- [ ] Squash merge creates single commit
- [ ] Rate limiting retries correctly

## Design Decisions

### Why Delegate PR Creation to Claude?

Claude already has context about the changes and can generate appropriate PR titles and descriptions. The `gh` CLI provides a simple, reliable interface that Claude can invoke. This also preserves the existing ralph.sh workflow where Claude handles git operations.

Alternative considered: Direct API calls for PR creation. Rejected because it would require duplicating PR description generation logic.

### Why Emoji-Based Review State Machine?

The Codex convention uses emoji reactions for lightweight, async signaling:
- No need for formal GitHub review process
- Bot reviewers can easily add/remove reactions
- Human reviewers can participate using the same interface
- Simple state machine with clear precedence rules

Alternative considered: GitHub's native review API (APPROVE, REQUEST_CHANGES). Rejected for MVP because it requires more complex reviewer setup and doesn't fit the bot-only workflow.

### Why Squash Merge Strategy?

Squash merge produces a clean commit history on the target branch:
- One commit per feature/unit
- Easier to revert if needed
- Cleaner git log for humans
- Matches common open-source project conventions

Alternative considered: Regular merge commits, rebase merge. Squash chosen for cleaner history.

### Why 2-Hour Review Timeout?

Balance between:
- Giving reviewers time to respond
- Not blocking the pipeline indefinitely
- Matching typical async review cycles

The timeout is configurable for teams with different workflows.

### Why 30-Second Poll Interval?

Trade-off between:
- Responsiveness to approval (lower is better)
- API rate limit consumption (higher is better)
- Server load (higher is better)

30 seconds is aggressive enough to catch approvals quickly while staying well under rate limits (5000 requests/hour authenticated).

## Future Enhancements

1. Human reviewer support (request review from specific users)
2. Required checks integration (wait for CI before merge)
3. Auto-assign reviewers based on CODEOWNERS
4. PR templates with unit-specific sections
5. Draft PR support for work-in-progress
6. Review request notifications (Slack, email)

## References

- [PRD Section 4.4: PR Review Loop](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [PRD Section 7: GitHub Integration](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [GitHub Reactions API](https://docs.github.com/en/rest/reactions)
- [GitHub Pull Requests API](https://docs.github.com/en/rest/pulls)
- [gh CLI Documentation](https://cli.github.com/manual/)
