---
task: 1
status: pending
backpressure: "go build ./internal/github/..."
depends_on: []
---

# PR Types

**Parent spec**: `/specs/GITHUB.md`
**Task**: #1 of 5 in implementation plan

## Objective

Define all core types for the GitHub PR lifecycle package: PRClient struct, configuration, review status types, PR info, merge result, and comment types.

## Dependencies

### External Specs (must be implemented)
- None (this is the foundation)

### Task Dependencies (within this unit)
- None (first task)

### Package Dependencies
- `net/http` (standard library)
- `time` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── github/
    ├── client.go    # CREATE: PRClient struct, PRClientConfig
    ├── review.go    # CREATE: ReviewStatus, ReviewState, PollResult
    ├── pr.go        # CREATE: PRInfo, MergeResult
    └── comments.go  # CREATE: PRComment
```

### Types to Implement

```go
// internal/github/client.go

// PRClient manages GitHub PR operations
type PRClient struct {
    httpClient    *http.Client
    owner         string
    repo          string
    pollInterval  time.Duration
    reviewTimeout time.Duration
    // events *events.Bus - added later when events package exists
}

// PRClientConfig holds configuration for the PR client
type PRClientConfig struct {
    Owner         string
    Repo          string
    PollInterval  time.Duration
    ReviewTimeout time.Duration
}
```

```go
// internal/github/review.go

// ReviewStatus represents the current state of PR review
type ReviewStatus string

const (
    ReviewPending          ReviewStatus = "pending"
    ReviewInProgress       ReviewStatus = "in_progress"
    ReviewApproved         ReviewStatus = "approved"
    ReviewChangesRequested ReviewStatus = "changes_requested"
    ReviewTimeout          ReviewStatus = "timeout"
)

// ReviewState holds the parsed review state from PR reactions
type ReviewState struct {
    Status       ReviewStatus
    HasEyes      bool
    HasThumbsUp  bool
    CommentCount int
    LastActivity time.Time
}

// PollResult represents the result of a single poll iteration
type PollResult struct {
    State       ReviewState
    Changed     bool
    ShouldMerge bool
    HasFeedback bool
    TimedOut    bool
}
```

```go
// internal/github/pr.go

// PRInfo holds information about a created PR
type PRInfo struct {
    Number       int
    URL          string
    Branch       string
    TargetBranch string
    Title        string
    CreatedAt    time.Time
}

// MergeResult holds the result of a merge operation
type MergeResult struct {
    Merged  bool
    SHA     string
    Message string
}
```

```go
// internal/github/comments.go

// PRComment represents a review comment on a PR
type PRComment struct {
    ID        int64
    Path      string
    Line      int
    Body      string
    Author    string
    CreatedAt time.Time
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/github/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Compilation | All types compile without errors |
| Type check | `go vet ./internal/github/...` passes |

### Test Fixtures

None required - types only task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- All fields are exported (public) as they need to be accessed by other packages
- `events.Bus` integration deferred until events package exists - leave comment placeholder
- Use `time.Duration` for intervals, not raw integers
- ReviewStatus uses string type for JSON serialization compatibility

## NOT In Scope

- Constructor functions (Task #2)
- API methods (Tasks #3, #4, #5)
- Event emission (deferred until events package)
- HTTP client setup (Task #2)
