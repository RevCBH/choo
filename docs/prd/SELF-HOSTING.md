---
prd_id: self-hosting
title: "Choo Self-Hosting"
status: complete
depends_on:
  - mvp-orchestrator
# Orchestrator-managed fields
# feature_branch: n/a (implemented before feature workflow)
# feature_status: complete
---

# Choo Self-Hosting - Product Requirements Document

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Complete   |
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

- [x] `Escalator` interface defined with `Escalate` method
- [x] `Terminal` escalator prints to stderr with severity indicators
- [x] `Slack` escalator posts to webhook URL
- [x] `Multi` escalator fans out to multiple backends
- [x] Escalation includes unit, title, message, and context map

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

- [x] `choo run specs/tasks/` discovers units and executes them
- [x] Parallelism flag controls concurrent workers
- [x] Events emit for unit lifecycle
- [x] Escalator injected into workers
- [x] Graceful shutdown on SIGINT

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

### 5.3 Acceptance Criteria

- [x] Claude writes commit messages based on actual changes
- [x] Claude pushes branches via `git push`
- [x] Claude creates PRs via `gh pr create` with contextual descriptions
- [x] Retry with exponential backoff on transient failures
- [x] Escalate to user after max retries (no fallback to direct operations)
- [x] PR URL captured from Claude's output

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

### 6.3 Acceptance Criteria

- [x] Poll PR reactions every 30 seconds
- [x] Emit events on status transitions
- [x] Delegate feedback response to Claude (commit + push)
- [x] Escalate if Claude cannot address feedback
- [x] Proceed to merge on approval

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

### 7.3 Acceptance Criteria

- [x] Detect merge conflicts during rebase
- [x] Delegate conflict resolution to Claude (not orchestrator)
- [x] Verify rebase completed before continuing
- [x] Retry with backoff on failure
- [x] Escalate to user after max retries (no fallback)
- [x] Force push with lease after successful resolution
- [x] Emit PRConflict and PRMerged events

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

### 8.3 Acceptance Criteria

- [x] CI runs on all PRs
- [x] Tests, vet, and lint all pass
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

## 11. Success Metrics

| Metric | Target |
|--------|--------|
| Can run on own codebase | Yes |
| Parallel unit execution | 2+ concurrent |
| Conflict auto-resolution | >80% success |
| Time to merge (no conflicts) | <5 min after approval |
| Escalation delivery | 100% to at least one backend |
