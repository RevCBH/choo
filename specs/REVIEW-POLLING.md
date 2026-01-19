# REVIEW-POLLING - PR Review Polling with Emoji-Based State Machine

## Overview

REVIEW-POLLING implements an automated PR review monitoring system using GitHub reactions as state indicators. The system polls PRs for emoji reactions (eyes for "in review", thumbs-up for "approved") and transitions through a state machine that handles the full review lifecycle.

This component bridges the gap between PR creation and merge. Without it, choo creates PRs but has no mechanism to wait for human approval or respond to feedback. The emoji-based protocol provides a lightweight, human-friendly interface that doesn't require complex GitHub webhook infrastructure.

The polling approach uses reactions on the PR itself (not individual comments) to signal high-level review state. Comments without approval reactions indicate change requests, which trigger automated feedback handling through Claude Code delegation.

```
┌─────────────────────────────────────────────────────────────────┐
│                    PR Review State Machine                       │
└─────────────────────────────────────────────────────────────────┘

    ┌──────────┐    eyes reaction    ┌─────────────┐
    │ Pending  │────────────────────▶│ In Review   │
    └──────────┘                     └─────────────┘
         │                                 │
         │ comments                        │ +1 reaction
         │ (no eyes/+1)                    │
         ▼                                 ▼
    ┌──────────┐                     ┌─────────────┐
    │ Changes  │                     │  Approved   │
    │ Requested│                     └─────────────┘
    └──────────┘                           │
         │                                 │
         │ delegate to Claude              │
         │ (commit + push)                 ▼
         └──────────────────────▶    ┌─────────────┐
                                     │   Merge     │
                                     └─────────────┘
```

## Requirements

### Functional Requirements

1. Poll PR reactions every 30 seconds (configurable)
2. Detect review status transitions via emoji reactions:
   - `eyes` (looking eyes) indicates review in progress
   - `+1` (thumbs up) indicates approval
3. Detect change requests via comments without approval reactions
4. Emit events on all status transitions for observability
5. Delegate feedback handling to Claude Code when changes are requested
6. Verify Claude successfully pushed changes after addressing feedback
7. Escalate when Claude cannot address feedback after retries
8. Proceed to merge queue on approval
9. Support configurable review timeout
10. Handle GitHub API rate limits gracefully

### Performance Requirements

| Metric | Target |
|--------|--------|
| Poll interval | 30 seconds (configurable) |
| Review timeout | 2 hours default (configurable) |
| API calls per poll | 2 (reactions + comments) |
| Event emission latency | < 10ms |
| Feedback handling timeout | 10 minutes per attempt |

### Constraints

- Depends on `internal/github` for API interactions
- Depends on `internal/events` for status event emission
- Depends on `internal/worker` for Claude invocation
- Requires valid GitHub token with `repo` scope
- Reactions must be on the PR/issue itself, not on comments
- Only one feedback handling attempt at a time per PR

## Design

### Module Structure

```
internal/github/
├── client.go       # PRClient base, HTTP handling
├── review.go       # Review status polling (exists, extend)
├── comments.go     # PR comment fetching (exists)
└── review_test.go  # Review polling tests

internal/worker/
├── prompt.go       # Task prompts (exists)
├── prompt_git.go   # Feedback prompt builder (new)
├── review.go       # Feedback handling worker (new)
└── review_test.go  # Feedback handling tests
```

### Core Types

```go
// internal/github/review.go

// ReviewStatus represents the current state of PR review
type ReviewStatus string

const (
    ReviewPending          ReviewStatus = "pending"           // No reactions, no comments
    ReviewInProgress       ReviewStatus = "in_progress"       // Has eyes reaction
    ReviewApproved         ReviewStatus = "approved"          // Has +1 reaction
    ReviewChangesRequested ReviewStatus = "changes_requested" // Has comments, no eyes/+1
    ReviewTimeout          ReviewStatus = "timeout"           // Exceeded review timeout
)

// ReviewState holds the parsed review state from PR reactions
type ReviewState struct {
    Status       ReviewStatus // Current computed status
    HasEyes      bool         // Whether eyes reaction present
    HasThumbsUp  bool         // Whether +1 reaction present
    CommentCount int          // Number of unaddressed comments
    LastActivity time.Time    // Most recent reaction/comment time
}

// PollResult represents the result of a single poll iteration
type PollResult struct {
    State       ReviewState // Current state
    Changed     bool        // Whether status changed from previous poll
    ShouldMerge bool        // True if approved and ready to merge
    HasFeedback bool        // True if changes requested
    TimedOut    bool        // True if review timeout exceeded
}

// Reaction represents a GitHub reaction on a PR/issue
type Reaction struct {
    ID        int64     `json:"id"`
    Content   string    `json:"content"`   // "eyes", "+1", "-1", etc.
    CreatedAt time.Time `json:"created_at"`
}

// ReviewPollerConfig holds configuration for the review poller
type ReviewPollerConfig struct {
    PollInterval  time.Duration // Time between polls (default 30s)
    ReviewTimeout time.Duration // Max time to wait for approval (default 2h)
    RequireCI     bool          // Whether to require CI pass before merge
}
```

```go
// internal/worker/review.go

// FeedbackHandler manages responding to PR feedback via Claude
type FeedbackHandler struct {
    github    *github.PRClient
    events    *events.Bus
    claude    ClaudeInvoker
    git       *git.WorktreeManager
    escalator Escalator
    config    FeedbackConfig
}

// FeedbackConfig holds configuration for feedback handling
type FeedbackConfig struct {
    MaxRetries       int           // Max Claude invocation attempts (default 3)
    InvocationTimeout time.Duration // Timeout per Claude invocation (default 10m)
}

// ClaudeInvoker abstracts Claude invocation for testing
type ClaudeInvoker interface {
    Invoke(ctx context.Context, prompt string, workdir string) error
}

// Escalator handles escalation when automated handling fails
type Escalator interface {
    Escalate(ctx context.Context, e Escalation) error
}

// Escalation represents an escalation event
type Escalation struct {
    Severity EscalationSeverity
    Unit     string
    Title    string
    Message  string
    Context  map[string]string
}

// EscalationSeverity indicates urgency level
type EscalationSeverity string

const (
    SeverityInfo     EscalationSeverity = "info"
    SeverityWarning  EscalationSeverity = "warning"
    SeverityBlocking EscalationSeverity = "blocking"
)
```

### API Surface

```go
// internal/github/review.go

// GetReviewStatus fetches the current review status from reactions and comments
func (c *PRClient) GetReviewStatus(ctx context.Context, prNumber int) (*ReviewState, error)

// PollReview polls the PR for review status changes
// Returns when: approval received, feedback found, timeout, or context cancelled
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error)

// WaitForApproval blocks until PR is approved or timeout
// Returns error on timeout or changes requested
func (c *PRClient) WaitForApproval(ctx context.Context, prNumber int, prStartTime time.Time) error

// getReactions fetches reactions on a PR (internal)
func (c *PRClient) getReactions(ctx context.Context, prNumber int) ([]Reaction, error)
```

```go
// internal/worker/prompt_git.go

// BuildFeedbackPrompt constructs the Claude prompt for addressing PR feedback
func BuildFeedbackPrompt(prURL string, comments []github.PRComment) string
```

```go
// internal/worker/review.go

// NewFeedbackHandler creates a new feedback handler
func NewFeedbackHandler(cfg FeedbackConfig, deps FeedbackDeps) *FeedbackHandler

// HandleFeedback addresses PR feedback by delegating to Claude
// Returns nil on success, error if Claude cannot address feedback
func (h *FeedbackHandler) HandleFeedback(ctx context.Context, prNumber int, prURL string, worktreePath string, branch string) error
```

### Status Determination Logic

The review status follows a precedence order:

```go
// Status precedence: approved > in_progress > changes_requested > pending
func determineStatus(hasThumbsUp, hasEyes bool, commentCount int) ReviewStatus {
    if hasThumbsUp {
        return ReviewApproved
    }
    if hasEyes {
        return ReviewInProgress
    }
    if commentCount > 0 {
        return ReviewChangesRequested
    }
    return ReviewPending
}
```

This means:
- A thumbs-up always means approved, even if there are eyes or comments
- Eyes without thumbs-up means review in progress (comments are being addressed)
- Comments without eyes or thumbs-up means changes are requested
- No reactions and no comments means pending initial review

### Event Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ PR Created  │───▶│  Polling    │───▶│   Status    │───▶│   Event     │
│             │    │  (30s tick) │    │   Change?   │    │   Emit      │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                          │                  │
                          │                  │ yes
                          │                  ▼
                          │           ┌─────────────┐
                          │           │  Approved?  │──yes──▶ PRReviewApproved
                          │           └─────────────┘
                          │                  │ no
                          │                  ▼
                          │           ┌─────────────┐
                          │           │  Feedback?  │──yes──▶ PRFeedbackReceived
                          │           └─────────────┘              │
                          │                  │ no                  ▼
                          │                  ▼            ┌─────────────┐
                          │           ┌─────────────┐     │   Claude    │
                          │           │ In Review?  │──▶  │   Handles   │
                          │           └─────────────┘     └─────────────┘
                          │                  │                    │
                          │                  ▼                    ▼
                          │           PRReviewInProgress  PRFeedbackAddressed
                          │                                       │
                          └───────────────────────────────────────┘
                                    (continue polling)
```

## Implementation Notes

### Polling Lifecycle

The polling loop runs as a goroutine with these characteristics:

1. **Ticker-based polling**: Uses `time.Ticker` for consistent 30-second intervals
2. **Context cancellation**: Respects context for graceful shutdown
3. **State diffing**: Only emits events when status actually changes
4. **Terminal conditions**: Exits on approval, timeout, or feedback requiring handling

```go
func (c *PRClient) PollReview(ctx context.Context, prNumber int) (*PollResult, error) {
    ticker := time.NewTicker(c.pollInterval)
    defer ticker.Stop()

    startTime := time.Now()
    var previousState *ReviewState

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-ticker.C:
            state, err := c.GetReviewStatus(ctx, prNumber)
            if err != nil {
                // Log and continue - transient errors shouldn't stop polling
                continue
            }

            result := &PollResult{
                State:   *state,
                Changed: previousState != nil && previousState.Status != state.Status,
            }

            // Check timeout
            if time.Since(startTime) >= c.reviewTimeout {
                result.TimedOut = true
                return result, nil
            }

            // Check terminal conditions
            if state.Status == ReviewApproved {
                result.ShouldMerge = true
                return result, nil
            }

            if state.Status == ReviewChangesRequested {
                result.HasFeedback = true
                return result, nil
            }

            previousState = state
        }
    }
}
```

### Feedback Prompt Construction

The feedback prompt must give Claude sufficient context to address reviewer comments:

```go
// internal/worker/prompt_git.go

func BuildFeedbackPrompt(prURL string, comments []github.PRComment) string {
    var commentText strings.Builder
    for _, c := range comments {
        commentText.WriteString(fmt.Sprintf("- @%s: %s\n", c.Author, c.Body))
        if c.Path != "" {
            commentText.WriteString(fmt.Sprintf("  (on %s:%d)\n", c.Path, c.Line))
        }
    }

    return fmt.Sprintf(`PR %s has received feedback. Please address the following comments:

%s

After making changes:
1. Stage and commit with a message like "address review feedback"
2. Push the changes

The orchestrator will continue polling for approval.`, prURL, commentText.String())
}
```

### Feedback Handling Flow

```go
// internal/worker/review.go

func (h *FeedbackHandler) HandleFeedback(ctx context.Context, prNumber int, prURL string, worktreePath string, branch string) error {
    comments, err := h.github.GetPRComments(ctx, prNumber)
    if err != nil {
        return fmt.Errorf("failed to get PR comments: %w", err)
    }

    prompt := BuildFeedbackPrompt(prURL, comments)

    // Retry loop for Claude invocation
    var lastErr error
    for attempt := 0; attempt < h.config.MaxRetries; attempt++ {
        invokeCtx, cancel := context.WithTimeout(ctx, h.config.InvocationTimeout)
        err := h.claude.Invoke(invokeCtx, prompt, worktreePath)
        cancel()

        if err == nil {
            break
        }
        lastErr = err
    }

    if lastErr != nil {
        h.escalator.Escalate(ctx, Escalation{
            Severity: SeverityBlocking,
            Unit:     "", // Set by caller
            Title:    "Failed to address PR feedback",
            Message:  fmt.Sprintf("Claude could not address feedback after %d attempts", h.config.MaxRetries),
            Context: map[string]string{
                "pr":    prURL,
                "error": lastErr.Error(),
            },
        })
        return lastErr
    }

    // Verify push happened
    cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
    cmd.Dir = worktreePath
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("branch not updated on remote after feedback: %w", err)
    }

    h.events.Emit(events.NewEvent(events.PRFeedbackAddressed, "").WithPR(prNumber))
    return nil
}
```

### Rate Limit Handling

The existing `PRClient.doRequest` handles rate limits with exponential backoff. The polling loop adds resilience by continuing on transient errors:

```go
case <-ticker.C:
    state, err := c.GetReviewStatus(ctx, prNumber)
    if err != nil {
        // Log error but continue polling
        // Rate limits are handled in doRequest with retry
        log.Printf("WARN: poll error (will retry): %v", err)
        continue
    }
```

### GitHub API Endpoints

The implementation uses these GitHub API endpoints:

| Endpoint | Purpose |
|----------|---------|
| `GET /repos/{owner}/{repo}/issues/{number}/reactions` | Fetch PR reactions (PRs are issues) |
| `GET /repos/{owner}/{repo}/pulls/{number}/comments` | Fetch review comments |

Note: Reactions on a PR are accessed via the issues endpoint, not the pulls endpoint.

### Edge Cases

1. **Multiple thumbs-up**: First thumbs-up wins; additional ones are ignored
2. **Eyes then thumbs-up**: Transitions directly to approved (eyes is superceded)
3. **Comments after approval**: Ignored; thumbs-up takes precedence
4. **Reaction removal**: Polled state reflects current state; removal causes transition
5. **Concurrent polling**: Each worker polls its own PR; no coordination needed
6. **Network partitions**: Polling continues through transient failures

### Security Considerations

1. **Token scope**: Requires `repo` scope for reading private repo reactions
2. **Reaction authenticity**: Anyone with repo access can add reactions; consider team restrictions
3. **Comment injection**: Feedback prompts include raw comment text; Claude handles sanitization
4. **Rate limit tokens**: Use separate tokens for high-volume polling if needed

## Testing Strategy

### Unit Tests

```go
// internal/github/review_test.go

func TestGetReviewStatus_ApprovedTakesPrecedence(t *testing.T) {
    // Setup mock server returning both eyes and +1 reactions
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "reactions") {
            json.NewEncoder(w).Encode([]Reaction{
                {ID: 1, Content: "eyes"},
                {ID: 2, Content: "+1"},
            })
            return
        }
        if strings.Contains(r.URL.Path, "comments") {
            json.NewEncoder(w).Encode([]ghReviewComment{})
            return
        }
    }))
    defer server.Close()

    client := newTestClient(t, server.URL)
    state, err := client.GetReviewStatus(context.Background(), 123)

    require.NoError(t, err)
    assert.Equal(t, ReviewApproved, state.Status)
    assert.True(t, state.HasThumbsUp)
    assert.True(t, state.HasEyes)
}

func TestGetReviewStatus_CommentsWithoutReactionsIsChangesRequested(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "reactions") {
            json.NewEncoder(w).Encode([]Reaction{})
            return
        }
        if strings.Contains(r.URL.Path, "comments") {
            json.NewEncoder(w).Encode([]ghReviewComment{
                {ID: 1, Body: "Please fix this", User: ghUser{Login: "reviewer"}},
            })
            return
        }
    }))
    defer server.Close()

    client := newTestClient(t, server.URL)
    state, err := client.GetReviewStatus(context.Background(), 123)

    require.NoError(t, err)
    assert.Equal(t, ReviewChangesRequested, state.Status)
    assert.Equal(t, 1, state.CommentCount)
}

func TestPollReview_ReturnsOnApproval(t *testing.T) {
    pollCount := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "reactions") {
            pollCount++
            if pollCount >= 2 {
                // Second poll returns approval
                json.NewEncoder(w).Encode([]Reaction{{ID: 1, Content: "+1"}})
            } else {
                json.NewEncoder(w).Encode([]Reaction{})
            }
            return
        }
        json.NewEncoder(w).Encode([]ghReviewComment{})
    }))
    defer server.Close()

    client := newTestClient(t, server.URL)
    client.pollInterval = 10 * time.Millisecond

    result, err := client.PollReview(context.Background(), 123)

    require.NoError(t, err)
    assert.True(t, result.ShouldMerge)
    assert.Equal(t, ReviewApproved, result.State.Status)
}

func TestPollReview_ReturnsOnFeedback(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "reactions") {
            json.NewEncoder(w).Encode([]Reaction{})
            return
        }
        if strings.Contains(r.URL.Path, "comments") {
            json.NewEncoder(w).Encode([]ghReviewComment{
                {ID: 1, Body: "Needs work", User: ghUser{Login: "reviewer"}},
            })
            return
        }
    }))
    defer server.Close()

    client := newTestClient(t, server.URL)
    client.pollInterval = 10 * time.Millisecond

    result, err := client.PollReview(context.Background(), 123)

    require.NoError(t, err)
    assert.True(t, result.HasFeedback)
    assert.Equal(t, ReviewChangesRequested, result.State.Status)
}

func TestPollReview_RespectsTimeout(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "reactions") {
            json.NewEncoder(w).Encode([]Reaction{})
            return
        }
        json.NewEncoder(w).Encode([]ghReviewComment{})
    }))
    defer server.Close()

    client := newTestClient(t, server.URL)
    client.pollInterval = 10 * time.Millisecond
    client.reviewTimeout = 50 * time.Millisecond

    result, err := client.PollReview(context.Background(), 123)

    require.NoError(t, err)
    assert.True(t, result.TimedOut)
}
```

```go
// internal/worker/review_test.go

func TestBuildFeedbackPrompt_IncludesAllComments(t *testing.T) {
    comments := []github.PRComment{
        {Author: "alice", Body: "Fix the null check", Path: "main.go", Line: 42},
        {Author: "bob", Body: "Add tests"},
    }

    prompt := BuildFeedbackPrompt("https://github.com/org/repo/pull/123", comments)

    assert.Contains(t, prompt, "@alice: Fix the null check")
    assert.Contains(t, prompt, "(on main.go:42)")
    assert.Contains(t, prompt, "@bob: Add tests")
    assert.Contains(t, prompt, "pull/123")
}

func TestHandleFeedback_RetriesOnFailure(t *testing.T) {
    invokeCount := 0
    mockClaude := &mockClaudeInvoker{
        invokeFunc: func(ctx context.Context, prompt string, workdir string) error {
            invokeCount++
            if invokeCount < 3 {
                return errors.New("transient error")
            }
            return nil
        },
    }

    handler := NewFeedbackHandler(FeedbackConfig{
        MaxRetries:        3,
        InvocationTimeout: time.Second,
    }, FeedbackDeps{
        Claude: mockClaude,
        // ... other deps
    })

    err := handler.HandleFeedback(context.Background(), 123, "https://...", "/tmp/wt", "branch")

    assert.NoError(t, err)
    assert.Equal(t, 3, invokeCount)
}

func TestHandleFeedback_EscalatesAfterMaxRetries(t *testing.T) {
    mockClaude := &mockClaudeInvoker{
        invokeFunc: func(ctx context.Context, prompt string, workdir string) error {
            return errors.New("persistent error")
        },
    }

    var escalated *Escalation
    mockEscalator := &mockEscalator{
        escalateFunc: func(ctx context.Context, e Escalation) error {
            escalated = &e
            return nil
        },
    }

    handler := NewFeedbackHandler(FeedbackConfig{
        MaxRetries:        2,
        InvocationTimeout: time.Second,
    }, FeedbackDeps{
        Claude:    mockClaude,
        Escalator: mockEscalator,
    })

    err := handler.HandleFeedback(context.Background(), 123, "https://...", "/tmp/wt", "branch")

    assert.Error(t, err)
    require.NotNil(t, escalated)
    assert.Equal(t, SeverityBlocking, escalated.Severity)
    assert.Contains(t, escalated.Message, "2 attempts")
}
```

### Integration Tests

1. **Full polling cycle**: Create PR, add eyes reaction, verify InProgress event, add thumbs-up, verify Approved event
2. **Feedback handling**: Create PR with comments, verify feedback detection, mock Claude response, verify push
3. **Timeout behavior**: Start polling with short timeout, verify timeout result
4. **Rate limit recovery**: Trigger rate limit, verify polling continues after backoff

### Manual Testing

- [ ] Create PR and verify it enters pending state
- [ ] Add eyes emoji to PR and verify InProgress status
- [ ] Add +1 emoji and verify Approved status
- [ ] Add comment without emoji and verify ChangesRequested status
- [ ] Verify events appear in orchestrator logs
- [ ] Verify feedback prompt includes all comment details
- [ ] Test timeout by setting short review timeout
- [ ] Verify escalation when Claude fails to address feedback

## Design Decisions

### Why Emoji-Based Protocol?

Emoji reactions provide a simple, visible, human-friendly interface:
- **No webhook infrastructure**: Polling is simpler than managing webhooks
- **Visible state**: Team members can see current review status at a glance
- **Low friction**: Adding a reaction is faster than navigating approval dialogs
- **GitHub-native**: Uses existing GitHub features, no external tooling

Trade-offs:
- Polling introduces latency (up to 30 seconds)
- Anyone with repo access can add reactions
- Must poll both reactions and comments

### Why Poll Instead of Webhooks?

Polling was chosen over webhooks for several reasons:
- **Deployment simplicity**: No need to expose public endpoints
- **Reliability**: No webhook delivery failures to handle
- **State recovery**: Can reconstruct state on restart by polling
- **Rate limit friendly**: 2 API calls per 30 seconds is well within limits

Trade-offs:
- Up to 30-second latency for status updates
- Continuous API usage even when no changes occur

### Why Delegate to Claude for Feedback?

Claude Code handles feedback because:
- **Context awareness**: Claude understands the codebase and original task
- **Intelligent fixes**: Can interpret reviewer intent, not just literal changes
- **Consistent style**: Maintains coding style from original implementation

Trade-offs:
- Adds Claude invocation cost
- Requires retry/escalation logic for failures
- May make changes that don't match reviewer expectations

## Future Enhancements

1. **Webhook support**: Optional webhook receiver for instant status updates
2. **Comment threading**: Track which comments have been addressed
3. **Approval requirements**: Require specific reviewers or number of approvals
4. **CI integration**: Wait for CI pass before allowing merge
5. **Review assignment**: Auto-request reviews from CODEOWNERS
6. **Feedback classification**: Categorize comments (style, logic, tests) for prioritization
7. **Review metrics**: Track time-to-approval, feedback rounds per PR

## References

- Self-Hosting PRD Section 6: PR Review Polling
- [GitHub Reactions API](https://docs.github.com/en/rest/reactions)
- [GitHub Pull Request Comments API](https://docs.github.com/en/rest/pulls/comments)
- Existing implementation: `internal/github/review.go`
- Existing implementation: `internal/events/types.go` (PR event types)
