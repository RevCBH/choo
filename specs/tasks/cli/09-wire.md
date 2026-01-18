---
task: 9
status: pending
backpressure: "go test ./internal/cli/... -run Wire"
depends_on: [1, 6]
---

# Component Wiring

**Parent spec**: `/specs/CLI.md`
**Task**: #9 of 9 in implementation plan

## Objective

Implement component assembly that wires together all orchestrator dependencies.

## Dependencies

### External Specs (must be implemented)
- CONFIG - provides `Config` type
- EVENTS - provides `Bus` type
- DISCOVERY - provides `Discovery` type
- SCHEDULER - provides `Scheduler` type
- WORKER - provides `Pool` type
- GIT - provides `WorktreeManager` type
- GITHUB - provides `PRClient` type

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)
- Task #6 must be complete (provides: `RunOptions`, `RunOrchestrator`)

### Package Dependencies
- All internal packages: config, events, discovery, scheduler, worker, git, github

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── wire.go    # CREATE: Component assembly and dependency injection
```

### Types to Implement

```go
// Orchestrator holds all wired components
type Orchestrator struct {
    Config    *config.Config
    Events    *events.Bus
    Discovery *discovery.Discovery
    Scheduler *scheduler.Scheduler
    Workers   *worker.Pool
    Git       *git.WorktreeManager
    GitHub    *github.PRClient
}
```

### Functions to Implement

```go
// WireOrchestrator assembles all components for orchestration
func WireOrchestrator(cfg *config.Config) (*Orchestrator, error) {
    // Create event bus first (other components depend on it)
    // Wire components in dependency order
    // Return assembled orchestrator
}

// Close shuts down all orchestrator components
func (o *Orchestrator) Close() error {
    // Stop workers
    // Close event bus
    // Clean up resources
}

// loadConfig loads configuration from .choo.yaml or defaults
func loadConfig(tasksDir string) (*config.Config, error) {
    // Look for .choo.yaml in current directory
    // Fall back to defaults if not found
    // Merge with CLI flags
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Wire -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWireOrchestrator_AllComponents` | All components are non-nil |
| `TestWireOrchestrator_EventBus` | Event bus is shared across components |
| `TestOrchestrator_Close` | Close shuts down cleanly |
| `TestLoadConfig_Default` | Defaults used when no config file |
| `TestLoadConfig_FromFile` | Config loaded from .choo.yaml |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Sample .choo.yaml | testdata/wire/ | Test config loading |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Component initialization order:
  1. Config (from file or defaults)
  2. Event Bus (capacity 1000)
  3. Discovery
  4. Git WorktreeManager
  5. GitHub PRClient
  6. Scheduler
  7. Worker Pool (depends on event bus, git manager)

- Lazy initialization:
  - Components should only be created when needed
  - `version` command should not trigger component creation
  - `status` command only needs Discovery

- Error handling:
  - If any component fails to initialize, clean up already-created components
  - Return clear error indicating which component failed

### Wiring Diagram

```
Config ──────────────────────────────────────────────────┐
                                                         │
Events.Bus ───────────────────────────────────┐          │
    │                                         │          │
    ├─► Discovery ─────────────────────────►  │          │
    │                                         │          │
    ├─► Git.WorktreeManager ──────────────►   │          │
    │                                         ▼          ▼
    ├─► GitHub.PRClient ──────────────────► Scheduler ◄──┘
    │                                         │
    │                                         │
    └─► Worker.Pool ◄─────────────────────────┘
```

## NOT In Scope

- Runtime reconfiguration
- Component hot-reload
- Distributed orchestration
