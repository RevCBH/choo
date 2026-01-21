# SPEC-REVIEW — Spec Review Loop with Schema Validation and Iteration Handling

## Overview

The spec review system provides automated quality assurance for generated specifications through a Claude-based reviewer agent. It validates that specs meet completeness, consistency, testability, and architecture criteria before proceeding to implementation.

The review loop orchestrates the interaction between spec generation and review, handling malformed outputs gracefully, applying feedback for iterative improvement, and transitioning to blocked states when human intervention is required. This ensures specs meet quality thresholds while providing clear feedback paths for both automated and manual resolution.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Spec Review System                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐               │
│  │    Spec      │    │   Review     │    │   Schema     │               │
│  │  Generator   │───▶│  Orchestrator│───▶│  Validator   │               │
│  └──────────────┘    └──────┬───────┘    └──────────────┘               │
│                             │                                            │
│                             ▼                                            │
│                      ┌──────────────┐                                    │
│                      │  Reviewer    │                                    │
│                      │   Agent      │                                    │
│                      │ (Task tool)  │                                    │
│                      └──────┬───────┘                                    │
│                             │                                            │
│         ┌───────────────────┼───────────────────┐                       │
│         ▼                   ▼                   ▼                        │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                │
│  │   Feedback  │     │   Criteria  │     │   Events    │                │
│  │  Applier    │     │  Evaluator  │     │  Publisher  │                │
│  └─────────────┘     └─────────────┘     └─────────────┘                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. **FR-1**: Invoke spec-reviewer agent via Task tool with subagent_type "general-purpose"
2. **FR-2**: Validate reviewer output against required JSON schema
3. **FR-3**: Retry once on malformed output (handles transient Claude issues)
4. **FR-4**: Transition to `review_blocked` state on second malformed output failure
5. **FR-5**: Apply feedback to specs when verdict is `needs_revision`
6. **FR-6**: Track iteration count and enforce configurable maximum (default: 3)
7. **FR-7**: Transition to `review_blocked` when max iterations exhausted
8. **FR-8**: Emit appropriate events for each state transition
9. **FR-9**: Include all iteration history in blocked state notifications
10. **FR-10**: Evaluate specs against four criteria: completeness, consistency, testability, architecture

### Performance Requirements

| Metric | Target |
|--------|--------|
| Single review invocation | < 60 seconds |
| Schema validation | < 10 milliseconds |
| Feedback application | < 5 seconds |
| Max iterations before block | 3 (configurable) |
| Retry attempts on malformed | 1 |

### Constraints

- Reviewer agent must be invoked via Task tool (not direct Claude API)
- Output must be valid JSON with required schema fields
- Feedback array required when verdict is `needs_revision`
- All scores must be numeric values 0-100
- Review blocked state requires user intervention via `choo feature resume`

## Design

### Module Structure

```
internal/review/
├── review.go       # Review loop orchestration
├── schema.go       # Output schema validation
├── criteria.go     # Review criteria definitions
├── feedback.go     # Feedback application logic
└── events.go       # Event type definitions
```

### Core Types

```go
// internal/review/review.go

// ReviewConfig configures the review loop behavior
type ReviewConfig struct {
    MaxIterations int      // Maximum review iterations before blocking (default: 3)
    Criteria      []string // Review criteria to evaluate
    RetryOnMalformed int   // Retry attempts on malformed output (default: 1)
}

// DefaultReviewConfig returns sensible defaults
func DefaultReviewConfig() ReviewConfig {
    return ReviewConfig{
        MaxIterations:    3,
        Criteria:         []string{"completeness", "consistency", "testability", "architecture"},
        RetryOnMalformed: 1,
    }
}

// ReviewResult represents the outcome of a single review
type ReviewResult struct {
    Verdict   string            `json:"verdict"`   // "pass" or "needs_revision"
    Score     map[string]int    `json:"score"`     // criteria -> score (0-100)
    Feedback  []ReviewFeedback  `json:"feedback"`  // Required when needs_revision
    RawOutput string            `json:"-"`         // For debugging malformed output
}

// ReviewFeedback represents a single piece of actionable feedback
type ReviewFeedback struct {
    Section    string `json:"section"`    // Spec section with issue
    Issue      string `json:"issue"`      // Description of the problem
    Suggestion string `json:"suggestion"` // How to fix it
}

// IterationHistory tracks review attempts for debugging
type IterationHistory struct {
    Iteration int           `json:"iteration"`
    Result    *ReviewResult `json:"result"`
    Timestamp time.Time     `json:"timestamp"`
}

// ReviewSession tracks the full review loop state
type ReviewSession struct {
    Feature      string             `json:"feature"`
    PRDPath      string             `json:"prd_path"`
    SpecsPath    string             `json:"specs_path"`
    Config       ReviewConfig       `json:"config"`
    Iterations   []IterationHistory `json:"iterations"`
    FinalVerdict string             `json:"final_verdict"` // "pass", "blocked", or ""
    BlockReason  string             `json:"block_reason"`  // Set when blocked
}
```

```go
// internal/review/schema.go

// SchemaError represents a validation failure
type SchemaError struct {
    Field   string
    Message string
}

func (e SchemaError) Error() string {
    return fmt.Sprintf("schema validation failed: %s - %s", e.Field, e.Message)
}

// ValidVerdicts defines acceptable verdict values
var ValidVerdicts = []string{"pass", "needs_revision"}

// RequiredScoreCriteria defines the criteria that must have scores
var RequiredScoreCriteria = []string{"completeness", "consistency", "testability", "architecture"}
```

```go
// internal/review/criteria.go

// Criterion defines a review criterion with its evaluation parameters
type Criterion struct {
    Name        string
    Description string
    MinScore    int // Minimum acceptable score (default: 70)
}

// DefaultCriteria returns the standard review criteria
func DefaultCriteria() []Criterion {
    return []Criterion{
        {
            Name:        "completeness",
            Description: "All PRD requirements have corresponding spec sections",
            MinScore:    70,
        },
        {
            Name:        "consistency",
            Description: "Types, interfaces, and naming are consistent throughout",
            MinScore:    70,
        },
        {
            Name:        "testability",
            Description: "Backpressure commands are specific and executable",
            MinScore:    70,
        },
        {
            Name:        "architecture",
            Description: "Follows existing patterns in codebase",
            MinScore:    70,
        },
    }
}
```

```go
// internal/review/events.go

// Review event types
const (
    SpecReviewStarted   EventType = "spec.review.started"
    SpecReviewFeedback  EventType = "spec.review.feedback"
    SpecReviewPassed    EventType = "spec.review.passed"
    SpecReviewBlocked   EventType = "spec.review.blocked"
    SpecReviewIteration EventType = "spec.review.iteration"
    SpecReviewMalformed EventType = "spec.review.malformed"
)

// ReviewStartedPayload contains data for review started events
type ReviewStartedPayload struct {
    Feature   string `json:"feature"`
    PRDPath   string `json:"prd_path"`
    SpecsPath string `json:"specs_path"`
}

// ReviewFeedbackPayload contains data for feedback events
type ReviewFeedbackPayload struct {
    Feature   string           `json:"feature"`
    Iteration int              `json:"iteration"`
    Feedback  []ReviewFeedback `json:"feedback"`
    Scores    map[string]int   `json:"scores"`
}

// ReviewBlockedPayload contains data for blocked events
type ReviewBlockedPayload struct {
    Feature      string             `json:"feature"`
    Reason       string             `json:"reason"`
    Iterations   []IterationHistory `json:"iterations"`
    Recovery     []string           `json:"recovery_actions"`
    CurrentSpecs string             `json:"current_specs_path"`
}

// ReviewMalformedPayload contains data for malformed output events
type ReviewMalformedPayload struct {
    Feature     string `json:"feature"`
    RawOutput   string `json:"raw_output"`
    ParseError  string `json:"parse_error"`
    RetryNumber int    `json:"retry_number"`
}
```

### API Surface

```go
// internal/review/review.go

// Reviewer orchestrates the spec review loop
type Reviewer struct {
    config    ReviewConfig
    publisher events.Publisher
    taskTool  TaskInvoker
}

// NewReviewer creates a new Reviewer with the given configuration
func NewReviewer(config ReviewConfig, publisher events.Publisher, taskTool TaskInvoker) *Reviewer {
    return &Reviewer{
        config:    config,
        publisher: publisher,
        taskTool:  taskTool,
    }
}

// TaskInvoker abstracts Task tool invocation for testing
type TaskInvoker interface {
    InvokeTask(ctx context.Context, prompt string, subagentType string) (string, error)
}

// RunReviewLoop executes the full review loop for a feature
func (r *Reviewer) RunReviewLoop(ctx context.Context, feature, prdPath, specsPath string) (*ReviewSession, error) {
    session := &ReviewSession{
        Feature:   feature,
        PRDPath:   prdPath,
        SpecsPath: specsPath,
        Config:    r.config,
    }

    r.publisher.Publish(SpecReviewStarted, ReviewStartedPayload{
        Feature:   feature,
        PRDPath:   prdPath,
        SpecsPath: specsPath,
    })

    for iteration := 1; iteration <= r.config.MaxIterations; iteration++ {
        result, err := r.reviewWithRetry(ctx, prdPath, specsPath)
        if err != nil {
            // Malformed output after retries
            session.FinalVerdict = "blocked"
            session.BlockReason = fmt.Sprintf("malformed output: %v", err)
            r.publishBlocked(session, "Reviewer output failed schema validation after retries")
            return session, nil
        }

        session.Iterations = append(session.Iterations, IterationHistory{
            Iteration: iteration,
            Result:    result,
            Timestamp: time.Now(),
        })

        r.publisher.Publish(SpecReviewIteration, ReviewFeedbackPayload{
            Feature:   feature,
            Iteration: iteration,
            Feedback:  result.Feedback,
            Scores:    result.Score,
        })

        if result.Verdict == "pass" {
            session.FinalVerdict = "pass"
            r.publisher.Publish(SpecReviewPassed, ReviewStartedPayload{
                Feature:   feature,
                PRDPath:   prdPath,
                SpecsPath: specsPath,
            })
            return session, nil
        }

        // needs_revision - apply feedback if not last iteration
        if iteration < r.config.MaxIterations {
            r.publisher.Publish(SpecReviewFeedback, ReviewFeedbackPayload{
                Feature:   feature,
                Iteration: iteration,
                Feedback:  result.Feedback,
                Scores:    result.Score,
            })

            if err := r.applyFeedback(ctx, specsPath, result.Feedback); err != nil {
                session.FinalVerdict = "blocked"
                session.BlockReason = fmt.Sprintf("failed to apply feedback: %v", err)
                r.publishBlocked(session, "Feedback application failed")
                return session, nil
            }
        }
    }

    // Max iterations exhausted
    session.FinalVerdict = "blocked"
    session.BlockReason = "max iterations exhausted with needs_revision verdict"
    r.publishBlocked(session, "Review did not pass after maximum iterations")
    return session, nil
}

// ReviewSpecs performs a single review invocation
func (r *Reviewer) ReviewSpecs(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error) {
    output, err := r.invokeReviewer(ctx, prdPath, specsPath)
    if err != nil {
        return nil, fmt.Errorf("reviewer invocation failed: %w", err)
    }

    result, err := ParseAndValidate(output)
    if err != nil {
        return &ReviewResult{
            Verdict:   "malformed",
            RawOutput: output,
        }, fmt.Errorf("malformed reviewer output: %w", err)
    }

    return result, nil
}

// reviewWithRetry attempts review with configured retries on malformed output
func (r *Reviewer) reviewWithRetry(ctx context.Context, prdPath, specsPath string) (*ReviewResult, error) {
    var lastErr error
    for attempt := 0; attempt <= r.config.RetryOnMalformed; attempt++ {
        result, err := r.ReviewSpecs(ctx, prdPath, specsPath)
        if err == nil {
            return result, nil
        }

        lastErr = err
        r.publisher.Publish(SpecReviewMalformed, ReviewMalformedPayload{
            Feature:     filepath.Base(specsPath),
            RawOutput:   result.RawOutput,
            ParseError:  err.Error(),
            RetryNumber: attempt + 1,
        })

        if attempt < r.config.RetryOnMalformed {
            // Brief delay before retry
            time.Sleep(time.Second)
        }
    }
    return nil, lastErr
}

func (r *Reviewer) invokeReviewer(ctx context.Context, prdPath, specsPath string) (string, error) {
    prompt := fmt.Sprintf(`Review the generated specs for quality and completeness.

PRD: %s
Specs: %s

Review criteria:
1. COMPLETENESS: All PRD requirements have corresponding spec sections
2. CONSISTENCY: Types, interfaces, and naming are consistent throughout
3. TESTABILITY: Backpressure commands are specific and executable
4. ARCHITECTURE: Follows existing patterns in codebase

Output format (MUST be valid JSON):
{
  "verdict": "pass" | "needs_revision",
  "score": { "completeness": 0-100, "consistency": 0-100, "testability": 0-100, "architecture": 0-100 },
  "feedback": [
    { "section": "...", "issue": "...", "suggestion": "..." }
  ]
}`, prdPath, specsPath)

    return r.taskTool.InvokeTask(ctx, prompt, "general-purpose")
}

func (r *Reviewer) publishBlocked(session *ReviewSession, reason string) {
    r.publisher.Publish(SpecReviewBlocked, ReviewBlockedPayload{
        Feature:      session.Feature,
        Reason:       reason,
        Iterations:   session.Iterations,
        CurrentSpecs: session.SpecsPath,
        Recovery: []string{
            "Review feedback from all iterations",
            "Manually edit specs to address issues",
            fmt.Sprintf("Run: choo feature resume %s", session.Feature),
        },
    })
}
```

```go
// internal/review/schema.go

// ParseAndValidate parses JSON output and validates against required schema
func ParseAndValidate(output string) (*ReviewResult, error) {
    // Extract JSON from output (may have surrounding text)
    jsonStr := extractJSON(output)
    if jsonStr == "" {
        return nil, &SchemaError{Field: "root", Message: "no valid JSON found in output"}
    }

    var result ReviewResult
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return nil, &SchemaError{Field: "root", Message: fmt.Sprintf("invalid JSON: %v", err)}
    }

    // Validate verdict
    if !isValidVerdict(result.Verdict) {
        return nil, &SchemaError{
            Field:   "verdict",
            Message: fmt.Sprintf("must be 'pass' or 'needs_revision', got '%s'", result.Verdict),
        }
    }

    // Validate score object exists
    if result.Score == nil {
        return nil, &SchemaError{Field: "score", Message: "missing required score object"}
    }

    // Validate all required criteria have scores
    for _, criterion := range RequiredScoreCriteria {
        score, ok := result.Score[criterion]
        if !ok {
            return nil, &SchemaError{
                Field:   fmt.Sprintf("score.%s", criterion),
                Message: "missing required criterion score",
            }
        }
        if score < 0 || score > 100 {
            return nil, &SchemaError{
                Field:   fmt.Sprintf("score.%s", criterion),
                Message: fmt.Sprintf("score must be 0-100, got %d", score),
            }
        }
    }

    // Validate feedback required when needs_revision
    if result.Verdict == "needs_revision" && len(result.Feedback) == 0 {
        return nil, &SchemaError{
            Field:   "feedback",
            Message: "feedback array required when verdict is 'needs_revision'",
        }
    }

    // Validate feedback structure
    for i, fb := range result.Feedback {
        if fb.Section == "" {
            return nil, &SchemaError{
                Field:   fmt.Sprintf("feedback[%d].section", i),
                Message: "section is required",
            }
        }
        if fb.Issue == "" {
            return nil, &SchemaError{
                Field:   fmt.Sprintf("feedback[%d].issue", i),
                Message: "issue is required",
            }
        }
        if fb.Suggestion == "" {
            return nil, &SchemaError{
                Field:   fmt.Sprintf("feedback[%d].suggestion", i),
                Message: "suggestion is required",
            }
        }
    }

    return &result, nil
}

func extractJSON(output string) string {
    // Find first { and last } to extract JSON object
    start := strings.Index(output, "{")
    end := strings.LastIndex(output, "}")
    if start == -1 || end == -1 || end <= start {
        return ""
    }
    return output[start : end+1]
}

func isValidVerdict(verdict string) bool {
    for _, v := range ValidVerdicts {
        if verdict == v {
            return true
        }
    }
    return false
}
```

```go
// internal/review/feedback.go

// FeedbackApplier applies review feedback to specs
type FeedbackApplier struct {
    taskTool TaskInvoker
}

// NewFeedbackApplier creates a new FeedbackApplier
func NewFeedbackApplier(taskTool TaskInvoker) *FeedbackApplier {
    return &FeedbackApplier{taskTool: taskTool}
}

// ApplyFeedback applies the given feedback to specs at the path
func (f *FeedbackApplier) ApplyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error {
    if len(feedback) == 0 {
        return nil
    }

    feedbackJSON, err := json.Marshal(feedback)
    if err != nil {
        return fmt.Errorf("failed to marshal feedback: %w", err)
    }

    prompt := fmt.Sprintf(`Apply the following review feedback to the specs.

Specs directory: %s

Feedback to apply:
%s

For each feedback item:
1. Locate the specified section in the specs
2. Address the issue according to the suggestion
3. Maintain consistency with the rest of the spec

Make the minimal changes necessary to address each issue.`, specsPath, string(feedbackJSON))

    _, err = f.taskTool.InvokeTask(ctx, prompt, "general-purpose")
    return err
}

func (r *Reviewer) applyFeedback(ctx context.Context, specsPath string, feedback []ReviewFeedback) error {
    applier := NewFeedbackApplier(r.taskTool)
    return applier.ApplyFeedback(ctx, specsPath, feedback)
}
```

## Implementation Notes

1. **JSON Extraction**: The reviewer agent may include explanatory text around JSON output. The `extractJSON` function handles this by finding the outermost `{...}` braces.

2. **Retry Strategy**: A single retry on malformed output handles transient Claude issues (e.g., truncated responses). More retries risk wasting time on systematic problems.

3. **Iteration History**: Full history is preserved for debugging blocked states. This includes raw scores and feedback from each iteration.

4. **Event-Driven Architecture**: All state transitions emit events for monitoring and UI updates. The blocked state event includes suggested recovery actions.

5. **Feedback Application**: Uses the Task tool to apply feedback, ensuring consistency with spec generation patterns. This allows Claude to understand context and make appropriate edits.

6. **Configurable Criteria**: While defaults are provided, the criteria list is configurable to allow project-specific review requirements.

## Testing Strategy

### Unit Tests

```go
// internal/review/schema_test.go

func TestParseAndValidate_ValidPass(t *testing.T) {
    input := `{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`
    result, err := ParseAndValidate(input)
    require.NoError(t, err)
    assert.Equal(t, "pass", result.Verdict)
    assert.Equal(t, 90, result.Score["completeness"])
}

func TestParseAndValidate_ValidNeedsRevision(t *testing.T) {
    input := `{"verdict": "needs_revision", "score": {"completeness": 60, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": [{"section": "Overview", "issue": "Missing diagram", "suggestion": "Add architecture diagram"}]}`
    result, err := ParseAndValidate(input)
    require.NoError(t, err)
    assert.Equal(t, "needs_revision", result.Verdict)
    assert.Len(t, result.Feedback, 1)
}

func TestParseAndValidate_InvalidVerdict(t *testing.T) {
    input := `{"verdict": "maybe", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`
    _, err := ParseAndValidate(input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "verdict")
}

func TestParseAndValidate_MissingScore(t *testing.T) {
    input := `{"verdict": "pass", "feedback": []}`
    _, err := ParseAndValidate(input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "score")
}

func TestParseAndValidate_NeedsRevisionWithoutFeedback(t *testing.T) {
    input := `{"verdict": "needs_revision", "score": {"completeness": 60, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`
    _, err := ParseAndValidate(input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "feedback")
}

func TestParseAndValidate_ExtractsJSONFromText(t *testing.T) {
    input := `Here is my review:
{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}
I hope this helps!`
    result, err := ParseAndValidate(input)
    require.NoError(t, err)
    assert.Equal(t, "pass", result.Verdict)
}

func TestParseAndValidate_ScoreOutOfRange(t *testing.T) {
    input := `{"verdict": "pass", "score": {"completeness": 150, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`
    _, err := ParseAndValidate(input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "0-100")
}

func TestParseAndValidate_MissingCriterion(t *testing.T) {
    input := `{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80}, "feedback": []}`
    _, err := ParseAndValidate(input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "architecture")
}
```

```go
// internal/review/review_test.go

func TestReviewer_RunReviewLoop_PassOnFirstIteration(t *testing.T) {
    mockTask := &mockTaskInvoker{
        response: `{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`,
    }
    mockPublisher := &mockPublisher{}

    reviewer := NewReviewer(DefaultReviewConfig(), mockPublisher, mockTask)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", "prd.md", "specs/")

    require.NoError(t, err)
    assert.Equal(t, "pass", session.FinalVerdict)
    assert.Len(t, session.Iterations, 1)
    assert.Contains(t, mockPublisher.events, SpecReviewPassed)
}

func TestReviewer_RunReviewLoop_PassAfterRevision(t *testing.T) {
    mockTask := &mockTaskInvoker{
        responses: []string{
            `{"verdict": "needs_revision", "score": {"completeness": 60, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": [{"section": "Overview", "issue": "Missing", "suggestion": "Add"}]}`,
            `{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`,
        },
    }
    mockPublisher := &mockPublisher{}

    reviewer := NewReviewer(DefaultReviewConfig(), mockPublisher, mockTask)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", "prd.md", "specs/")

    require.NoError(t, err)
    assert.Equal(t, "pass", session.FinalVerdict)
    assert.Len(t, session.Iterations, 2)
}

func TestReviewer_RunReviewLoop_BlockedAfterMaxIterations(t *testing.T) {
    mockTask := &mockTaskInvoker{
        response: `{"verdict": "needs_revision", "score": {"completeness": 60, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": [{"section": "Overview", "issue": "Missing", "suggestion": "Add"}]}`,
    }
    mockPublisher := &mockPublisher{}

    config := DefaultReviewConfig()
    config.MaxIterations = 2

    reviewer := NewReviewer(config, mockPublisher, mockTask)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", "prd.md", "specs/")

    require.NoError(t, err)
    assert.Equal(t, "blocked", session.FinalVerdict)
    assert.Contains(t, session.BlockReason, "max iterations")
    assert.Contains(t, mockPublisher.events, SpecReviewBlocked)
}

func TestReviewer_RunReviewLoop_BlockedOnMalformedOutput(t *testing.T) {
    mockTask := &mockTaskInvoker{
        response: `This is not valid JSON at all`,
    }
    mockPublisher := &mockPublisher{}

    reviewer := NewReviewer(DefaultReviewConfig(), mockPublisher, mockTask)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", "prd.md", "specs/")

    require.NoError(t, err)
    assert.Equal(t, "blocked", session.FinalVerdict)
    assert.Contains(t, session.BlockReason, "malformed")
    assert.Contains(t, mockPublisher.events, SpecReviewMalformed)
}

func TestReviewer_RetryOnMalformedThenSuccess(t *testing.T) {
    mockTask := &mockTaskInvoker{
        responses: []string{
            `Not valid JSON`,
            `{"verdict": "pass", "score": {"completeness": 90, "consistency": 85, "testability": 80, "architecture": 95}, "feedback": []}`,
        },
    }
    mockPublisher := &mockPublisher{}

    reviewer := NewReviewer(DefaultReviewConfig(), mockPublisher, mockTask)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", "prd.md", "specs/")

    require.NoError(t, err)
    assert.Equal(t, "pass", session.FinalVerdict)
}
```

### Integration Tests

```go
// internal/review/integration_test.go

func TestReviewLoop_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup test fixtures
    prdPath := setupTestPRD(t)
    specsPath := setupTestSpecs(t)

    // Use real Task tool invoker
    taskTool := task.NewInvoker()
    publisher := events.NewMemoryPublisher()

    reviewer := NewReviewer(DefaultReviewConfig(), publisher, taskTool)
    session, err := reviewer.RunReviewLoop(context.Background(), "test-feature", prdPath, specsPath)

    require.NoError(t, err)
    assert.NotEmpty(t, session.FinalVerdict)
    assert.NotEmpty(t, session.Iterations)

    // Verify events were published
    assert.True(t, publisher.HasEvent(SpecReviewStarted))
    assert.True(t, publisher.HasEvent(SpecReviewIteration))
}

func TestBlockedStateRecovery_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create a scenario that will block
    prdPath := setupIncompleteTestPRD(t)
    specsPath := setupTestSpecs(t)

    taskTool := task.NewInvoker()
    publisher := events.NewMemoryPublisher()

    config := DefaultReviewConfig()
    config.MaxIterations = 1 // Force quick block

    reviewer := NewReviewer(config, publisher, taskTool)
    session, _ := reviewer.RunReviewLoop(context.Background(), "test-feature", prdPath, specsPath)

    assert.Equal(t, "blocked", session.FinalVerdict)

    // Verify blocked payload has recovery info
    blockedEvents := publisher.GetEvents(SpecReviewBlocked)
    require.Len(t, blockedEvents, 1)

    payload := blockedEvents[0].Payload.(ReviewBlockedPayload)
    assert.NotEmpty(t, payload.Recovery)
    assert.NotEmpty(t, payload.Iterations)
}
```

## Design Decisions

| Decision | Rationale | Alternatives Considered |
|----------|-----------|------------------------|
| Single retry on malformed output | Balances handling transient issues vs. wasting time on systematic problems | Multiple retries (risks delay), no retries (too strict) |
| Task tool for feedback application | Maintains consistency with spec generation; Claude understands context | Direct file edits (loses context), human intervention (too slow) |
| JSON schema validation | Ensures structured, parseable output for automation | Free-form text (harder to process), XML (less familiar to LLMs) |
| 3 default max iterations | Allows meaningful improvement while bounding time/cost | Higher limit (costly), lower limit (may block prematurely) |
| Event-driven state changes | Enables monitoring, UI updates, audit trail | Direct state mutations (no observability), callbacks (tighter coupling) |
| Preserve full iteration history | Essential for debugging blocked states and understanding trends | Last iteration only (loses context), summary only (loses detail) |

## Future Enhancements

1. **Weighted Criteria**: Allow per-criterion weights to prioritize certain aspects (e.g., testability for TDD projects)
2. **Partial Approval**: Allow passing with warnings when scores are borderline
3. **Criterion-Specific Feedback**: Route feedback to specialized appliers per criterion type
4. **Review Caching**: Cache reviews for unchanged spec sections to speed up iterations
5. **Confidence Scoring**: Add reviewer confidence to help identify uncertain verdicts
6. **Parallel Criteria Evaluation**: Review multiple criteria concurrently for faster feedback

## References

- PRD §3.2: Spec Reviewer Agent
- PRD §6: Spec Review Loop
- PRD §6.2: Implementation
- PRD §8: Spec Review Events
- PRD §10 Phase 4: Spec Review Loop Tasks
- `.claude/agents/spec-reviewer.md` (agent definition)
