---
prd_id: feature-workflow
title: "PRD-Based Automated Feature Development Workflow"
status: draft
depends_on:
  - web-ui
# Orchestrator-managed fields
# feature_branch: feature/feature-workflow
# feature_status: pending
# spec_review_iterations: 0
---

# PRD-Based Automated Feature Development Workflow

## Document Info

| Field   | Value      |
| ------- | ---------- |
| Status  | Draft      |
| Author  | Claude     |
| Created | 2026-01-20 |
| Target  | v0.4       |

---

## 1. Overview

### 1.1 Goal

Enable choo to autonomously develop features from Product Requirements Documents (PRDs): analyze dependencies, generate specs with review loops, create tasks, run the development workflow against feature branches, and open PRs to main when complete.

### 1.2 Current State

Choo currently requires manual orchestration of the spec→task→development pipeline:
1. Manually write PRD
2. Manually invoke spec-generator agents
3. Manually invoke spec-validator
4. Manually invoke task-generator / ralph-prep
5. Run `choo run` against main branch
6. Manually coordinate feature PRs

### 1.3 Proposed Solution

A fully automated feature development pipeline:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     PRD-Based Feature Workflow                               │
└─────────────────────────────────────────────────────────────────────────────┘

 docs/prds/                     .claude/agents/                 specs/tasks/
 ┌─────────────┐               ┌─────────────────┐             ┌─────────────┐
 │ FEATURE-A   │───┬──────────▶│ prd-prioritizer │────────────▶│ feature-a/  │
 │ FEATURE-B   │   │           │ spec-generator  │             │   ├ IMPL..  │
 │ FEATURE-C   │   │           │ spec-reviewer   │◀───loop────▶│   ├ 01-..   │
 └─────────────┘   │           │ spec-validator  │             │   └ 02-..   │
                   │           │ task-generator  │             └──────┬──────┘
                   │           └─────────────────┘                    │
                   │                                                  │
                   │   choo next-feature                              │ commit to
                   │   choo feature start <prd>                       │ feature branch
                   ▼                                                  ▼
         ┌─────────────────┐                              ┌─────────────────┐
         │ feature/<prd>   │◀─────────────────────────────│ choo run        │
         │ (branch)        │  unit PRs target             │ (orchestrator)  │
         └────────┬────────┘  feature branch              └─────────────────┘
                  │                                                │
                  │ All units merged + branch clean                │
                  ▼                                                ▼
         ┌─────────────────┐                              ┌─────────────────┐
         │ PR: feature→main│◀── auto-triggered ──────────│ escalate notify │
         └─────────────────┘                              └─────────────────┘
```

### 1.4 Success Criteria

1. `choo next-feature` recommends which PRD to implement next
2. `choo feature start <prd-id>` generates specs with automated review loop
3. Specs and tasks are committed to feature branch before `choo run` begins
4. `choo run --feature <prd-id>` targets feature branch instead of main
5. Orchestrator auto-triggers feature PR when all units merged and branch is clean
6. Notifications via existing escalation system

---

## 2. PRD Storage and Format

### 2.1 Location

PRDs stored under `docs/prds/` with standardized YAML frontmatter.

### 2.2 Frontmatter Schema

```yaml
---
# === Required fields ===
prd_id: streaming-events        # Unique identifier (used for branch names)
title: "Event Streaming Architecture"
status: draft                   # draft | approved | in_progress | complete | archived

# === Optional dependency hints ===
depends_on:                     # Other PRD IDs this feature logically depends on
  - self-hosting                # Claude's analysis is authoritative, but hints help
  - web-ui

# === Complexity estimate (optional) ===
estimated_units: 3              # Expected number of units
estimated_tasks: 15             # Expected total tasks

# === Feature workflow state (orchestrator-managed) ===
# feature_branch: feature/streaming-events
# feature_status: pending       # pending | generating_specs | reviewing_specs |
#                               # review_blocked | validating_specs | generating_tasks |
#                               # specs_committed | in_progress | units_complete |
#                               # pr_open | complete | failed
# feature_started_at: null
# feature_completed_at: null
# spec_review_iterations: 0
# last_spec_review: null
---
```

### 2.3 Data Ownership Model

**Single-Writer Assumption**: Only the orchestrator running on the feature branch updates PRD frontmatter. This ensures:
- No merge conflicts on frontmatter fields
- Consistent state tracking
- Clear source of truth

**Rules**:
1. Unit PRs **must not** modify PRD frontmatter fields
2. Frontmatter changes propagate to main via the feature PR (when complete)
3. If the orchestrator detects non-frontmatter changes to the PRD (body edits), it triggers a **drift assessment** to determine if any work branches have diverged from the updated requirements

**Drift Assessment** (on PRD body change detection):
1. Pause unit work
2. Diff PRD body against last-seen version
3. Invoke Claude to assess impact on in-progress units
4. If significant: escalate to user with affected units list
5. If minor: log and continue

### 2.4 PRD Body Format

PRDs follow a standard markdown structure:
- Overview and goals
- Architecture diagrams
- Requirements (functional, performance, constraints)
- Design details
- Implementation considerations
- Success criteria

---

## 3. New Agents

### 3.1 PRD Prioritizer Agent

**Purpose**: Analyze all PRDs and recommend implementation order based on dependencies and refactoring impact.

**Location**: `.claude/agents/prd-prioritizer.md`

**Invocation**:
```
Task tool with subagent_type: "general-purpose"

Prompt: "Analyze PRDs and recommend next feature to implement.

PRD Directory: docs/prds/
Current specs: specs/tasks/

Consider:
1. Explicit depends_on hints in PRD frontmatter (guidance, not authoritative)
2. Implicit dependencies from PRD content (what systems does it require?)
3. Refactoring impact (which features enable others?)
4. Current codebase state (what infrastructure exists?)

Output format:
1. Ranked list of PRDs with reasoning
2. Dependency graph visualization
3. Recommended implementation order"
```

**Analysis Criteria**:
- Foundation features first (features others depend on)
- Refactoring enablers (features that simplify future implementations)
- Independent features last (can be parallelized)
- Technical debt (features that fix or improve existing systems)

### 3.2 Spec Reviewer Agent

**Purpose**: Review generated specs for quality, completeness, and consistency. Outputs specific feedback for improvement or approval.

**Location**: `.claude/agents/spec-reviewer.md`

**Invocation**:
```
Task tool with subagent_type: "general-purpose"

Prompt: "Review the generated specs for {FEATURE}.

PRD: docs/prds/{FEATURE}.md
Specs: specs/tasks/{feature}/

Review criteria:
1. COMPLETENESS: All PRD requirements have corresponding spec sections
2. CONSISTENCY: Types, interfaces, and naming are consistent throughout
3. TESTABILITY: Backpressure commands are specific and executable
4. ARCHITECTURE: Follows existing patterns in codebase

Output format (MUST be valid JSON):
{
  "verdict": "pass" | "needs_revision",
  "score": { "completeness": 0-100, "consistency": 0-100, ... },
  "feedback": [
    { "section": "...", "issue": "...", "suggestion": "..." }
  ]
}"
```

**Output Schema Validation**:
The orchestrator validates reviewer output against the expected schema:
- Must be valid JSON
- Must contain `verdict` field with value `"pass"` or `"needs_revision"`
- Must contain `score` object with numeric values
- `feedback` array required when `verdict` is `"needs_revision"`

**Malformed Output Handling**:
1. On invalid JSON or missing required fields: retry once (transient Claude issue)
2. On second failure: transition to `review_blocked` with error details
3. Notification includes raw output for debugging

**Review Loop**:
1. Generate spec via spec-generator
2. Run spec-reviewer
3. Validate reviewer output schema
4. If needs_revision: apply feedback, regenerate, repeat
5. If pass: proceed to validation
6. Max iterations: 3 (configurable)

**Iteration Exhaustion Handling**:
When `max_iterations` reached with `verdict: "needs_revision"`:
1. Transition to `review_blocked` state
2. Send notification with:
   - Current spec state
   - All feedback from each iteration
   - Reviewer scores per iteration
   - Suggested recovery actions
3. Await user intervention (see `choo feature resume`)

---

## 4. New CLI Commands

### 4.1 `choo next-feature`

Recommend which PRD to implement next.

```
choo next-feature [prd-dir]

Analyze PRDs and recommend next feature to implement.

Arguments:
  prd-dir    Path to PRDs directory (default: docs/prds)

Flags:
  --explain         Show detailed reasoning for recommendation
  --top N           Show top N recommendations (default: 3)
  --json            Output as JSON

Examples:
  choo next-feature
  choo next-feature --explain --top 5
```

### 4.2 `choo feature start`

Start a feature workflow from a PRD.

```
choo feature start <prd-id> [flags]

Create feature branch, generate specs with review, generate tasks, commit to branch.

Arguments:
  prd-id    The PRD ID to start (e.g., "streaming-events")

Flags:
  --prd-dir string        PRDs directory (default: docs/prds)
  --specs-dir string      Output specs directory (default: specs/tasks)
  --skip-spec-review      Skip automated spec review loop
  --max-review-iter int   Max spec review iterations (default: 3)
  --dry-run               Show plan without executing

Examples:
  choo feature start streaming-events
  choo feature start streaming-events --skip-spec-review
  choo feature start streaming-events --dry-run
```

**Workflow**:
1. Read PRD from `docs/prds/<prd-id>.md`
2. Create feature branch `feature/<prd-id>` from main
3. Update PRD frontmatter: `feature_status: generating_specs`
4. Invoke spec-generator agent with PRD
5. Loop: spec-reviewer → validate output → feedback → update specs (max iterations)
   - On iteration exhaustion or malformed output: transition to `review_blocked`, notify, exit
6. Update PRD frontmatter: `feature_status: validating_specs`
7. Invoke spec-validator agent
8. Update PRD frontmatter: `feature_status: generating_tasks`
9. Invoke task-generator agent
10. **Commit specs and tasks to feature branch** (see §5.3)
11. Update PRD frontmatter: `feature_status: specs_committed`

### 4.3 `choo feature status`

Show feature workflow status.

```
choo feature status [prd-id]

Show status of feature workflows.

Arguments:
  prd-id    Specific PRD to check (optional, shows all if omitted)

Examples:
  choo feature status                    # Show all in-progress features
  choo feature status streaming-events   # Show specific feature
```

### 4.4 `choo feature resume`

Resume a blocked or paused feature workflow.

```
choo feature resume <prd-id> [flags]

Resume a feature workflow from a blocked state.

Arguments:
  prd-id    The PRD ID to resume

Flags:
  --skip-review           Skip remaining review iterations and proceed to validation
  --from-validation       Resume from spec validation (after manual spec edits)
  --from-tasks            Resume from task generation (after manual spec edits)

Examples:
  choo feature resume streaming-events
  choo feature resume streaming-events --skip-review
  choo feature resume streaming-events --from-validation
```

**Use Cases**:
- Resume after `review_blocked` (iteration exhaustion or malformed output)
- Resume after manual spec edits
- Resume after user intervention on any blocked state

---

## 5. Feature Branch Management

### 5.1 Branch Naming

Pattern: `feature/<prd-id>`

Examples:
- `feature/streaming-events`
- `feature/multi-repo`
- `feature/webhooks`

### 5.2 Orchestrator Integration

When running with `--feature` flag, the orchestrator:
1. Targets the feature branch instead of main for unit PRs
2. Tracks feature state in PRD frontmatter
3. **Auto-triggers feature completion** when conditions are met (see §5.4)

```go
// In orchestrator config
type Config struct {
    // ... existing fields ...

    FeatureBranch string  // Set when --feature flag provided
    FeatureMode   bool    // true when in feature mode
}

func (o *Orchestrator) getTargetBranch() string {
    if o.cfg.FeatureMode && o.cfg.FeatureBranch != "" {
        return o.cfg.FeatureBranch
    }
    return o.cfg.TargetBranch
}
```

### 5.3 Commit Specs to Feature Branch

**Purpose**: Ensure generated specs and tasks are available to worktrees before `choo run` begins.

**Timing**: After task generation completes, before transitioning to `specs_committed` state.

**Implementation**:
```go
func (f *FeatureWorkflow) commitSpecs(ctx context.Context, prdID string) error {
    specsDir := filepath.Join("specs/tasks", prdID)

    // Stage all generated spec files
    if err := git.Add(specsDir); err != nil {
        return fmt.Errorf("failed to stage specs: %w", err)
    }

    // Commit with standardized message (no Claude invocation)
    commitMsg := fmt.Sprintf("chore(feature): add specs for %s", prdID)
    if err := git.Commit(commitMsg); err != nil {
        return fmt.Errorf("failed to commit specs: %w", err)
    }

    // Push to remote feature branch
    if err := git.Push(); err != nil {
        return fmt.Errorf("failed to push specs: %w", err)
    }

    return nil
}
```

**Failure Handling**:
- On staging failure: transition to `failed`, notify with error details
- On commit failure: transition to `failed`, notify (likely nothing to commit)
- On push failure: retry once, then transition to `failed`, notify

**What Gets Committed**:
- `specs/tasks/<prd-id>/IMPLEMENTATION_PLAN.md`
- `specs/tasks/<prd-id>/*.md` (all unit specs)
- PRD frontmatter updates (feature_status changes)

### 5.4 Auto-Triggered Feature Completion

**Trigger Conditions** (all must be true):
1. All units in `specs/tasks/<feature>/` have merged PRs
2. Feature branch is clean (no uncommitted changes)
3. No pending or in-progress units remain

**Idempotency**:
- If PR from feature branch to main already exists: no-op (log and skip)
- If feature already in `pr_open` or `complete` state: no-op
- Check performed before any PR creation attempt

**Implementation**:
```go
func (o *Orchestrator) checkFeatureCompletion(ctx context.Context) error {
    if !o.cfg.FeatureMode {
        return nil
    }

    // Check if already completed or PR open
    prd, err := o.loadPRD()
    if err != nil {
        return err
    }
    if prd.FeatureStatus == "pr_open" || prd.FeatureStatus == "complete" {
        return nil // idempotent: already done
    }

    // Check all units complete
    allComplete, err := o.allUnitsComplete()
    if err != nil || !allComplete {
        return err
    }

    // Check branch is clean
    if !o.branchIsClean() {
        return nil // not ready yet
    }

    // Check if PR already exists
    existingPR, err := o.findExistingPR()
    if err != nil {
        return err
    }
    if existingPR != nil {
        // PR exists, just update state
        prd.FeatureStatus = "pr_open"
        return o.savePRD(prd)
    }

    // Create PR and notify
    return o.openFeaturePR(ctx)
}

func (o *Orchestrator) openFeaturePR(ctx context.Context) error {
    // Update state
    prd.FeatureStatus = "units_complete"
    o.savePRD(prd)

    // Open PR
    pr, err := github.CreatePR(o.cfg.FeatureBranch, "main", ...)
    if err != nil {
        return o.escalateError("feature_pr_failed", err)
    }

    // Update state and notify
    prd.FeatureStatus = "pr_open"
    o.savePRD(prd)

    o.notify(FeaturePROpened, pr.URL)
    return nil
}
```

**On Completion**:
1. Transition to `units_complete`
2. Open PR from `feature/<prd-id>` to main
3. Transition to `pr_open`
4. Send notification with PR URL
5. Update PRD frontmatter

---

## 6. Spec Review Loop

### 6.1 Flow

```
┌─────────────┐
│ Generate    │
│ Spec        │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Validate   │──────────────────┐
│  Output     │                  │ malformed (retry once)
└──────┬──────┘                  │
       │ valid                   ▼
       ▼                  ┌─────────────┐
┌─────────────┐           │  review_    │
│   Review    │           │  blocked    │◀──┐
│   Spec      │           └─────────────┘   │
└──────┬──────┘                             │
       │                                    │
       ├─── needs_revision ──▶ ┌──────────┐ │
       │    (iter < max)       │  Apply   │ │
       │                       │ Feedback │ │
       │                       └────┬─────┘ │
       │                            │       │
       │              ◀─────────────┘       │
       │                                    │
       ├─── needs_revision ─────────────────┘
       │    (iter >= max)
       │
       │ pass
       ▼
┌─────────────┐
│  Validate   │
│   Spec      │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Generate   │
│   Tasks     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Commit    │
│   Specs     │
└─────────────┘
```

### 6.2 Implementation

```go
// internal/review/review.go

type ReviewConfig struct {
    MaxIterations int
    Criteria      []string // completeness, consistency, testability, architecture
}

type ReviewResult struct {
    Verdict  string            // "pass" or "needs_revision"
    Score    map[string]int    // criteria -> score (0-100)
    Feedback []ReviewFeedback
    RawOutput string           // For debugging malformed output
}

type ReviewFeedback struct {
    Section    string
    Issue      string
    Suggestion string
}

func (r *Reviewer) ReviewSpecs(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error) {
    // Invoke spec-reviewer agent
    output, err := r.invokeReviewer(ctx, prdPath, specsPath)
    if err != nil {
        return nil, err
    }

    // Validate output schema
    result, err := r.parseAndValidate(output)
    if err != nil {
        // Return result with raw output for debugging
        return &ReviewResult{
            Verdict:   "malformed",
            RawOutput: output,
        }, fmt.Errorf("malformed reviewer output: %w", err)
    }

    return result, nil
}

func (r *Reviewer) parseAndValidate(output string) (*ReviewResult, error) {
    var result ReviewResult
    if err := json.Unmarshal([]byte(output), &result); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }

    // Validate required fields
    if result.Verdict != "pass" && result.Verdict != "needs_revision" {
        return nil, fmt.Errorf("invalid verdict: %s", result.Verdict)
    }
    if result.Score == nil {
        return nil, fmt.Errorf("missing score object")
    }
    if result.Verdict == "needs_revision" && len(result.Feedback) == 0 {
        return nil, fmt.Errorf("needs_revision requires feedback")
    }

    return &result, nil
}

func (r *Reviewer) ApplyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error {
    // Invoke Claude to apply feedback
    // Re-run spec-generator with feedback context
}
```

---

## 7. Configuration

### 7.1 `.choo.yaml` Additions

```yaml
# Feature workflow configuration
feature:
  # PRDs directory (default: docs/prds)
  prd_dir: docs/prds

  # Specs output directory (default: specs/tasks)
  specs_dir: specs/tasks

  # Branch prefix for feature branches (default: feature/)
  branch_prefix: "feature/"

  # Spec review configuration
  spec_review:
    # Maximum review iterations before blocking (default: 3)
    max_iterations: 3

    # Review criteria (all must pass)
    criteria:
      - completeness      # All PRD requirements covered
      - consistency       # Types and interfaces consistent
      - testability       # Backpressure commands are valid
      - architecture      # Follows existing patterns

  # Notification configuration for feature events
  notification:
    # Notification backends (default: [terminal])
    # Options: terminal, web, slack, webhook
    backends:
      - terminal
      - web  # Only if web UI is active

    # Slack webhook URL (from env: CHOO_SLACK_WEBHOOK)
    slack_webhook: ""
```

---

## 8. Event Types

### 8.1 New Events

```go
// Feature lifecycle events
const (
    FeatureStarted        EventType = "feature.started"
    FeatureSpecsGenerated EventType = "feature.specs.generated"
    FeatureSpecsReviewed  EventType = "feature.specs.reviewed"
    FeatureSpecsCommitted EventType = "feature.specs.committed"
    FeatureTasksGenerated EventType = "feature.tasks.generated"
    FeatureUnitsComplete  EventType = "feature.units.complete"
    FeaturePROpened       EventType = "feature.pr.opened"
    FeatureCompleted      EventType = "feature.completed"
    FeatureFailed         EventType = "feature.failed"
)

// Spec review events
const (
    SpecReviewStarted   EventType = "spec.review.started"
    SpecReviewFeedback  EventType = "spec.review.feedback"
    SpecReviewPassed    EventType = "spec.review.passed"
    SpecReviewBlocked   EventType = "spec.review.blocked"
    SpecReviewIteration EventType = "spec.review.iteration"
    SpecReviewMalformed EventType = "spec.review.malformed"
)

// PRD events
const (
    PRDDiscovered   EventType = "prd.discovered"
    PRDSelected     EventType = "prd.selected"
    PRDUpdated      EventType = "prd.updated"
    PRDBodyChanged  EventType = "prd.body.changed"
    PRDDriftDetected EventType = "prd.drift.detected"
)
```

---

## 9. Feature Workflow State Machine

```
┌─────────┐
│ pending │ ─────────────────────────────────────────┐
└────┬────┘                                          │
     │ choo feature start                            │
     ▼                                               │
┌────────────┐                                       │
│ generating │                                       │
│   specs    │                                       │
└─────┬──────┘                                       │
      │                                              │
      ▼                                              │
┌────────────┐     needs_revision     ┌──────────┐  │
│  reviewing │ ◀─────────────────────▶│ updating │  │
│   specs    │      (iter < max)      │  specs   │  │
└─────┬──────┘                        └──────────┘  │
      │                                             │
      │ pass                                        │
      │                                             │
      │ iter >= max OR malformed output             │
      ├────────────────────────────────────────┐    │
      │                                        ▼    │
      │                               ┌─────────────┐
      │                               │   review_   │
      │                               │   blocked   │
      │                               └──────┬──────┘
      │                                      │
      │              choo feature resume     │
      │◀─────────────────────────────────────┘
      │
      ▼
┌────────────┐
│ validating │
│   specs    │
└─────┬──────┘
      │
      ▼
┌────────────┐
│ generating │
│   tasks    │
└─────┬──────┘
      │
      ▼
┌────────────┐
│   specs_   │  ◀── commit specs/tasks to feature branch
│ committed  │
└─────┬──────┘
      │
      │ choo run --feature
      ▼
┌────────────┐
│in_progress │  ◀── orchestrator runs units
│  (units)   │
└─────┬──────┘
      │ all units merged + branch clean
      ▼
┌──────────────┐
│units_complete│
└──────┬───────┘
       │ auto-triggered (idempotent)
       ▼
┌────────────┐
│  pr_open   │  PR: feature branch → main
└─────┬──────┘
      │ merged
      ▼
┌────────────┐
│  complete  │
└────────────┘

┌────────────┐ ◀────────────────────────────────────┘
│   failed   │   (any failure with escalation)
└────────────┘
```

---

## 10. Implementation Phases

### Phase 1: PRD Foundation

**Tasks**:
1. Create `docs/prds/README.md` template
2. Implement `internal/feature/discovery.go` - PRD frontmatter parsing
3. Add feature event types to `internal/events/types.go`

**Deliverables**:
- PRDs can be stored with standardized frontmatter
- PRDs can be discovered and parsed

### Phase 2: PRD Prioritizer

**Tasks**:
1. Create `.claude/agents/prd-prioritizer.md`
2. Implement `internal/feature/prioritizer.go` - Claude invocation
3. Implement `internal/cli/next_feature.go` - `choo next-feature` command
4. Add configuration for prioritization in `.choo.yaml`

**Deliverables**:
- `choo next-feature` recommends which PRD to implement next

### Phase 3: Feature Branch Management

**Tasks**:
1. Implement `internal/feature/branch.go` - Feature branch creation/management
2. Implement `internal/feature/feature.go` - Feature type and lifecycle
3. Update orchestrator config to support feature mode
4. Update `internal/cli/run.go` to support `--feature` flag

**Deliverables**:
- Feature branches can be created from PRDs
- Orchestrator can target feature branches

### Phase 4: Spec Review Loop

**Tasks**:
1. Create `.claude/agents/spec-reviewer.md`
2. Implement `internal/review/review.go` - Review loop orchestration
3. Implement `internal/review/schema.go` - Output schema validation
4. Implement `internal/review/criteria.go` - Review criteria
5. Implement `internal/review/feedback.go` - Feedback application
6. Add spec review events
7. Implement `review_blocked` state handling

**Deliverables**:
- Automated spec review with feedback loop
- Schema validation for reviewer output
- Iteration exhaustion and malformed output handling
- Configurable review criteria

### Phase 5: Spec Commit Step

**Tasks**:
1. Implement `internal/feature/commit.go` - Commit specs to feature branch
2. Add `specs_committed` state to workflow
3. Add commit failure handling and notifications

**Deliverables**:
- Specs and tasks committed to feature branch before `choo run`
- Worktrees reliably receive generated specs

### Phase 6: Feature Workflow Commands

**Tasks**:
1. Implement `internal/feature/workflow.go` - Workflow state machine
2. Implement `internal/cli/feature_start.go` - `choo feature start`
3. Implement `internal/cli/feature_status.go` - `choo feature status`
4. Implement `internal/cli/feature_resume.go` - `choo feature resume`
5. Update `.choo.yaml` schema with feature configuration

**Deliverables**:
- Complete feature workflow from PRD to specs_committed
- Resume capability for blocked states

### Phase 7: Auto-Triggered Completion

**Tasks**:
1. Implement completion condition checks in orchestrator
2. Add idempotency checks (existing PR detection)
3. Implement auto PR creation
4. Extend escalation system for feature notifications
5. Add web UI notification support

**Deliverables**:
- Orchestrator auto-triggers feature PR when conditions met
- Idempotent completion (no duplicate PRs)
- Feature completion notifications

### Phase 8: Drift Detection

**Tasks**:
1. Implement PRD body change detection
2. Implement drift assessment invocation
3. Add escalation for significant drift

**Deliverables**:
- System detects when PRD body changes during feature development
- Users notified if changes may affect in-progress work

---

## 11. Files to Create

| File | Purpose |
|------|---------|
| `internal/feature/discovery.go` | PRD frontmatter parsing |
| `internal/feature/branch.go` | Feature branch create/manage |
| `internal/feature/workflow.go` | Feature state machine |
| `internal/feature/prioritizer.go` | Claude prioritization invocation |
| `internal/feature/commit.go` | Commit specs to feature branch |
| `internal/feature/drift.go` | PRD body change detection |
| `internal/review/review.go` | Spec review loop |
| `internal/review/schema.go` | Reviewer output validation |
| `internal/review/criteria.go` | Review criteria definitions |
| `internal/review/feedback.go` | Apply feedback to specs |
| `internal/cli/next_feature.go` | `choo next-feature` command |
| `internal/cli/feature.go` | `choo feature` subcommands |
| `.claude/agents/prd-prioritizer.md` | Prioritizer agent definition |
| `.claude/agents/spec-reviewer.md` | Reviewer agent definition |
| `docs/prds/README.md` | PRD index and format guide |

## 12. Files to Modify

| File | Changes |
|------|---------|
| `internal/config/config.go` | Add `FeatureConfig` struct |
| `internal/events/types.go` | Add feature/review/PRD event types |
| `internal/cli/cli.go` | Add feature subcommands |
| `internal/cli/run.go` | Add `--feature` flag |
| `internal/orchestrator/orchestrator.go` | Support feature mode, auto-completion checks |

---

## 13. Acceptance Criteria

- [ ] `choo next-feature` analyzes PRDs and outputs ranked recommendations
- [ ] `choo next-feature --explain` shows detailed reasoning
- [ ] `choo feature start <prd>` creates feature branch
- [ ] `choo feature start <prd>` generates specs with automated review
- [ ] Spec review loop validates reviewer output schema
- [ ] Spec review loop handles malformed output (retry once, then block)
- [ ] Spec review loop handles iteration exhaustion (block and notify)
- [ ] `choo feature start <prd>` generates tasks after spec validation
- [ ] Specs and tasks are committed to feature branch before `choo run`
- [ ] `choo feature resume <prd>` resumes blocked workflows
- [ ] `choo run --feature <prd>` targets feature branch
- [ ] Unit PRs are created against feature branch
- [ ] Orchestrator auto-triggers feature PR when all units merged + branch clean
- [ ] Auto-trigger is idempotent (no duplicate PRs)
- [ ] Completion sends notification via escalation system
- [ ] All operations update PRD frontmatter
- [ ] Unit PRs do not modify PRD frontmatter
- [ ] PRD body changes trigger drift assessment

---

## 14. Verification

1. **Unit tests**: Each new package has table-driven tests
2. **Integration test**: `choo feature start` on a sample PRD
3. **Schema validation test**: Malformed reviewer output handling
4. **Idempotency test**: Multiple completion checks don't create duplicate PRs
5. **End-to-end**: Run full workflow on a small feature PRD
6. **Manual verification**:
   - `choo next-feature --explain` shows sensible prioritization
   - Feature branch created correctly
   - Specs generated and reviewed
   - Blocked state works on iteration exhaustion
   - `choo feature resume` continues from blocked state
   - Tasks created in `specs/tasks/<prd-id>/`
   - Specs committed to feature branch
   - Unit PRs target feature branch
   - Feature PR opens automatically when units complete
   - Notification received
