# Choo Self-Hosting - Product Requirements Document

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-19 |
| Target  | v0.2       |

---

## 1. Overview

### 1.1 Goal

Enable choo to develop itself by closing the remaining gaps between the implemented MVP components and a fully operational orchestration loop.

### 1.2 Current State

The v0.1 implementation completed these components:

| Component | Status | Notes |
|-----------|--------|-------|
| Discovery | Complete | Parses specs/tasks/, extracts dependencies |
| Scheduler | Complete | DAG, topological sort, ready queue, state machine |
| Worker | Complete | Task loop, Claude CLI invocation, backpressure |
| Git | Complete | Worktrees, branches, commits, merges |
| GitHub | Partial | PR fetch/update/merge, but no creation or polling |
| Events | Complete | 30+ event types, pub/sub bus |
| CLI | Partial | Commands exist, orchestrator loop not wired |
| Config | Complete | YAML config, env overrides, validation |

### 1.3 Gaps for Self-Hosting

```
┌─────────────────────────────────────────────────────────────────┐
│                     SELF-HOSTING GAPS                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐       │
│  │  CRITICAL   │     │    CORE     │     │   INFRA     │       │
│  ├─────────────┤     ├─────────────┤     ├─────────────┤       │
│  │ Orchestrator│     │ Claude Git  │     │ CI/Actions  │       │
│  │   Wiring    │     │ Delegation  │     │             │       │
│  │   (~150 LOC)│     │  (~100 LOC) │     │  (~200 LOC) │       │
│  └─────────────┘     ├─────────────┤     ├─────────────┤       │
│                      │ PR Review   │     │ Escalation  │       │
│  ┌─────────────┐     │  Polling    │     │  Interface  │       │
│  │  Escalation │     │  (~150 LOC) │     │  (~150 LOC) │       │
│  │  Interface  │     ├─────────────┤     └─────────────┘       │
│  │   (~100 LOC)│     │  Conflict   │                           │
│  └─────────────┘     │ Resolution  │                           │
│                      │  (~100 LOC) │                           │
│                      └─────────────┘                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 1.4 Success Criteria

Choo can:
1. Discover its own specs in `specs/tasks/`
2. Execute tasks in parallel worktrees
3. Delegate commit, push, and PR creation to Claude Code
4. Poll for review approval
5. Handle merge conflicts via Claude Code
6. Merge approved PRs
7. Escalate to user when Claude cannot complete an operation

---

## 2. Design Philosophy: Claude Code Delegation

### 2.1 Principle

Claude Code is already trusted with the hard work—implementing tasks, resolving conflicts, fixing baseline failures. Git operations (commit, push, PR) are simpler by comparison. **The orchestrator should coordinate, not execute.**

### 2.2 Current vs. Proposed

| Operation | Current | Proposed |
|-----------|---------|----------|
| File editing | Claude Code | Claude Code (unchanged) |
| Staging | Orchestrator | Claude Code |
| Commit message | Template | Claude Code writes |
| Git push | Orchestrator | Claude Code |
| PR creation | Direct `gh` call | Claude Code via `gh` |
| PR description | Template | Claude Code writes |
| Conflict resolution | Claude Code | Claude Code (unchanged) |
| PR merge | GitHub API | GitHub API (unchanged) |

### 2.3 Benefits

1. **Rich commit messages** — Claude understands *why* changes were made
2. **Contextual PR descriptions** — Summarizes implementation approach
3. **Simpler orchestrator** — Less Go code to maintain
4. **Natural workflow** — Mirrors how a human developer works
5. **Consistent delegation model** — Claude handles all creative work

### 2.4 Verification Pattern

The orchestrator verifies outcomes rather than controlling steps:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Invoke    │────▶│   Verify    │────▶│  Continue   │
│   Claude    │     │   Outcome   │     │  or Retry   │
└─────────────┘     └─────────────┘     └─────────────┘
                          │
                          │ max retries exceeded
                          ▼
                    ┌─────────────┐
                    │  Escalate   │
                    │  to User    │
                    └─────────────┘
```

---

## 3. Gap 1: Escalation Interface

### 3.1 Problem

When Claude cannot complete an operation after retries, choo has no way to alert the user. The system needs a clean abstraction for escalation that can support multiple backends (terminal, Slack, email, webhook, etc.).

### 3.2 Design

```
┌─────────────────────────────────────────────────────────────────┐
│                     Escalation Interface                         │
└─────────────────────────────────────────────────────────────────┘

                         Escalator
                             │
            ┌────────────────┼────────────────┐
            ▼                ▼                ▼
     ┌───────────┐    ┌───────────┐    ┌───────────┐
     │  Terminal │    │   Slack   │    │  Webhook  │
     │ Escalator │    │ Escalator │    │ Escalator │
     └───────────┘    └───────────┘    └───────────┘
```

### 3.3 Implementation

```go
// internal/escalate/escalate.go

package escalate

import "context"

// Severity indicates how urgent the escalation is
type Severity string

const (
    SeverityInfo     Severity = "info"     // FYI, no action needed
    SeverityWarning  Severity = "warning"  // May need attention
    SeverityCritical Severity = "critical" // Requires immediate action
    SeverityBlocking Severity = "blocking" // Cannot proceed without user
)

// Escalation represents something that needs user attention
type Escalation struct {
    Severity Severity
    Unit     string            // Which unit is affected
    Title    string            // Short summary
    Message  string            // Detailed explanation
    Context  map[string]string // Additional context (PR URL, error, etc.)
}

// Escalator is the interface for notifying users
type Escalator interface {
    // Escalate sends a notification to the user
    // Returns nil if notification was sent successfully
    Escalate(ctx context.Context, e Escalation) error

    // Name returns the escalator type for logging
    Name() string
}

// Multi wraps multiple escalators and fans out to all of them
type Multi struct {
    escalators []Escalator
}

func NewMulti(escalators ...Escalator) *Multi {
    return &Multi{escalators: escalators}
}

func (m *Multi) Escalate(ctx context.Context, e Escalation) error {
    var firstErr error
    for _, esc := range m.escalators {
        if err := esc.Escalate(ctx, e); err != nil && firstErr == nil {
            firstErr = err
        }
    }
    return firstErr
}

func (m *Multi) Name() string {
    return "multi"
}
```

### 3.4 Terminal Escalator (Default)

```go
// internal/escalate/terminal.go

package escalate

import (
    "context"
    "fmt"
    "os"
)

type Terminal struct{}

func NewTerminal() *Terminal {
    return &Terminal{}
}

func (t *Terminal) Escalate(ctx context.Context, e Escalation) error {
    prefix := ""
    switch e.Severity {
    case SeverityCritical, SeverityBlocking:
        prefix = "🚨 "
    case SeverityWarning:
        prefix = "⚠️  "
    default:
        prefix = "ℹ️  "
    }

    fmt.Fprintf(os.Stderr, "\n%s[%s] %s\n", prefix, e.Severity, e.Title)
    fmt.Fprintf(os.Stderr, "   Unit: %s\n", e.Unit)
    fmt.Fprintf(os.Stderr, "   %s\n", e.Message)

    for k, v := range e.Context {
        fmt.Fprintf(os.Stderr, "   %s: %s\n", k, v)
    }

    return nil
}

func (t *Terminal) Name() string {
    return "terminal"
}
```

### 3.5 Slack Escalator (Optional)

```go
// internal/escalate/slack.go

package escalate

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type Slack struct {
    webhookURL string
    client     *http.Client
}

func NewSlack(webhookURL string) *Slack {
    return &Slack{
        webhookURL: webhookURL,
        client:     &http.Client{},
    }
}

func (s *Slack) Escalate(ctx context.Context, e Escalation) error {
    emoji := map[Severity]string{
        SeverityInfo:     ":information_source:",
        SeverityWarning:  ":warning:",
        SeverityCritical: ":rotating_light:",
        SeverityBlocking: ":octagonal_sign:",
    }[e.Severity]

    payload := map[string]any{
        "text": fmt.Sprintf("%s *[%s]* %s", emoji, e.Unit, e.Title),
        "blocks": []map[string]any{
            {
                "type": "section",
                "text": map[string]string{
                    "type": "mrkdwn",
                    "text": fmt.Sprintf("*%s*\n%s", e.Title, e.Message),
                },
            },
        },
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
    }
    return nil
}

func (s *Slack) Name() string {
    return "slack"
}
```

### 3.6 Acceptance Criteria

- [ ] `Escalator` interface defined with `Escalate` method
- [ ] `Terminal` escalator prints to stderr with severity indicators
- [ ] `Slack` escalator posts to webhook URL
- [ ] `Multi` escalator fans out to multiple backends
- [ ] Escalation includes unit, title, message, and context map

---

## 4. Gap 2: Orchestrator Wiring

### 4.1 Problem

The `choo run` command parses flags but has TODO placeholders where the orchestration loop should be:

```go
// internal/cli/run.go:118-120
// TODO: Wire orchestrator components (task #9)
// TODO: Run discovery (task #9)
// TODO: Execute scheduler loop (task #9)
```

All components exist but aren't connected.

### 4.2 Design

```
┌─────────────────────────────────────────────────────────────────┐
│                      Orchestrator Loop                          │
└─────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   ┌───────────┐        ┌───────────┐        ┌───────────┐
   │ Discovery │───────▶│ Scheduler │───────▶│  Worker   │
   │           │        │           │        │   Pool    │
   └───────────┘        └───────────┘        └───────────┘
         │                    │                    │
         │                    │                    │
         ▼                    ▼                    ▼
   ┌─────────────────────────────────────────────────────┐
   │                     Event Bus                        │
   └─────────────────────────────────────────────────────┘
                              │
                              ▼
   ┌─────────────────────────────────────────────────────┐
   │                    Escalator                         │
   └─────────────────────────────────────────────────────┘
```

### 4.3 Implementation

```go
// internal/orchestrator/orchestrator.go

type Orchestrator struct {
    cfg       *config.Config
    bus       *events.Bus
    escalator escalate.Escalator
    scheduler *scheduler.Scheduler
    pool      *worker.Pool
}

func New(cfg *config.Config, bus *events.Bus, esc escalate.Escalator) *Orchestrator {
    return &Orchestrator{
        cfg:       cfg,
        bus:       bus,
        escalator: esc,
    }
}

func (o *Orchestrator) Run(ctx context.Context, specsDir string) error {
    // 1. Discovery
    units, err := discovery.Discover(specsDir)
    if err != nil {
        return fmt.Errorf("discovery failed: %w", err)
    }

    // 2. Build scheduler
    o.scheduler = scheduler.New(units)
    if err := o.scheduler.Build(); err != nil {
        return fmt.Errorf("scheduler build failed: %w", err)
    }

    // 3. Initialize worker pool
    o.pool = worker.NewPool(o.cfg.Parallelism)

    // 4. Main loop
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        ready := o.scheduler.Ready()
        if len(ready) == 0 {
            if o.scheduler.AllComplete() {
                o.bus.Emit(events.NewEvent(events.OrchCompleted, ""))
                return nil
            }
            if o.scheduler.AllBlockedOrFailed() {
                return fmt.Errorf("all units blocked or failed")
            }
            time.Sleep(100 * time.Millisecond)
            continue
        }

        for _, unit := range ready {
            o.scheduler.MarkInProgress(unit.ID)
            o.pool.Submit(func() error {
                return o.executeUnit(ctx, unit)
            })
        }
    }
}

func (o *Orchestrator) executeUnit(ctx context.Context, unit *discovery.Unit) error {
    w := worker.New(unit, o.cfg, o.bus, o.escalator)
    err := w.Execute(ctx)

    if err != nil {
        o.scheduler.MarkFailed(unit.ID)
        o.bus.Emit(events.NewEvent(events.UnitFailed, unit.ID).WithError(err))
    } else {
        o.scheduler.MarkComplete(unit.ID)
        o.bus.Emit(events.NewEvent(events.UnitCompleted, unit.ID))
    }

    return err
}
```

### 4.4 Acceptance Criteria

- [ ] `choo run specs/tasks/` discovers units and executes them
- [ ] Parallelism flag controls concurrent workers
- [ ] Events emit for unit lifecycle
- [ ] Escalator injected into workers
- [ ] Graceful shutdown on SIGINT

---

## 5. Gap 3: Claude Git Delegation

### 5.1 Problem

Currently the orchestrator controls git operations:
- Orchestrator commits with templated message
- Orchestrator pushes branch
- Orchestrator calls `gh pr create` directly

This loses Claude's contextual understanding and produces generic commit/PR messages.

### 5.2 Design: Full Delegation

Claude Code handles the complete git workflow after task completion:

```
┌─────────────────────────────────────────────────────────────────┐
│                  Claude Git Delegation Flow                      │
└─────────────────────────────────────────────────────────────────┘

  Task Complete
       │
       ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Invoke    │────▶│   Verify    │────▶│   Invoke    │
│   Claude:   │     │   Commit    │     │   Claude:   │
│   Commit    │     │   Exists    │     │   Push      │
└─────────────┘     └─────────────┘     └─────────────┘
                          │                    │
                    retry with                 │
                    backoff                    ▼
                          │             ┌─────────────┐
                          │             │   Verify    │
                          ▼             │   Branch    │
                    ┌───────────┐       │   Pushed    │
                    │ Escalate  │       └─────────────┘
                    │ to User   │             │
                    └───────────┘             │
                                              ▼
                                   (After all tasks)
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Invoke    │
                                 │   Claude:   │
                                 │   Create PR │
                                 └─────────────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Verify    │
                                 │   PR URL    │
                                 └─────────────┘
```

### 5.3 Implementation

#### 5.3.1 Prompt Builder

```go
// internal/worker/prompt_git.go

package worker

import "fmt"

// BuildCommitPrompt creates a prompt for Claude to commit changes
func BuildCommitPrompt(taskTitle string, files []string) string {
    return fmt.Sprintf(`Task "%s" is complete.

Stage and commit the changes:
1. Run: git add -A
2. Run: git commit with a conventional commit message

Guidelines for the commit message:
- Use conventional commit format (feat:, fix:, refactor:, etc.)
- First line: concise summary of what changed (50 chars or less)
- If needed, add a blank line then detailed explanation
- Explain WHY, not just WHAT

Files changed:
%s

Do NOT push yet. Just stage and commit.`, taskTitle, formatFileList(files))
}

// BuildPushPrompt creates a prompt for Claude to push the branch
func BuildPushPrompt(branch string) string {
    return fmt.Sprintf(`Push the branch to origin:

git push -u origin %s

If the push fails due to a transient error (network, etc.), that's okay -
the orchestrator will retry. Just attempt the push.`, branch)
}

// BuildPRPrompt creates a prompt for Claude to create a PR
func BuildPRPrompt(branch, targetBranch, unitTitle string) string {
    return fmt.Sprintf(`All tasks for unit "%s" are complete.

Create a pull request:
- Source branch: %s
- Target branch: %s

Use the gh CLI:
  gh pr create --base %s --head %s --title "..." --body "..."

Guidelines for the PR:
- Title: Clear, concise summary of the unit's purpose
- Body:
  - Brief overview of what was implemented
  - Key changes or decisions made
  - Any notes for reviewers

Print the PR URL when done so the orchestrator can capture it.`, unitTitle, branch, targetBranch, targetBranch, branch)
}

func formatFileList(files []string) string {
    if len(files) == 0 {
        return "(no files listed)"
    }
    result := ""
    for _, f := range files {
        result += "- " + f + "\n"
    }
    return result
}
```

#### 5.3.2 Retry with Backoff

```go
// internal/worker/retry.go

package worker

import (
    "context"
    "math"
    "time"
)

// RetryConfig controls retry behavior
type RetryConfig struct {
    MaxAttempts     int
    InitialBackoff  time.Duration
    MaxBackoff      time.Duration
    BackoffMultiply float64
}

var DefaultRetryConfig = RetryConfig{
    MaxAttempts:     3,
    InitialBackoff:  1 * time.Second,
    MaxBackoff:      30 * time.Second,
    BackoffMultiply: 2.0,
}

// RetryResult indicates what happened
type RetryResult struct {
    Success  bool
    Attempts int
    LastErr  error
}

// RetryWithBackoff retries an operation with exponential backoff
// It retries on ANY error - the assumption is that Claude failures
// are transient (network, rate limits, etc.)
func RetryWithBackoff(
    ctx context.Context,
    cfg RetryConfig,
    operation func(ctx context.Context) error,
) RetryResult {
    var lastErr error
    backoff := cfg.InitialBackoff

    for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
        err := operation(ctx)
        if err == nil {
            return RetryResult{Success: true, Attempts: attempt}
        }

        lastErr = err

        if attempt < cfg.MaxAttempts {
            select {
            case <-ctx.Done():
                return RetryResult{Success: false, Attempts: attempt, LastErr: ctx.Err()}
            case <-time.After(backoff):
            }

            // Exponential backoff
            backoff = time.Duration(float64(backoff) * cfg.BackoffMultiply)
            if backoff > cfg.MaxBackoff {
                backoff = cfg.MaxBackoff
            }
        }
    }

    return RetryResult{Success: false, Attempts: cfg.MaxAttempts, LastErr: lastErr}
}
```

#### 5.3.3 Worker Integration

```go
// internal/worker/git_delegate.go

package worker

import (
    "context"
    "fmt"
    "regexp"
    "strings"

    "choo/internal/escalate"
)

// commitViaClaudeCode invokes Claude to stage and commit
func (w *Worker) commitViaClaudeCode(ctx context.Context, taskTitle string) error {
    files, _ := w.git.GetChangedFiles(ctx)
    prompt := BuildCommitPrompt(taskTitle, files)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify commit was created
        hasCommit, err := w.git.HasNewCommit(ctx)
        if err != nil {
            return err
        }
        if !hasCommit {
            return fmt.Errorf("claude did not create a commit")
        }
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to commit changes",
            Message:  fmt.Sprintf("Claude could not commit after %d attempts", result.Attempts),
            Context: map[string]string{
                "task":  taskTitle,
                "error": result.LastErr.Error(),
            },
        })
        return result.LastErr
    }

    return nil
}

// pushViaClaudeCode invokes Claude to push the branch
func (w *Worker) pushViaClaudeCode(ctx context.Context) error {
    prompt := BuildPushPrompt(w.branch)

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify branch exists on remote
        exists, err := w.git.BranchExistsOnRemote(ctx, w.branch)
        if err != nil {
            return err
        }
        if !exists {
            return fmt.Errorf("branch not found on remote after push")
        }
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to push branch",
            Message:  fmt.Sprintf("Claude could not push after %d attempts", result.Attempts),
            Context: map[string]string{
                "branch": w.branch,
                "error":  result.LastErr.Error(),
            },
        })
        return result.LastErr
    }

    w.bus.Emit(events.NewEvent(events.BranchPushed, w.unit.ID).WithPayload(map[string]string{
        "branch": w.branch,
    }))

    return nil
}

// createPRViaClaudeCode invokes Claude to create the PR
func (w *Worker) createPRViaClaudeCode(ctx context.Context) (string, error) {
    prompt := BuildPRPrompt(w.branch, w.cfg.TargetBranch, w.unit.Title)

    var prURL string

    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        output, err := w.invokeClaudeWithOutput(ctx, prompt)
        if err != nil {
            return err
        }

        // Extract PR URL from output
        url := extractPRURL(output)
        if url == "" {
            return fmt.Errorf("could not find PR URL in claude output")
        }

        prURL = url
        return nil
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to create PR",
            Message:  fmt.Sprintf("Claude could not create PR after %d attempts", result.Attempts),
            Context: map[string]string{
                "branch": w.branch,
                "target": w.cfg.TargetBranch,
                "error":  result.LastErr.Error(),
            },
        })
        return "", result.LastErr
    }

    return prURL, nil
}

var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

func extractPRURL(output string) string {
    match := prURLPattern.FindString(output)
    return match
}
```

### 5.4 Acceptance Criteria

- [ ] Claude writes commit messages based on actual changes
- [ ] Claude pushes branches via `git push`
- [ ] Claude creates PRs via `gh pr create` with contextual descriptions
- [ ] Retry with exponential backoff on transient failures
- [ ] Escalate to user after max retries (no fallback to direct operations)
- [ ] PR URL captured from Claude's output

---

## 6. Gap 4: PR Review Polling

### 6.1 Problem

PRs are created but choo doesn't wait for approval. The review emoji state machine is designed but not implemented.

### 6.2 Design

```
┌─────────────────────────────────────────────────────────────────┐
│                    PR Review State Machine                       │
└─────────────────────────────────────────────────────────────────┘

    ┌──────────┐    👀 reaction    ┌─────────────┐
    │ Pending  │──────────────────▶│ In Review   │
    └──────────┘                   └─────────────┘
         │                               │
         │ comments                      │ 👍 reaction
         │ (no 👀/👍)                    │
         ▼                               ▼
    ┌──────────┐                   ┌─────────────┐
    │ Changes  │                   │  Approved   │
    │ Requested│                   └─────────────┘
    └──────────┘                         │
         │                               │
         │ delegate to Claude            │
         │                               ▼
         └──────────────────────▶  ┌─────────────┐
                                   │   Merge     │
                                   └─────────────┘
```

### 6.3 Implementation

```go
// internal/github/review.go

type ReviewStatus string

const (
    ReviewPending    ReviewStatus = "pending"
    ReviewInProgress ReviewStatus = "in_progress"
    ReviewApproved   ReviewStatus = "approved"
    ReviewChanges    ReviewStatus = "changes_requested"
)

func (c *Client) PollReview(ctx context.Context, prNumber int) (<-chan ReviewStatus, error) {
    ch := make(chan ReviewStatus)

    go func() {
        defer close(ch)
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        var lastStatus ReviewStatus
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                status, err := c.GetReviewStatus(prNumber)
                if err != nil {
                    continue // retry on error
                }

                if status != lastStatus {
                    ch <- status
                    lastStatus = status
                }

                if status == ReviewApproved {
                    return
                }
            }
        }
    }()

    return ch, nil
}

func (c *Client) GetReviewStatus(prNumber int) (ReviewStatus, error) {
    reactions, err := c.getReactions(prNumber)
    if err != nil {
        return "", err
    }

    hasEyes := false
    hasThumbsUp := false
    for _, r := range reactions {
        switch r.Content {
        case "eyes":
            hasEyes = true
        case "+1":
            hasThumbsUp = true
        }
    }

    if hasThumbsUp {
        return ReviewApproved, nil
    }
    if hasEyes {
        return ReviewInProgress, nil
    }

    comments, err := c.getPRComments(prNumber)
    if err != nil {
        return "", err
    }

    if len(comments) > 0 {
        return ReviewChanges, nil
    }

    return ReviewPending, nil
}
```

### 6.4 Feedback Handling via Claude

When status is `ReviewChanges`, delegate feedback response to Claude:

```go
// internal/worker/prompt_git.go

func BuildFeedbackPrompt(prURL string, comments []github.Comment) string {
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

```go
// internal/worker/review.go

func (w *Worker) handleFeedback(ctx context.Context, prNumber int, prURL string) error {
    comments, err := w.github.GetPRComments(prNumber)
    if err != nil {
        return err
    }

    prompt := BuildFeedbackPrompt(prURL, comments)

    // Delegate to Claude to address feedback
    result := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        return w.invokeClaude(ctx, prompt)
    })

    if !result.Success {
        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to address PR feedback",
            Message:  fmt.Sprintf("Claude could not address feedback after %d attempts", result.Attempts),
            Context: map[string]string{
                "pr":    prURL,
                "error": result.LastErr.Error(),
            },
        })
        return result.LastErr
    }

    // Claude should have committed and pushed
    // Verify push happened
    if _, err := w.git.BranchExistsOnRemote(ctx, w.branch); err != nil {
        return fmt.Errorf("branch not updated on remote after feedback: %w", err)
    }

    w.bus.Emit(events.NewEvent(events.PRFeedbackAddressed, w.unit.ID).WithPR(prNumber))
    return nil
}
```

### 6.5 Acceptance Criteria

- [ ] Poll PR reactions every 30 seconds
- [ ] Emit events on status transitions
- [ ] Delegate feedback response to Claude (commit + push)
- [ ] Escalate if Claude cannot address feedback
- [ ] Proceed to merge on approval

---

## 7. Gap 5: Conflict Resolution

### 7.1 Problem

When multiple units merge to main, later merges may conflict. The system needs to detect and delegate conflict resolution to Claude.

### 7.2 Design

```
┌─────────────────────────────────────────────────────────────────┐
│                   Merge Conflict Flow                           │
└─────────────────────────────────────────────────────────────────┘

  ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
  │  Fetch  │────▶│ Rebase  │────▶│Conflict?│────▶│  Merge  │
  │ Latest  │     │         │     │         │     │         │
  └─────────┘     └─────────┘     └─────────┘     └─────────┘
                                       │
                                       │ Yes
                                       ▼
                                 ┌───────────┐
                                 │  Invoke   │
                                 │  Claude   │
                                 │  to Fix   │
                                 └───────────┘
                                       │
                                       ▼
                                 ┌───────────┐
                                 │  Verify   │
                                 │ Resolved  │
                                 └───────────┘
                                       │
                          ┌────────────┴────────────┐
                          │                         │
                          ▼                         ▼
                    ┌───────────┐            ┌───────────┐
                    │   Force   │            │ Escalate  │
                    │   Push    │            │ to User   │
                    └───────────┘            └───────────┘
```

### 7.3 Implementation

```go
// internal/worker/prompt_git.go

func BuildConflictPrompt(targetBranch string, conflictedFiles []string) string {
    return fmt.Sprintf(`The rebase onto %s resulted in merge conflicts.

Conflicted files:
%s

Please resolve all conflicts:
1. Open each conflicted file
2. Find the conflict markers (<<<<<<, =======, >>>>>>>)
3. Edit to resolve - keep the correct code, remove markers
4. Stage resolved files: git add <file>
5. Continue the rebase: git rebase --continue

If the rebase continues successfully, do NOT push - the orchestrator will handle that.

If you cannot resolve a conflict, explain why in your response.`, targetBranch, formatFileList(conflictedFiles))
}
```

```go
// internal/worker/merge.go

func (w *Worker) mergeWithConflictResolution(ctx context.Context) error {
    // Fetch latest
    if err := w.git.Fetch(ctx); err != nil {
        return fmt.Errorf("fetch failed: %w", err)
    }

    // Try rebase
    result, err := w.git.RebaseOnto(ctx, w.cfg.TargetBranch)
    if err != nil {
        return fmt.Errorf("rebase failed: %w", err)
    }

    if !result.HasConflicts {
        // No conflicts, force push and merge
        return w.forcePushAndMerge(ctx)
    }

    // Emit conflict event
    w.bus.Emit(events.NewEvent(events.PRConflict, w.unit.ID).WithPR(w.prNumber).WithPayload(map[string]any{
        "files": result.Files,
    }))

    // Delegate conflict resolution to Claude
    prompt := BuildConflictPrompt(w.cfg.TargetBranch, result.Files)

    retryResult := RetryWithBackoff(ctx, DefaultRetryConfig, func(ctx context.Context) error {
        if err := w.invokeClaude(ctx, prompt); err != nil {
            return err
        }

        // Verify rebase completed (no longer in rebase state)
        inRebase, err := w.git.IsRebaseInProgress(ctx)
        if err != nil {
            return err
        }
        if inRebase {
            // Claude didn't complete the rebase
            w.git.AbortRebase(ctx)
            return fmt.Errorf("claude did not complete rebase")
        }
        return nil
    })

    if !retryResult.Success {
        w.git.AbortRebase(ctx) // Clean up

        w.escalator.Escalate(ctx, escalate.Escalation{
            Severity: escalate.SeverityBlocking,
            Unit:     w.unit.ID,
            Title:    "Failed to resolve merge conflicts",
            Message:  fmt.Sprintf("Claude could not resolve conflicts after %d attempts", retryResult.Attempts),
            Context: map[string]string{
                "files":  strings.Join(result.Files, ", "),
                "target": w.cfg.TargetBranch,
                "error":  retryResult.LastErr.Error(),
            },
        })
        return retryResult.LastErr
    }

    return w.forcePushAndMerge(ctx)
}

func (w *Worker) forcePushAndMerge(ctx context.Context) error {
    // Force push the rebased branch
    if err := w.git.ForcePushWithLease(ctx, w.branch); err != nil {
        return fmt.Errorf("force push failed: %w", err)
    }

    // Merge via GitHub API
    if err := w.github.Merge(w.prNumber); err != nil {
        return fmt.Errorf("merge failed: %w", err)
    }

    w.bus.Emit(events.NewEvent(events.PRMerged, w.unit.ID).WithPR(w.prNumber))
    return nil
}
```

### 7.4 Acceptance Criteria

- [ ] Detect merge conflicts during rebase
- [ ] Delegate conflict resolution to Claude (not orchestrator)
- [ ] Verify rebase completed before continuing
- [ ] Retry with backoff on failure
- [ ] Escalate to user after max retries (no fallback)
- [ ] Force push with lease after successful resolution
- [ ] Emit PRConflict and PRMerged events

---

## 8. Gap 6: CI Integration

### 8.1 Problem

No automated testing on PRs. Self-hosting requires confidence that changes don't break the build.

### 8.2 Design

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build
        run: go build -v ./...

      - name: Test
        run: go test -v -race -coverprofile=coverage.txt ./...

      - name: Vet
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

### 8.3 Status Check Integration

```go
// internal/github/checks.go

type CheckStatus string

const (
    CheckPending CheckStatus = "pending"
    CheckSuccess CheckStatus = "success"
    CheckFailure CheckStatus = "failure"
)

func (c *Client) GetCheckStatus(ctx context.Context, ref string) (CheckStatus, error) {
    runs, err := c.getCheckRuns(ref)
    if err != nil {
        return "", err
    }

    allComplete := true
    anyFailed := false

    for _, run := range runs {
        if run.Status != "completed" {
            allComplete = false
        }
        if run.Conclusion == "failure" {
            anyFailed = true
        }
    }

    if anyFailed {
        return CheckFailure, nil
    }
    if !allComplete {
        return CheckPending, nil
    }
    return CheckSuccess, nil
}
```

### 8.4 Acceptance Criteria

- [ ] CI runs on all PRs
- [ ] Tests, vet, and lint all pass
- [ ] Optional: block merge until CI passes

---

## 9. Implementation Plan

### 9.1 Unit Breakdown

| Unit | Tasks | Est. LOC | Dependencies |
|------|-------|----------|--------------|
| escalation | Interface, Terminal, Slack, Multi | 150 | - |
| orchestrator | Wire components, main loop | 150 | escalation |
| claude-git | Prompts, delegation, verification | 200 | escalation |
| review-polling | Poll loop, feedback delegation | 150 | claude-git |
| conflict-resolution | Rebase, delegation, verification | 150 | claude-git |
| ci | GitHub Actions workflow | 100 | - |

### 9.2 Task Sequence

```
         ┌─────────────┐
         │ escalation  │
         └──────┬──────┘
                │
         ┌──────┴──────┐
         ▼             ▼
   ┌───────────┐ ┌───────────┐
   │orchestrator│ │claude-git │
   └─────┬─────┘ └─────┬─────┘
         │             │
         │      ┌──────┴──────┐
         │      ▼             ▼
         │ ┌────────────┐ ┌────────────┐
         │ │  review-   │ │ conflict-  │
         │ │  polling   │ │ resolution │
         │ └────────────┘ └────────────┘
         │
         └─────────────────────┐
                               ▼
                          ┌────────┐
                          │   ci   │
                          └────────┘
```

### 9.3 Spec File Structure

```
specs/
├── README.md
├── completed/           # v0.1 archived specs
└── tasks/
    ├── escalation/
    │   ├── IMPLEMENTATION_PLAN.md
    │   ├── 01-interface.md
    │   ├── 02-terminal.md
    │   └── 03-multi.md
    ├── orchestrator/
    │   ├── IMPLEMENTATION_PLAN.md
    │   ├── 01-orchestrator-type.md
    │   ├── 02-main-loop.md
    │   └── 03-wire-cli.md
    ├── claude-git/
    │   ├── IMPLEMENTATION_PLAN.md
    │   ├── 01-prompts.md
    │   ├── 02-commit-delegate.md
    │   ├── 03-push-delegate.md
    │   ├── 04-pr-delegate.md
    │   └── 05-verification.md
    ├── review-polling/
    │   ├── IMPLEMENTATION_PLAN.md
    │   ├── 01-poll-loop.md
    │   ├── 02-status-detection.md
    │   └── 03-feedback-delegate.md
    ├── conflict-resolution/
    │   ├── IMPLEMENTATION_PLAN.md
    │   ├── 01-detect-conflicts.md
    │   ├── 02-resolution-delegate.md
    │   └── 03-verification.md
    └── ci/
        ├── IMPLEMENTATION_PLAN.md
        └── 01-github-actions.md
```

---

## 10. Configuration Additions

Add to `.choo.yaml`:

```yaml
# Escalation configuration
escalation:
  backends:
    - terminal                    # Always enabled
    - slack                       # Optional
  slack_webhook: ""               # Set via CHOO_SLACK_WEBHOOK env var

# Retry configuration
retry:
  max_attempts: 3
  initial_backoff: 1s
  max_backoff: 30s
  backoff_multiply: 2.0

# Review configuration
review:
  poll_interval: 30s
  require_ci: true

# Merge configuration
merge:
  strategy: squash                # squash | merge | rebase
```

---

## 11. Testing Strategy

### 11.1 Unit Tests

| Package | Tests |
|---------|-------|
| escalate | Interface compliance, Multi fan-out |
| orchestrator | Loop termination, dispatch ordering |
| worker | Prompt generation, retry logic, verification |
| github | Review status parsing, check status |

### 11.2 Integration Tests

| Scenario | Description |
|----------|-------------|
| Happy path | Single unit: discover → execute → commit → push → PR → merge |
| Commit failure | Mock Claude failure, verify escalation |
| Push failure | Network failure, verify retry then escalation |
| PR feedback | Mock comments, verify Claude delegation |
| Conflict resolution | Inject conflict, verify Claude delegation |
| Escalation | Verify Terminal and Multi escalators |

### 11.3 Self-Hosting Test

Ultimate test: `choo run specs/tasks/` on its own codebase.

---

## 12. Resolved Questions

1. **PR Creation**: Delegated to Claude via `gh pr create`. Claude writes contextual titles and descriptions.

2. **Commit Messages**: Delegated to Claude. Claude writes conventional commits based on actual changes.

3. **Git Operations**: Orchestrator verifies outcomes, Claude executes operations (stage, commit, push).

4. **Failure Handling**: Retry with exponential backoff for transient failures. Escalate to user after max retries (no fallback to direct operations).

5. **Escalation Backends**: Clean `Escalator` interface supports Terminal (default), Slack, or custom webhooks.

---

## 13. Success Metrics

| Metric | Target |
|--------|--------|
| Can run on own codebase | Yes |
| Parallel unit execution | 2+ concurrent |
| Conflict auto-resolution | >80% success |
| Time to merge (no conflicts) | <5 min after approval |
| Escalation delivery | 100% to at least one backend |
