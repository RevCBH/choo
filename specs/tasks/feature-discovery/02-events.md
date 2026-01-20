---
task: 2
status: complete
backpressure: "go build ./internal/events/..."
depends_on: [1]
---

# Event Types

**Parent spec**: `/specs/FEATURE-DISCOVERY.md`
**Task**: #2 of 7 in implementation plan

## Objective

Add feature lifecycle and PRD event type constants to the existing event system in `internal/events/types.go`.

## Dependencies

### External Specs (must be implemented)
- Events module (already exists)

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: PRD types for context)

### Package Dependencies
- None (extending existing module)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── types.go    # MODIFY: Add feature and PRD event constants
```

### Constants to Add

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

// PRD events
const (
    PRDDiscovered    EventType = "prd.discovered"
    PRDSelected      EventType = "prd.selected"
    PRDUpdated       EventType = "prd.updated"
    PRDBodyChanged   EventType = "prd.body.changed"
    PRDDriftDetected EventType = "prd.drift.detected"
)
```

## Backpressure

### Validation Command

```bash
go build ./internal/events/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `FeatureStarted` | Equals `EventType("feature.started")` |
| `FeatureCompleted` | Equals `EventType("feature.completed")` |
| `PRDDiscovered` | Equals `EventType("prd.discovered")` |
| `PRDDriftDetected` | Equals `EventType("prd.drift.detected")` |

### Test Fixtures

None required - pure constant definitions.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Add new constant blocks after existing Git operation events in `types.go`
- Follow existing naming convention: `Category.Action` (lowercase with dots)
- Feature events relate to PRD-driven feature workflows
- PRD events relate to PRD file discovery and state changes
- Keep event types organized in logical groups with comments

## NOT In Scope

- Event emission logic (handled by Repository in Task #7)
- Event handlers for these new types (separate spec)
- Payload type definitions (consumers define their own)
