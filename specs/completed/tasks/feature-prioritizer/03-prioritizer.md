---
task: 3
status: complete
backpressure: "go test ./internal/feature/... -run TestPrioritizer"
depends_on: [1, 2]
---

# Prioritizer Core

**Parent spec**: `/specs/FEATURE-PRIORITIZER.md`
**Task**: #3 of 5 in implementation plan

## Objective

Implement the Prioritizer struct that orchestrates PRD loading, prompt building, and Claude agent invocation to produce prioritized recommendations.

## Dependencies

### External Specs (must be implemented)
- FEATURE-DISCOVERY - provides context about implemented specs

### Task Dependencies (within this unit)
- Task #1 (provides: `PriorityResult`, `Recommendation`, `PrioritizeOptions`)
- Task #2 (provides: `LoadPRDs`, `PRDForPrioritization`)

### Package Dependencies
- `context` - Context handling for cancellation
- `strings` - Prompt building

## Deliverables

### Files to Create/Modify

```
internal/feature/
├── prioritizer.go       # CREATE
└── prioritizer_test.go  # CREATE
```

### Types to Implement

```go
// Prioritizer analyzes PRDs and recommends implementation order
type Prioritizer struct {
    prdDir   string
    specsDir string
}

// AgentInvoker abstracts the Claude agent invocation for testing
type AgentInvoker interface {
    Invoke(ctx context.Context, prompt string) (string, error)
}
```

### Functions to Implement

```go
// NewPrioritizer creates a new prioritizer for the given directories
func NewPrioritizer(prdDir, specsDir string) *Prioritizer

// Prioritize analyzes PRDs and returns ranked recommendations
func (p *Prioritizer) Prioritize(ctx context.Context, invoker AgentInvoker, opts PrioritizeOptions) (*PriorityResult, error)

// buildPrompt constructs the Claude prompt with PRD content and context
func (p *Prioritizer) buildPrompt(prds []*PRDForPrioritization, specs []string, opts PrioritizeOptions) string

// loadExistingSpecs finds completed spec files for context
func (p *Prioritizer) loadExistingSpecs() ([]string, error)
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestPrioritizer -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestNewPrioritizer` | Sets prdDir and specsDir correctly |
| `TestPrioritizer_BuildPrompt_IncludesPRDs` | Prompt contains PRD content |
| `TestPrioritizer_BuildPrompt_IncludesSpecs` | Prompt contains existing spec names |
| `TestPrioritizer_Prioritize_Success` | Returns parsed result from mock invoker |
| `TestPrioritizer_Prioritize_TopN` | Truncates results to TopN |
| `TestPrioritizer_Prioritize_NoPRDs` | Returns error when no PRDs found |
| `TestPrioritizer_Prioritize_InvokerError` | Propagates invoker errors |

### Test Fixtures

```go
// Mock agent invoker for testing
type mockInvoker struct {
    response string
    err      error
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (string, error) {
    return m.response, m.err
}
```

### CI Compatibility

- [x] No external API keys required (uses mock invoker)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Prompt Structure

The prompt should include:
1. System context: agent role and analysis criteria
2. PRD list: each PRD with ID, title, content, depends_on hints
3. Existing specs: names of completed specs in `specs/completed/`
4. Output format: expected JSON structure
5. Explain flag: request detailed analysis if ShowReason=true

### Analysis Criteria (in prompt)

1. Foundation features (features others depend on)
2. Refactoring enablers (simplify future work)
3. Technical debt fixes
4. Independent features (can parallelize)

### Error Handling

- Return descriptive errors with context
- Wrap underlying errors with `fmt.Errorf("...: %w", err)`
- Check for empty PRD directory early

## NOT In Scope

- Response parsing logic (Task #4)
- CLI command and output formatting (Task #5)
- Actual Claude API integration (uses AgentInvoker interface)
