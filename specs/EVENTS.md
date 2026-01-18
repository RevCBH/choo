# EVENTS - Event System for Ralph Orchestrator

## Overview

The Events package provides a decoupled communication mechanism for ralph-orch through an event bus architecture. It enables loose coupling between components (scheduler, workers, CLI, logging) by allowing publishers to emit events without knowing their consumers, and subscribers to react to events without knowing their sources.

The event bus uses a buffered channel for asynchronous delivery, supports multiple handlers per event type (fan-out), and provides built-in handlers for logging and state persistence. This architecture is the foundation for future extensibility to TUI, MCP, and web dashboard interfaces.

```
                    ┌─────────────────────────────────────────┐
                    │              Event Bus                  │
                    │         chan Event (buffered)           │
                    └───────────────────┬─────────────────────┘
                                        │
        ┌───────────────────────────────┼───────────────────────────────┐
        │                               │                               │
   Publishers                      Dispatch                        Handlers
        │                               │                               │
   ┌────┴────┐                          │                     ┌─────────┴─────────┐
   │         │                          │                     │                   │
Scheduler  Worker                       ▼                  LogHandler      StateHandler
   │         │                    ┌───────────┐                │                   │
   │         │                    │  Fan-out  │                ▼                   ▼
   │         │                    │  to all   │           Structured         Frontmatter
   │         │                    │ handlers  │              Logs             Persist
   └────┬────┘                    └───────────┘
        │
   Emit Events:
   - UnitStarted
   - TaskCompleted
   - PRCreated
   - etc.
```

## Requirements

### Functional Requirements

1. Provide an EventBus that accepts event publications and dispatches to subscribers
2. Support multiple handlers subscribing to receive all events (fan-out pattern)
3. Use buffered channel for non-blocking event emission
4. Define event types for all unit, task, and PR lifecycle transitions
5. Include timestamp, event type, unit ID, task number, PR number, payload, and error in events
6. Provide built-in LogHandler for structured logging output
7. Provide built-in StateHandler for frontmatter persistence
8. Support graceful shutdown with channel close
9. Handle buffer overflow gracefully (drop events with warning, never block)
10. Allow handlers to be registered at any time before or after bus starts

### Performance Requirements

| Metric | Target |
|--------|--------|
| Event emission latency | <1ms (non-blocking) |
| Handler dispatch latency | <10ms per handler |
| Buffer size | 1000 events default |
| Memory per event | <1KB |

### Constraints

- No external dependencies beyond standard library
- Thread-safe for concurrent publishers
- Handlers must not panic (bus should recover)
- Event types must be string constants for JSON serialization
- Events are immutable once created

## Design

### Module Structure

```
internal/events/
├── bus.go          # EventBus implementation
├── types.go        # Event struct and EventType constants
└── handlers.go     # Built-in handlers (LogHandler, StateHandler)
```

### Core Types

```go
// internal/events/types.go

// Event represents a single occurrence in the orchestrator lifecycle
type Event struct {
    // Time is when the event occurred (set by bus on emit)
    Time time.Time `json:"time"`

    // Type identifies what happened
    Type EventType `json:"type"`

    // Unit is the unit ID this event relates to (empty for orchestrator events)
    Unit string `json:"unit,omitempty"`

    // Task is the task number within the unit (nil if not task-related)
    Task *int `json:"task,omitempty"`

    // PR is the pull request number (nil if not PR-related)
    PR *int `json:"pr,omitempty"`

    // Payload contains event-specific data (type varies by event)
    Payload any `json:"payload,omitempty"`

    // Error contains error message if this is a failure event
    Error string `json:"error,omitempty"`
}

// EventType is a string constant identifying the event category
type EventType string

// Orchestrator lifecycle events
const (
    OrchStarted   EventType = "orch.started"
    OrchCompleted EventType = "orch.completed"
    OrchFailed    EventType = "orch.failed"
)

// Unit lifecycle events
const (
    UnitQueued    EventType = "unit.queued"
    UnitStarted   EventType = "unit.started"
    UnitCompleted EventType = "unit.completed"
    UnitFailed    EventType = "unit.failed"
)

// Task lifecycle events
const (
    TaskStarted        EventType = "task.started"
    TaskClaudeInvoke   EventType = "task.claude.invoke"
    TaskClaudeDone     EventType = "task.claude.done"
    TaskBackpressure   EventType = "task.backpressure"
    TaskValidationOK   EventType = "task.validation.ok"
    TaskValidationFail EventType = "task.validation.fail"
    TaskCommitted      EventType = "task.committed"
    TaskCompleted      EventType = "task.completed"
    TaskRetry          EventType = "task.retry"
    TaskFailed         EventType = "task.failed"
)

// PR lifecycle events
const (
    PRCreated           EventType = "pr.created"
    PRReviewPending     EventType = "pr.review.pending"
    PRReviewInProgress  EventType = "pr.review.in_progress"
    PRReviewApproved    EventType = "pr.review.approved"
    PRFeedbackReceived  EventType = "pr.feedback.received"
    PRFeedbackAddressed EventType = "pr.feedback.addressed"
    PRMergeQueued       EventType = "pr.merge.queued"
    PRConflict          EventType = "pr.conflict"
    PRMerged            EventType = "pr.merged"
    PRFailed            EventType = "pr.failed"
)

// Git operation events
const (
    WorktreeCreated EventType = "worktree.created"
    WorktreeRemoved EventType = "worktree.removed"
    BranchPushed    EventType = "branch.pushed"
)
```

```go
// internal/events/bus.go

// Handler is a function that processes events
type Handler func(Event)

// Bus is the central event dispatcher
type Bus struct {
    // handlers is the list of registered event handlers
    handlers []Handler

    // ch is the buffered channel for event delivery
    ch chan Event

    // done signals the dispatch loop to exit
    done chan struct{}

    // mu protects handler registration
    mu sync.RWMutex
}
```

```go
// internal/events/handlers.go

// LogConfig configures the logging handler
type LogConfig struct {
    // Writer is where logs are written (default: os.Stderr)
    Writer io.Writer

    // IncludePayload includes event payload in log output
    IncludePayload bool

    // TimeFormat is the timestamp format (default: RFC3339)
    TimeFormat string
}

// StateConfig configures the state persistence handler
type StateConfig struct {
    // Units is the map of unit ID to Unit pointer for state updates
    Units map[string]*discovery.Unit

    // OnError is called when state persistence fails
    OnError func(error)
}
```

### API Surface

```go
// internal/events/bus.go

// NewBus creates a new event bus with the specified buffer size
func NewBus(bufferSize int) *Bus

// Subscribe registers a handler to receive all events
// Handlers are called in registration order
// Safe to call from multiple goroutines
func (b *Bus) Subscribe(h Handler)

// Emit publishes an event to all handlers
// Sets event.Time to current time
// Non-blocking: drops event if buffer is full (logs warning)
// Safe to call from multiple goroutines
func (b *Bus) Emit(e Event)

// Close stops the dispatch loop and releases resources
// Blocks until all pending events are processed
// Safe to call multiple times (subsequent calls are no-op)
func (b *Bus) Close()

// Len returns the current number of pending events in the buffer
func (b *Bus) Len() int
```

```go
// internal/events/types.go

// NewEvent creates an event with the given type and unit
func NewEvent(eventType EventType, unit string) Event

// WithTask returns a copy of the event with the task number set
func (e Event) WithTask(task int) Event

// WithPR returns a copy of the event with the PR number set
func (e Event) WithPR(pr int) Event

// WithPayload returns a copy of the event with the payload set
func (e Event) WithPayload(payload any) Event

// WithError returns a copy of the event with the error message set
func (e Event) WithError(err error) Event

// IsFailure returns true if this is a failure event type
func (e Event) IsFailure() bool

// String returns a human-readable representation of the event
func (e Event) String() string
```

```go
// internal/events/handlers.go

// LogHandler returns a handler that logs events to the configured writer
func LogHandler(cfg LogConfig) Handler

// StateHandler returns a handler that persists unit state changes to frontmatter
func StateHandler(cfg StateConfig) Handler
```

### Event Flow

```
┌──────────────┐    Emit()     ┌──────────────┐
│   Worker     │──────────────▶│              │
└──────────────┘               │              │
                               │   Buffered   │
┌──────────────┐    Emit()     │   Channel    │
│  Scheduler   │──────────────▶│   (1000)     │
└──────────────┘               │              │
                               │              │
┌──────────────┐    Emit()     │              │
│  PR Manager  │──────────────▶│              │
└──────────────┘               └──────┬───────┘
                                      │
                                      │ dispatch loop
                                      ▼
                               ┌──────────────┐
                               │   Fan-out    │
                               │  to handlers │
                               └──────┬───────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
              ▼                       ▼                       ▼
       ┌────────────┐          ┌────────────┐          ┌────────────┐
       │ LogHandler │          │StateHandler│          │ (Future)   │
       │            │          │            │          │ TUI/MCP    │
       └────────────┘          └────────────┘          └────────────┘
```

### Event Type Categories

| Category | Events | Description |
|----------|--------|-------------|
| Orchestrator | orch.started, orch.completed, orch.failed | Top-level orchestration lifecycle |
| Unit | unit.queued, unit.started, unit.completed, unit.failed | Unit execution lifecycle |
| Task | task.started, task.claude.*, task.validation.*, task.completed, task.failed | Individual task execution |
| PR | pr.created, pr.review.*, pr.feedback.*, pr.merge.*, pr.merged, pr.failed | Pull request lifecycle |
| Git | worktree.created, worktree.removed, branch.pushed | Git operations |

### Handler Execution Order

Handlers are called synchronously in registration order within the dispatch goroutine:

1. LogHandler (always first for debugging)
2. StateHandler (persist state before other side effects)
3. Custom handlers (user-registered)

This ensures logs capture events before state changes, and state is persisted before external notifications.

## Implementation Notes

### Thread Safety

The bus uses a read-write mutex to protect handler registration:

```go
func (b *Bus) Subscribe(h Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers = append(b.handlers, h)
}

func (b *Bus) dispatch(e Event) {
    b.mu.RLock()
    handlers := b.handlers
    b.mu.RUnlock()

    for _, h := range handlers {
        h(e)
    }
}
```

### Buffer Overflow Handling

When the buffer is full, Emit drops the event and logs a warning rather than blocking:

```go
func (b *Bus) Emit(e Event) {
    e.Time = time.Now()
    select {
    case b.ch <- e:
        // Delivered
    default:
        // Buffer full, drop event
        log.Printf("WARN: event buffer full, dropping %s", e.Type)
    }
}
```

### Dispatch Loop

The dispatch loop runs in a dedicated goroutine and processes events sequentially:

```go
func (b *Bus) loop() {
    for {
        select {
        case e := <-b.ch:
            b.dispatch(e)
        case <-b.done:
            // Drain remaining events before exiting
            for {
                select {
                case e := <-b.ch:
                    b.dispatch(e)
                default:
                    return
                }
            }
        }
    }
}
```

### LogHandler Output Format

The log handler produces structured log output:

```
[orch.started]
[unit.started] app-shell
[task.started] app-shell task=#1
[task.claude.invoke] app-shell task=#1
[task.claude.done] app-shell task=#1
[task.validation.ok] app-shell task=#1
[task.committed] app-shell task=#1
[task.completed] app-shell task=#1
[pr.created] app-shell pr=#42
[pr.review.approved] app-shell pr=#42
[pr.merged] app-shell pr=#42
[unit.completed] app-shell
```

### StateHandler State Mapping

The state handler maps events to unit/task status updates:

| Event | State Change |
|-------|--------------|
| UnitStarted | unit.Status = InProgress |
| UnitCompleted | unit.Status = Complete |
| UnitFailed | unit.Status = Failed |
| TaskStarted | task.Status = InProgress |
| TaskCompleted | task.Status = Complete |
| TaskFailed | task.Status = Failed |
| PRCreated | unit.Status = PROpen, unit.PRNumber = pr |
| PRReviewInProgress | unit.Status = InReview |
| PRMergeQueued | unit.Status = Merging |

## Testing Strategy

### Unit Tests

```go
// internal/events/bus_test.go

func TestBus_SubscribeAndEmit(t *testing.T) {
    bus := NewBus(100)
    defer bus.Close()

    received := make(chan Event, 1)
    bus.Subscribe(func(e Event) {
        received <- e
    })

    bus.Emit(NewEvent(UnitStarted, "test-unit"))

    select {
    case e := <-received:
        if e.Type != UnitStarted {
            t.Errorf("expected UnitStarted, got %s", e.Type)
        }
        if e.Unit != "test-unit" {
            t.Errorf("expected test-unit, got %s", e.Unit)
        }
        if e.Time.IsZero() {
            t.Error("expected Time to be set")
        }
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for event")
    }
}

func TestBus_FanOut(t *testing.T) {
    bus := NewBus(100)
    defer bus.Close()

    var count atomic.Int32
    for i := 0; i < 3; i++ {
        bus.Subscribe(func(e Event) {
            count.Add(1)
        })
    }

    bus.Emit(NewEvent(UnitStarted, "test"))

    // Wait for handlers
    time.Sleep(50 * time.Millisecond)

    if count.Load() != 3 {
        t.Errorf("expected 3 handlers called, got %d", count.Load())
    }
}

func TestBus_BufferOverflow(t *testing.T) {
    bus := NewBus(1) // Tiny buffer
    defer bus.Close()

    // Block the handler
    blocker := make(chan struct{})
    bus.Subscribe(func(e Event) {
        <-blocker
    })

    // Fill buffer + 1
    bus.Emit(NewEvent(UnitStarted, "test1"))
    bus.Emit(NewEvent(UnitStarted, "test2"))
    bus.Emit(NewEvent(UnitStarted, "test3")) // Should be dropped

    // Unblock handler
    close(blocker)

    // Verify bus didn't deadlock
    bus.Close()
}

func TestBus_Close(t *testing.T) {
    bus := NewBus(100)

    var processed atomic.Int32
    bus.Subscribe(func(e Event) {
        processed.Add(1)
    })

    bus.Emit(NewEvent(UnitStarted, "test1"))
    bus.Emit(NewEvent(UnitStarted, "test2"))

    // Close should drain pending events
    bus.Close()

    if processed.Load() != 2 {
        t.Errorf("expected 2 events processed, got %d", processed.Load())
    }

    // Second close should be no-op
    bus.Close()
}
```

```go
// internal/events/types_test.go

func TestEvent_WithBuilders(t *testing.T) {
    e := NewEvent(TaskCompleted, "app-shell").
        WithTask(3).
        WithPR(42).
        WithPayload(map[string]string{"file": "01-types.md"})

    if e.Unit != "app-shell" {
        t.Errorf("expected app-shell, got %s", e.Unit)
    }
    if e.Task == nil || *e.Task != 3 {
        t.Errorf("expected task 3, got %v", e.Task)
    }
    if e.PR == nil || *e.PR != 42 {
        t.Errorf("expected PR 42, got %v", e.PR)
    }
    if e.Payload == nil {
        t.Error("expected payload to be set")
    }
}

func TestEvent_IsFailure(t *testing.T) {
    failures := []EventType{OrchFailed, UnitFailed, TaskFailed, PRFailed}
    for _, ft := range failures {
        e := Event{Type: ft}
        if !e.IsFailure() {
            t.Errorf("expected %s to be failure", ft)
        }
    }

    successes := []EventType{OrchCompleted, UnitCompleted, TaskCompleted, PRMerged}
    for _, st := range successes {
        e := Event{Type: st}
        if e.IsFailure() {
            t.Errorf("expected %s to not be failure", st)
        }
    }
}
```

```go
// internal/events/handlers_test.go

func TestLogHandler(t *testing.T) {
    var buf bytes.Buffer
    handler := LogHandler(LogConfig{Writer: &buf})

    e := NewEvent(TaskCompleted, "app-shell").WithTask(1)
    e.Time = time.Date(2025, 1, 18, 10, 30, 0, 0, time.UTC)

    handler(e)

    output := buf.String()
    if !strings.Contains(output, "[task.completed]") {
        t.Errorf("expected event type in output: %s", output)
    }
    if !strings.Contains(output, "app-shell") {
        t.Errorf("expected unit in output: %s", output)
    }
    if !strings.Contains(output, "task=#1") {
        t.Errorf("expected task number in output: %s", output)
    }
}

func TestStateHandler(t *testing.T) {
    unit := &discovery.Unit{
        ID:     "app-shell",
        Status: discovery.UnitStatusPending,
    }
    units := map[string]*discovery.Unit{"app-shell": unit}

    handler := StateHandler(StateConfig{Units: units})

    handler(NewEvent(UnitStarted, "app-shell"))

    if unit.Status != discovery.UnitStatusInProgress {
        t.Errorf("expected InProgress, got %s", unit.Status)
    }
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Multi-publisher stress test | 10 goroutines emitting 1000 events each, verify all received |
| Handler panic recovery | Handler that panics, verify bus continues operating |
| Graceful shutdown | Emit events, close bus, verify all processed |
| State persistence | Emit lifecycle events, verify frontmatter updated correctly |

### Manual Testing

- [ ] Events flow from worker to log output during task execution
- [ ] State persists to frontmatter after unit completion
- [ ] Buffer overflow warning appears under heavy load
- [ ] Close drains all pending events before returning
- [ ] Multiple handlers all receive the same event

## Design Decisions

### Why Buffered Channel over sync.Cond?

Channels provide simpler semantics for the producer-consumer pattern. A buffered channel naturally handles backpressure (via the buffer size) and allows non-blocking sends. sync.Cond would require more complex coordination and manual queue management.

### Why Fan-out to All Handlers Instead of Event Type Filtering?

For the MVP, all handlers need most events (logging needs everything, state handler needs all state changes). Type-specific subscriptions add complexity without current benefit. Future versions can add filtered subscriptions if needed.

### Why Drop Events on Buffer Full Instead of Blocking?

Blocking would cause cascading slowdowns - if a handler is slow, it would slow down all publishers (scheduler, workers, PR manager). Dropping events with a warning maintains system responsiveness while alerting operators to the issue.

### Why String Event Types Instead of Integers?

String constants are:
- Self-documenting in logs and JSON output
- Easier to debug
- Safely serializable without version compatibility issues
- Negligible performance difference for event rates we expect (<100/sec)

### Why Synchronous Handler Dispatch?

Handlers execute synchronously within the dispatch loop for several reasons:
1. Ordering guarantees - events are processed in order
2. Simpler error handling - no need to track async handler failures
3. StateHandler needs synchronous writes to ensure state consistency
4. LogHandler benefits from ordered output

If a handler needs async processing, it can spawn its own goroutine internally.

## Future Enhancements

1. Event type filtering on Subscribe for performance optimization
2. Event history buffer for late subscribers (replay last N events)
3. Metrics emission (event counts, latencies, buffer utilization)
4. Event persistence to file for crash recovery
5. Remote event streaming via WebSocket for web dashboard
6. Event schema versioning for backward compatibility

## References

- [PRD Section 4.6: Event System](/Users/bennett/conductor/workspaces/choo/lahore/docs/MVP%20DESIGN%20SPEC.md)
- [Go Channels](https://go.dev/tour/concurrency/2)
- [Event-Driven Architecture Patterns](https://martinfowler.com/articles/201701-event-driven.html)
