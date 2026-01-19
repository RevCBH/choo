---
task: 3
status: complete
backpressure: "go test ./internal/cli/... -run Signal"
depends_on: [1]
---

# Signal Handling

**Parent spec**: `/specs/CLI.md`
**Task**: #3 of 9 in implementation plan

## Objective

Implement signal handler for graceful shutdown on SIGINT/SIGTERM.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct)

### Package Dependencies
- `os/signal` (standard library)
- `syscall` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── signals.go    # CREATE: Signal handling for graceful shutdown
```

### Types to Implement

```go
// SignalHandler manages graceful shutdown on interrupt
type SignalHandler struct {
    signals    chan os.Signal
    shutdown   chan struct{}
    cancel     context.CancelFunc
    onShutdown []func()
    mu         sync.Mutex
}
```

### Functions to Implement

```go
// NewSignalHandler creates a signal handler with the given context cancel
func NewSignalHandler(cancel context.CancelFunc) *SignalHandler {
    // Initialize channels
    // Store cancel function
    // Return handler
}

// Start begins listening for signals
func (h *SignalHandler) Start() {
    // Register for SIGINT and SIGTERM
    // Start goroutine to handle signals
}

// OnShutdown registers a callback to run on shutdown
func (h *SignalHandler) OnShutdown(fn func()) {
    // Thread-safe append to callbacks
}

// Wait blocks until shutdown is triggered
func (h *SignalHandler) Wait() {
    // Block on shutdown channel
}

// Stop stops the signal handler and cleans up
func (h *SignalHandler) Stop() {
    // Stop signal notification
    // Close channels if needed
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Signal -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestSignalHandler_New` | `NewSignalHandler(cancel) != nil` |
| `TestSignalHandler_GracefulShutdown` | SIGINT triggers cancel and callbacks |
| `TestSignalHandler_MultipleCallbacks` | All registered callbacks are called |
| `TestSignalHandler_Wait` | Wait blocks until shutdown triggered |
| `TestSignalHandler_ContextCancelled` | Context is cancelled on signal |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Signal handler goroutine should log when signal is received
- Callbacks should be called in registration order
- Use mutex to protect callback slice during registration
- The shutdown channel should be closed after all callbacks complete
- Exit code 130 for SIGINT, 131 for SIGTERM (handled by caller, not this module)

### Shutdown Flow

```
SIGINT/SIGTERM received
    -> Log signal received
    -> Call cancel() to cancel context
    -> Execute callbacks in order
    -> Close shutdown channel
```

## NOT In Scope

- Exit code handling (handled by command implementations)
- Progress saving (handled by callbacks registered by commands)
- Multiple signal handling (first signal triggers shutdown)
