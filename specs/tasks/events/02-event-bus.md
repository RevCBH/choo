---
task: 2
status: pending
backpressure: "go test ./internal/events/..."
depends_on: [1]
---

# Event Bus

**Parent spec**: `/specs/EVENTS.md`
**Task**: #2 of 3 in implementation plan

> **Note**: Task numbers are identifiers for reference. The Ralph agent chooses which
> ready task to work on from all tasks whose dependencies are satisfied.

## Objective

Implement the EventBus with buffered channel dispatch, handler registration, fan-out delivery, and graceful shutdown.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `Event`, `EventType`, `NewEvent`)

### Package Dependencies
- Standard library only (`sync`, `log`, `time`)

## Deliverables

### Files to Create/Modify

```
internal/
└── events/
    └── bus.go    # CREATE: Bus struct and methods
```

### Types to Implement

```go
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

    // closed tracks if Close has been called
    closed bool

    // mu protects handler registration and closed flag
    mu sync.RWMutex
}
```

### Functions to Implement

```go
// NewBus creates a new event bus with the specified buffer size
// Starts the dispatch goroutine automatically
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

### Internal Functions

```go
// loop runs in a dedicated goroutine and processes events sequentially
func (b *Bus) loop()

// dispatch calls all handlers with the event (recovers from panics)
func (b *Bus) dispatch(e Event)
```

## Backpressure

### Validation Command

```bash
go test ./internal/events/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestBus_SubscribeAndEmit` | Handler receives event with correct Type, Unit, and non-zero Time |
| `TestBus_FanOut` | 3 handlers all receive the same event (count == 3) |
| `TestBus_BufferOverflow` | Bus does not block when buffer is full, event is dropped |
| `TestBus_Close` | Drains all pending events before returning |
| `TestBus_CloseIdempotent` | Second Close() call is no-op, no panic |
| `TestBus_Len` | Returns correct count of pending events |
| `TestBus_EmitSetsTime` | Event.Time is set to approximately current time |
| `TestBus_HandlerPanic` | Bus continues operating after handler panics |
| `TestBus_ConcurrentEmit` | Multiple goroutines can emit without race conditions |
| `TestBus_ConcurrentSubscribe` | Multiple goroutines can subscribe without race conditions |

### Test Fixtures

No external fixtures required.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

### Thread Safety

Use a read-write mutex to protect handler registration:

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
        // Recover from handler panic
        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("WARN: handler panicked: %v", r)
                }
            }()
            h(e)
        }()
    }
}
```

### Buffer Overflow Handling

When the buffer is full, Emit drops the event and logs a warning:

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

The dispatch loop drains remaining events on close:

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

### Default Buffer Size

Use 1000 as the default buffer size if 0 or negative is passed.

## NOT In Scope

- Event type definitions (Task #1)
- LogHandler implementation (Task #3)
- StateHandler implementation (Task #3)
