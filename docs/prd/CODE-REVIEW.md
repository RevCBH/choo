---
prd_id: code-review
title: "Advisory Code Review for Pre-Merge Workflow"
status: completed
depends_on:
  - mvp-orchestrator
# Orchestrator-managed fields
# feature_branch: feature/code-review
# feature_status: pending
# spec_review_iterations: 0
---

# Advisory Code Review for Pre-Merge Workflow

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Completed  |
| Author  | Claude     |
| Created | 2026-01-21 |
| Target  | v0.6       |

---

## 1. Overview

### 1.1 Goal

Replace the placeholder `logReviewPlaceholder()` in the worktree pre-merge workflow with a fully functional **advisory** code review system that:

1. Runs automated code review once per unit after all tasks complete
2. Feeds discovered issues to the implementing model for remediation
3. **Never blocks the merge** - review is purely advisory
4. Supports multiple review providers (Codex as default, Claude as alternative)
5. Configures the review provider independently from the task provider

### 1.2 Current State

The pre-merge workflow currently contains a placeholder function:

```go
// internal/worker/worker.go:546-557
func logReviewPlaceholder(ctx context.Context) {
    // Placeholder - no actual review performed
    fmt.Fprintf(os.Stderr, "Code review placeholder - not implemented\n")
}
```

This placeholder is called during `mergeToFeatureBranch()` (lines 328-387) but performs no actual review. The system proceeds directly to merge without any code quality checks.

### 1.3 Proposed Solution

An advisory code review system that integrates into the existing pre-merge workflow:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Advisory Code Review Flow                                 │
└─────────────────────────────────────────────────────────────────────────────┘

    All Tasks Complete
           │
           ▼
    ┌─────────────┐
    │ Run Code    │  ──── codex review --base <branch>
    │ Review      │       OR claude diff-based review
    └──────┬──────┘
           │
           ├──── Review Error ────▶ Log warning, proceed to merge
           │
           ├──── No Issues ───────▶ Log success, proceed to merge
           │
           ▼
    ┌─────────────┐
    │ Issues      │
    │ Found       │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ Feed Issues │  ──── Task provider attempts fix (single iteration)
    │ to Provider │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ Commit Fix  │  ──── If changes made, commit review fixes
    │ Attempt     │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ Merge       │  ◀─── Always proceeds regardless of review outcome
    │ (always)    │
    └─────────────┘
```

### 1.4 Key Design Decisions

#### 1.4.1 Advisory, Non-Blocking Reviews

Reviews are **advisory only** and never block the merge. Rationale:

- **Autonomy**: The system should complete work without human intervention
- **Velocity**: Blocking reviews create bottlenecks in automated pipelines
- **Trust**: The implementing model is trusted; review provides a second opinion
- **Graceful degradation**: If review fails, work isn't lost

After the fix attempt (if any), merge proceeds regardless of outcome.

#### 1.4.2 Separate Reviewer Interface

A new `Reviewer` interface rather than extending `Provider`:

- **Different semantics**: Review involves structured output parsing, not task execution
- **Different invocation**: Codex uses `codex review --base <branch>` vs `codex exec --yolo`
- **Clean separation**: Review concerns are distinct from task execution concerns
- **Extensibility**: Easy to add new review providers without modifying task providers

#### 1.4.3 Single Fix Iteration

The system attempts exactly one fix iteration:

- **Simplicity**: Avoids infinite loops or complex convergence logic
- **Resource efficiency**: Limits compute spent on review cycles
- **Predictability**: Users know exactly what to expect
- **Configurable**: `max_fix_iterations` allows future expansion if needed

### 1.5 Success Criteria

1. Code review runs automatically after all tasks complete in a unit
2. Review issues are parsed and fed to the implementing model
3. Fix attempts are committed if changes are made
4. Merge proceeds regardless of review outcome (advisory)
5. Review can be disabled via configuration
6. Codex and Claude review providers work correctly
7. Review events are emitted for observability

---

## 2. Architecture

### 2.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Worker Pre-Merge Flow                               │
└─────────────────────────────────────────────────────────────────────────────┘

┌───────────────────┐     ┌───────────────────┐     ┌───────────────────┐
│   Worker          │     │   Reviewer        │     │   Provider        │
│   (worker.go)     │────▶│   Interface       │────▶│   (for fixes)     │
└───────────────────┘     └───────────────────┘     └───────────────────┘
         │                         │
         │                         │
         ▼                         ▼
┌───────────────────┐     ┌───────────────────┐
│   review.go       │     │   codex_reviewer  │
│   (orchestration) │     │   claude_reviewer │
└───────────────────┘     └───────────────────┘
         │
         ▼
┌───────────────────┐
│   prompt_review   │
│   (fix prompts)   │
└───────────────────┘
```

### 2.2 Interface Relationships

```go
// Reviewer is separate from Provider
type Reviewer interface {
    Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)
    Name() ProviderType
}

// Worker holds both
type Worker struct {
    provider Provider   // For task execution and fix attempts
    reviewer Reviewer   // For code review (may be nil if disabled)
    // ...
}
```

### 2.3 Configuration Flow

```
.choo.yaml                    Orchestrator                   Worker
┌─────────────┐              ┌─────────────┐              ┌─────────────┐
│ code_review:│              │ Resolve     │              │ reviewer    │
│   enabled   │─────────────▶│ Reviewer    │─────────────▶│ field set   │
│   provider  │              │ from config │              │ (or nil)    │
│   ...       │              └─────────────┘              └─────────────┘
└─────────────┘
```

---

## 3. Requirements

### 3.1 Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1 | System MUST run code review after all tasks in a unit complete | High |
| FR-2 | System MUST NOT block merge due to review failures or issues | High |
| FR-3 | System MUST support Codex as a review provider | High |
| FR-4 | System MUST support Claude as a review provider | High |
| FR-5 | System MUST parse review output into structured issues | High |
| FR-6 | System MUST feed issues to implementing provider for fix attempt | High |
| FR-7 | System MUST commit fix attempts if changes are made | Medium |
| FR-8 | System MUST allow disabling code review via configuration | Medium |
| FR-9 | System MUST emit events for review lifecycle | Medium |
| FR-10 | System SHOULD support configurable max fix iterations | Low |

### 3.2 Non-Functional Requirements

| ID | Requirement | Metric |
|----|-------------|--------|
| NFR-1 | Review execution SHOULD complete within reasonable time | < 5 minutes typical |
| NFR-2 | Review failures MUST NOT crash the worker | 100% graceful handling |
| NFR-3 | Configuration MUST be backwards compatible | No breaking changes |
| NFR-4 | Review provider MUST be independent of task provider | Separate config keys |

### 3.3 Constraints

1. **Go 1.21+**: Must use existing Go version
2. **Existing patterns**: Must follow established provider/worker patterns
3. **No new dependencies**: Use existing libraries where possible
4. **Configuration schema**: Must extend existing `.choo.yaml` schema

---

## 4. Design

### 4.1 Type Definitions

#### 4.1.1 Reviewer Interface

```go
// internal/provider/reviewer.go

package provider

import "context"

// Reviewer performs code review on changes in a worktree.
// Unlike Provider (task execution), Reviewer produces structured feedback.
type Reviewer interface {
    // Review examines changes between baseBranch and HEAD in workdir.
    // Returns structured review results or error.
    // Errors should be treated as non-fatal (advisory review).
    Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error)

    // Name returns the provider type for logging/events.
    Name() ProviderType
}

// ReviewResult contains the structured output of a code review.
type ReviewResult struct {
    // Passed is true if no issues were found.
    Passed bool

    // Issues contains individual review findings.
    Issues []ReviewIssue

    // Summary is a human-readable overview of the review.
    Summary string

    // RawOutput preserves the original reviewer output for debugging.
    RawOutput string
}

// ReviewIssue represents a single finding from the code review.
type ReviewIssue struct {
    // File is the path to the file containing the issue.
    File string

    // Line is the line number (0 if not applicable).
    Line int

    // Severity indicates issue importance: "error", "warning", "suggestion".
    Severity string

    // Message describes what the issue is.
    Message string

    // Suggestion provides recommended fix (may be empty).
    Suggestion string
}
```

#### 4.1.2 Configuration Types

```go
// internal/config/config.go

// CodeReviewConfig controls the advisory code review system.
type CodeReviewConfig struct {
    // Enabled controls whether code review runs. Default: true.
    Enabled bool `yaml:"enabled"`

    // Provider specifies which reviewer to use: "codex" or "claude".
    // Default: "codex".
    Provider ProviderType `yaml:"provider"`

    // MaxFixIterations limits how many times the system attempts fixes.
    // Default: 1 (single review-fix cycle).
    MaxFixIterations int `yaml:"max_fix_iterations"`

    // Command overrides the CLI path for the reviewer.
    // Default: "" (uses system PATH).
    Command string `yaml:"command,omitempty"`
}

// DefaultCodeReviewConfig returns sensible defaults.
func DefaultCodeReviewConfig() CodeReviewConfig {
    return CodeReviewConfig{
        Enabled:          true,
        Provider:         ProviderCodex,
        MaxFixIterations: 1,
        Command:          "",
    }
}
```

### 4.2 Codex Reviewer Implementation

```go
// internal/provider/codex_reviewer.go

package provider

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
)

// CodexReviewer implements Reviewer using Codex CLI.
type CodexReviewer struct {
    command string // Path to codex CLI, empty for system PATH
}

// NewCodexReviewer creates a CodexReviewer with optional command override.
func NewCodexReviewer(command string) *CodexReviewer {
    return &CodexReviewer{command: command}
}

func (r *CodexReviewer) Name() ProviderType {
    return ProviderCodex
}

func (r *CodexReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
    cmdPath := r.command
    if cmdPath == "" {
        cmdPath = "codex"
    }

    // Invoke: codex review --base <baseBranch>
    cmd := exec.CommandContext(ctx, cmdPath, "review", "--base", baseBranch)
    cmd.Dir = workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        // Check exit code - non-zero may mean issues found (not error)
        if exitErr, ok := err.(*exec.ExitError); ok {
            // Codex review returns non-zero when issues found
            return r.parseOutput(string(output), exitErr.ExitCode())
        }
        return nil, fmt.Errorf("codex review failed: %w", err)
    }

    return r.parseOutput(string(output), 0)
}

func (r *CodexReviewer) parseOutput(output string, exitCode int) (*ReviewResult, error) {
    result := &ReviewResult{
        RawOutput: output,
        Passed:    exitCode == 0,
        Issues:    []ReviewIssue{},
    }

    // Parse codex review output format
    // Expected: structured text or JSON output
    // Implementation parses actual codex review output format
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        if issue := r.parseLine(line); issue != nil {
            result.Issues = append(result.Issues, *issue)
        }
    }

    if len(result.Issues) > 0 {
        result.Passed = false
        result.Summary = fmt.Sprintf("Found %d issues", len(result.Issues))
    } else {
        result.Summary = "No issues found"
    }

    return result, nil
}

func (r *CodexReviewer) parseLine(line string) *ReviewIssue {
    // Parse individual issue lines from codex output
    // Format depends on codex review output specification
    // Returns nil if line is not an issue
    return nil // Implementation detail
}
```

### 4.3 Claude Reviewer Implementation

```go
// internal/provider/claude_reviewer.go

package provider

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
)

// ClaudeReviewer implements Reviewer using Claude with diff-based prompts.
type ClaudeReviewer struct {
    command string // Path to claude CLI
}

// NewClaudeReviewer creates a ClaudeReviewer with optional command override.
func NewClaudeReviewer(command string) *ClaudeReviewer {
    return &ClaudeReviewer{command: command}
}

func (r *ClaudeReviewer) Name() ProviderType {
    return ProviderClaude
}

func (r *ClaudeReviewer) Review(ctx context.Context, workdir, baseBranch string) (*ReviewResult, error) {
    // Get diff for review
    diff, err := r.getDiff(ctx, workdir, baseBranch)
    if err != nil {
        return nil, fmt.Errorf("failed to get diff: %w", err)
    }

    if diff == "" {
        return &ReviewResult{
            Passed:  true,
            Summary: "No changes to review",
        }, nil
    }

    // Build review prompt requesting JSON output
    prompt := BuildClaudeReviewPrompt(diff)

    // Invoke Claude
    cmdPath := r.command
    if cmdPath == "" {
        cmdPath = "claude"
    }

    cmd := exec.CommandContext(ctx, cmdPath, "-p", prompt)
    cmd.Dir = workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("claude review failed: %w", err)
    }

    return r.parseOutput(string(output))
}

func (r *ClaudeReviewer) getDiff(ctx context.Context, workdir, baseBranch string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "diff", baseBranch+"...HEAD")
    cmd.Dir = workdir
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return string(output), nil
}

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
        Passed  bool           `json:"passed"`
        Summary string         `json:"summary"`
        Issues  []ReviewIssue  `json:"issues"`
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
```

### 4.4 Worker Integration

```go
// internal/worker/review.go

package worker

import (
    "context"
    "fmt"
    "os"

    "github.com/your-org/choo/internal/provider"
)

// runCodeReview performs advisory code review after task completion.
// This function NEVER returns an error that blocks the merge.
// All review failures are logged but do not prevent merge.
func (w *Worker) runCodeReview(ctx context.Context) {
    if w.reviewer == nil {
        return // Review disabled
    }

    w.emitEvent(ctx, CodeReviewStarted, nil)

    // Determine base branch for comparison
    targetRef := w.getTargetRef()

    // Run the review - errors are logged but don't block
    result, err := w.reviewer.Review(ctx, w.worktreePath, targetRef)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Code review failed to run: %v\n", err)
        w.emitEvent(ctx, CodeReviewFailed, map[string]any{"error": err.Error()})
        return // Proceed to merge anyway
    }

    // No issues found - success
    if result.Passed || len(result.Issues) == 0 {
        fmt.Fprintf(os.Stderr, "Code review passed: %s\n", result.Summary)
        w.emitEvent(ctx, CodeReviewPassed, map[string]any{"summary": result.Summary})
        return
    }

    // Issues found - attempt fix
    fmt.Fprintf(os.Stderr, "Code review found %d issues, attempting fix\n", len(result.Issues))
    w.emitEvent(ctx, CodeReviewIssuesFound, map[string]any{
        "count":  len(result.Issues),
        "issues": result.Issues,
    })

    // Build fix prompt and invoke the implementing provider
    fixPrompt := BuildReviewFixPrompt(result.Issues)
    if err := w.invokeProviderForFix(ctx, fixPrompt); err != nil {
        fmt.Fprintf(os.Stderr, "Fix attempt failed: %v\n", err)
        // Still proceed to merge
        return
    }

    // Commit any fix changes
    if err := w.commitReviewFixes(ctx); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to commit review fixes: %v\n", err)
        // Still proceed to merge
    } else {
        w.emitEvent(ctx, CodeReviewFixApplied, nil)
    }

    // Merge proceeds regardless of fix outcome
}

// invokeProviderForFix asks the task provider to address review issues.
func (w *Worker) invokeProviderForFix(ctx context.Context, fixPrompt string) error {
    return w.provider.Invoke(ctx, TaskPrompt{Content: fixPrompt})
}

// commitReviewFixes commits any changes made during the fix attempt.
func (w *Worker) commitReviewFixes(ctx context.Context) error {
    // Check if there are changes to commit
    // Stage and commit with standardized message
    // Returns nil if no changes
    return nil // Implementation detail
}
```

### 4.5 Prompt Templates

```go
// internal/worker/prompt_review.go

package worker

import (
    "fmt"
    "strings"

    "github.com/your-org/choo/internal/provider"
)

// BuildReviewFixPrompt creates a prompt for the task provider to fix issues.
func BuildReviewFixPrompt(issues []provider.ReviewIssue) string {
    var sb strings.Builder

    sb.WriteString("Code review found the following issues that need to be addressed:\n\n")

    for i, issue := range issues {
        sb.WriteString(fmt.Sprintf("## Issue %d: %s\n", i+1, issue.Severity))
        if issue.File != "" {
            sb.WriteString(fmt.Sprintf("**File**: %s", issue.File))
            if issue.Line > 0 {
                sb.WriteString(fmt.Sprintf(":%d", issue.Line))
            }
            sb.WriteString("\n")
        }
        sb.WriteString(fmt.Sprintf("**Problem**: %s\n", issue.Message))
        if issue.Suggestion != "" {
            sb.WriteString(fmt.Sprintf("**Suggestion**: %s\n", issue.Suggestion))
        }
        sb.WriteString("\n")
    }

    sb.WriteString("Please address these issues. Focus on the most critical ones first.\n")
    sb.WriteString("Make minimal changes needed to resolve the issues.\n")

    return sb.String()
}

// BuildClaudeReviewPrompt creates a prompt for Claude to review a diff.
func BuildClaudeReviewPrompt(diff string) string {
    return fmt.Sprintf(`Review the following code changes and identify any issues.

Focus on:
1. Bugs or logical errors
2. Security vulnerabilities
3. Performance problems
4. Code style and best practices

Output your review as JSON in this exact format:
{
  "passed": true/false,
  "summary": "Brief summary of findings",
  "issues": [
    {
      "file": "path/to/file.go",
      "line": 42,
      "severity": "error|warning|suggestion",
      "message": "Description of the issue",
      "suggestion": "How to fix it"
    }
  ]
}

If there are no issues, set "passed": true and "issues": [].

DIFF:
%s`, diff)
}
```

### 4.6 Event Types

```go
// internal/events/types.go (additions)

const (
    // Code review events
    CodeReviewStarted     EventType = "codereview.started"
    CodeReviewPassed      EventType = "codereview.passed"
    CodeReviewIssuesFound EventType = "codereview.issues_found"
    CodeReviewFixApplied  EventType = "codereview.fix_applied"
    CodeReviewFailed      EventType = "codereview.failed"
)
```

---

## 5. Configuration

### 5.1 YAML Schema

```yaml
# .choo.yaml additions

code_review:
  # Enable/disable code review. Default: true
  enabled: true

  # Review provider: "codex" (default) or "claude"
  provider: codex

  # Maximum fix iterations after review. Default: 1
  # Set to 0 to disable fix attempts (review-only mode)
  max_fix_iterations: 1

  # Optional: Override CLI path for the reviewer
  # Default: uses system PATH
  command: ""
```

### 5.2 Configuration Resolution

```go
// internal/orchestrator/orchestrator.go (additions)

func (o *Orchestrator) resolveReviewer() (provider.Reviewer, error) {
    cfg := o.config.CodeReview

    if !cfg.Enabled {
        return nil, nil // Review disabled
    }

    switch cfg.Provider {
    case provider.ProviderCodex:
        return provider.NewCodexReviewer(cfg.Command), nil
    case provider.ProviderClaude:
        return provider.NewClaudeReviewer(cfg.Command), nil
    default:
        return nil, fmt.Errorf("unknown review provider: %s", cfg.Provider)
    }
}
```

---

## 6. Implementation Plan

### Phase 1: Core Types and Interfaces

**Goal**: Establish the foundational types and interfaces.

**Tasks**:
1. Create `/internal/provider/reviewer.go` with `Reviewer` interface and result types
2. Add `CodeReviewConfig` to `/internal/config/config.go` with defaults
3. Add code review events to `/internal/events/types.go`

**Deliverables**:
- Reviewer interface defined
- Configuration types defined
- Events defined

### Phase 2: Reviewer Implementations

**Goal**: Implement Codex and Claude review providers.

**Tasks**:
1. Create `/internal/provider/codex_reviewer.go`
   - Implement `codex review --base <branch>` invocation
   - Parse output into `ReviewResult`
   - Handle exit codes appropriately
2. Create `/internal/provider/claude_reviewer.go`
   - Implement diff retrieval via `git diff`
   - Build review prompt requesting JSON output
   - Parse structured response

**Deliverables**:
- Working Codex reviewer
- Working Claude reviewer

### Phase 3: Worker Integration

**Goal**: Integrate review into the pre-merge workflow.

**Tasks**:
1. Create `/internal/worker/review.go` with `runCodeReview()` advisory logic
2. Create `/internal/worker/prompt_review.go` with prompt builders
3. Modify `/internal/worker/worker.go`:
   - Add `reviewer provider.Reviewer` field to Worker struct
   - Replace `logReviewPlaceholder()` call with `runCodeReview()`
   - Delete `logReviewPlaceholder()` function (lines 546-557)
4. Add `Reviewer` to WorkerDeps in `/internal/worker/pool.go` or `/internal/worker/deps.go`

**Deliverables**:
- Review integrated into pre-merge flow
- Placeholder removed
- Fix attempt logic working

### Phase 4: Orchestrator Wiring

**Goal**: Wire up configuration and dependency injection.

**Tasks**:
1. Modify `/internal/orchestrator/orchestrator.go`:
   - Add reviewer resolution logic
   - Create reviewer via factory function
   - Pass reviewer to worker pool via WorkerDeps
2. Update configuration loading to include `CodeReviewConfig`

**Deliverables**:
- Reviewer properly injected into workers
- Configuration flows correctly

### Phase 5: Testing and Verification

**Goal**: Ensure correctness and reliability.

**Tasks**:
1. Add unit tests for `CodexReviewer` output parsing
2. Add unit tests for `ClaudeReviewer` output parsing
3. Add unit tests for `runCodeReview()` logic paths
4. Add integration test with mock reviewer
5. Manual testing with real Codex/Claude

**Deliverables**:
- Comprehensive test coverage
- Verified functionality

---

## 7. Files to Create

| File | Purpose |
|------|---------|
| `internal/provider/reviewer.go` | Reviewer interface and result types |
| `internal/provider/codex_reviewer.go` | Codex implementation using `codex review` CLI |
| `internal/provider/claude_reviewer.go` | Claude implementation with diff-based prompts |
| `internal/worker/review.go` | Advisory review orchestration logic |
| `internal/worker/prompt_review.go` | Prompt builders for fix and review |

## 8. Files to Modify

| File | Lines | Changes |
|------|-------|---------|
| `internal/config/config.go` | - | Add `CodeReviewConfig` struct with defaults |
| `internal/worker/worker.go` | 546-557 | Delete `logReviewPlaceholder()` function |
| `internal/worker/worker.go` | ~332 | Replace placeholder call with `runCodeReview()` |
| `internal/worker/worker.go` | struct | Add `reviewer provider.Reviewer` field |
| `internal/worker/pool.go` or `deps.go` | - | Add `Reviewer` to WorkerDeps |
| `internal/orchestrator/orchestrator.go` | ~620-667 | Add reviewer resolution and injection |
| `internal/events/types.go` | - | Add code review event types |

---

## 9. Acceptance Criteria

### Functional Criteria

- [ ] Code review runs automatically after all tasks complete in a unit
- [ ] Codex reviewer correctly invokes `codex review --base <branch>`
- [ ] Claude reviewer correctly builds diff-based prompts
- [ ] Review output is parsed into structured `ReviewResult`
- [ ] Issues are fed to the implementing provider as a fix prompt
- [ ] Fix attempts are committed if changes are made
- [ ] Merge **always** proceeds regardless of review outcome
- [ ] Review can be disabled via `code_review.enabled: false`
- [ ] Review provider can be configured independently of task provider

### Event Criteria

- [ ] `codereview.started` emitted when review begins
- [ ] `codereview.passed` emitted when no issues found
- [ ] `codereview.issues_found` emitted when issues discovered
- [ ] `codereview.fix_applied` emitted when fix committed
- [ ] `codereview.failed` emitted on review execution error

### Non-Blocking Criteria

- [ ] Review execution errors do not block merge
- [ ] Review issues do not block merge
- [ ] Fix attempt failures do not block merge
- [ ] Commit failures for fixes do not block merge

---

## 10. Verification

### 10.1 Unit Tests

```bash
go test ./internal/provider/... -run TestCodexReviewer
go test ./internal/provider/... -run TestClaudeReviewer
go test ./internal/worker/... -run TestRunCodeReview
```

### 10.2 Build Verification

```bash
go build ./...
```

### 10.3 Manual Testing

1. **Basic Review Flow**:
   - Configure `.choo.yaml` with `code_review.enabled: true`
   - Run a unit that produces reviewable changes
   - Verify review is invoked (check stderr output)
   - Verify review results are reported

2. **Issues Found Flow**:
   - Introduce code that will trigger review issues
   - Verify issues are detected and logged
   - Verify fix prompt is sent to task provider
   - Verify merge proceeds regardless

3. **Review Error Flow**:
   - Configure invalid reviewer command
   - Verify error is logged
   - Verify merge proceeds anyway

4. **Disabled Review**:
   - Set `code_review.enabled: false`
   - Verify review is completely skipped

5. **Provider Independence**:
   - Configure `provider: codex` for tasks
   - Configure `code_review.provider: claude`
   - Verify both work independently

### 10.4 Integration Test

```go
func TestCodeReviewIntegration(t *testing.T) {
    // Create mock reviewer that returns issues
    mockReviewer := &MockReviewer{
        Result: &provider.ReviewResult{
            Passed: false,
            Issues: []provider.ReviewIssue{
                {File: "test.go", Line: 10, Severity: "warning", Message: "test issue"},
            },
        },
    }

    // Create worker with mock reviewer
    worker := NewWorker(WorkerConfig{}, WorkerDeps{
        Provider: mockProvider,
        Reviewer: mockReviewer,
    })

    // Run pre-merge flow
    err := worker.mergeToFeatureBranch(ctx)

    // Verify merge succeeded despite issues
    assert.NoError(t, err)

    // Verify review was called
    assert.True(t, mockReviewer.Called)

    // Verify fix was attempted
    assert.True(t, mockProvider.FixCalled)
}
```

---

## 11. Critical Files Reference

| File | Lines | Purpose |
|------|-------|---------|
| `internal/worker/worker.go` | 546-557 | Current placeholder to replace |
| `internal/worker/worker.go` | 328-387 | `mergeToFeatureBranch()` integration point |
| `internal/provider/provider.go` | all | Pattern for Reviewer interface |
| `internal/provider/codex.go` | all | Pattern for CodexReviewer |
| `internal/config/config.go` | all | Config extension pattern |
| `internal/orchestrator/orchestrator.go` | 620-667 | Provider resolution pattern |

---

## 12. Future Enhancements

The following are explicitly out of scope for this PRD but may be considered for future iterations:

1. **Multiple fix iterations**: Allow `max_fix_iterations > 1` for iterative refinement
2. **Review-then-approve mode**: Optional blocking mode for human-in-the-loop workflows
3. **Custom review criteria**: Configurable focus areas (security, performance, style)
4. **Review caching**: Skip review if changes haven't changed since last review
5. **Review metrics**: Track review pass rates, common issues, fix success rates
6. **Additional providers**: GitHub Copilot, custom webhook-based reviewers
