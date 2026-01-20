---
task: 3
status: pending
backpressure: "go test ./internal/feature/... -run TestDrift"
depends_on: [1]
---

# Drift Detection

**Parent spec**: `/specs/FEATURE-WORKFLOW.md`
**Task**: #3 of 6 in implementation plan

## Objective

Implement drift detection that monitors PRD body changes and assesses impact on in-progress units.

## Dependencies

### External Specs (must be implemented)
- CLAUDE-GIT - provides claude client for impact assessment

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `FeatureStatus`)

### Package Dependencies
- `internal/claude` - Claude client for assessment
- `crypto/sha256` - for body hashing

## Deliverables

### Files to Create/Modify

```
internal/
└── feature/
    └── drift.go    # CREATE: Drift detection
```

### Types to Implement

```go
// DriftDetector monitors PRD changes and assesses impact
type DriftDetector struct {
    prd          *PRD
    lastBodyHash string
    lastBody     string
    claude       *claude.Client
}

// DriftResult contains the assessment of PRD changes
type DriftResult struct {
    HasDrift       bool
    Significant    bool
    Changes        string   // Diff summary
    AffectedUnits  []string
    Recommendation string
}

// AssessDriftRequest is the request to Claude for drift assessment
type AssessDriftRequest struct {
    OriginalBody    string
    NewBody         string
    Diff            string
    InProgressUnits []string
}
```

### Functions to Implement

```go
// NewDriftDetector creates a drift detector for the given PRD
func NewDriftDetector(prd *PRD, claudeClient *claude.Client) *DriftDetector

// CheckDrift compares current PRD body to last known state
func (d *DriftDetector) CheckDrift(ctx context.Context) (*DriftResult, error)

// UpdateBaseline sets the current PRD body as the new baseline
func (d *DriftDetector) UpdateBaseline()

// hashBody computes SHA256 hash of PRD body for fast comparison
func hashBody(body string) string

// computeDiff generates a diff summary between old and new body
func computeDiff(oldBody, newBody string) string
```

## Backpressure

### Validation Command

```bash
go test ./internal/feature/... -run TestDrift
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestDriftDetector_NoDrift` | Returns `HasDrift: false` when body unchanged |
| `TestDriftDetector_DetectsDrift` | Returns `HasDrift: true` when body changed |
| `TestDriftDetector_HashComparison` | Hash comparison completes in <50ms |
| `TestDriftDetector_SignificantDrift` | Claude assessment marks significant changes |
| `TestDriftDetector_MinorDrift` | Claude assessment marks minor changes |
| `TestDriftDetector_UpdateBaseline` | Baseline update clears drift detection |
| `TestHashBody_Deterministic` | Same input produces same hash |
| `TestComputeDiff_EmptyBodies` | Handles empty strings |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| `drift_minor.json` | `internal/feature/testdata/` | Minor PRD change scenario |
| `drift_significant.json` | `internal/feature/testdata/` | Significant PRD change scenario |

### CI Compatibility

- [x] No external API keys required (mock Claude client)
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Hash comparison must complete in <50ms (per spec requirement)
- Use SHA256 for body hashing
- Claude assessment is only invoked when drift is detected
- Pause unit work before assessment to prevent wasted effort

```go
func (d *DriftDetector) CheckDrift(ctx context.Context) (*DriftResult, error) {
    currentHash := hashBody(d.prd.Body)
    if currentHash == d.lastBodyHash {
        return &DriftResult{HasDrift: false}, nil
    }

    // Compute diff
    diff := computeDiff(d.lastBody, d.prd.Body)

    // Invoke Claude to assess impact
    assessment, err := d.claude.AssessDrift(ctx, AssessDriftRequest{
        OriginalBody:    d.lastBody,
        NewBody:         d.prd.Body,
        Diff:            diff,
        InProgressUnits: d.getInProgressUnits(),
    })
    if err != nil {
        return nil, fmt.Errorf("drift assessment failed: %w", err)
    }

    return &DriftResult{
        HasDrift:       true,
        Significant:    assessment.Significant,
        Changes:        diff,
        AffectedUnits:  assessment.AffectedUnits,
        Recommendation: assessment.Recommendation,
    }, nil
}
```

## NOT In Scope

- State definitions (Task #1)
- Commit operations (Task #2)
- Completion logic (Task #4)
- Review cycle management (Task #5)
- Workflow orchestration (Task #6)
