# Task 04: Implement Event Handler

```yaml
task: 04-handler
unit: history-store
depends_on: [03-store]
backpressure: "go test ./internal/history/... -run TestHandler -v"
```

## Objective

Implement the `Handler` struct that sends events to the daemon via HTTP. This is used by the CLI during `choo run` to stream events.

## Requirements

1. Create `internal/history/handler.go` with:

   ```go
   type Handler struct {
       client *daemon.Client  // Will be implemented in daemon unit
       runID  string
       seq    atomic.Int64
   }

   // Constructor - takes daemon client and run ID
   func NewHandler(client *daemon.Client, runID string) *Handler

   // Implement events.Handler interface
   func (h *Handler) Handle(event events.Event) error

   // Helper to convert internal event to EventRecord
   func (h *Handler) toEventRecord(event events.Event) EventRecord
   ```

2. The `Handle` method should:
   - Increment sequence counter atomically
   - Convert the event bus event to `EventRecord`
   - Send to daemon via `client.SendEvent(runID, record)`
   - Return any errors from the daemon

3. Event type mapping from `events.Event` to `EventRecord.Type`:
   - Map existing event types to the canonical string format
   - Extract unit, task, PR from event data where applicable
   - Copy payload, preserving JSON structure

4. For now, stub the daemon client dependency:
   ```go
   // Placeholder interface until daemon unit implements it
   type DaemonClient interface {
       SendEvent(runID string, event EventRecord) error
   }
   ```

## Acceptance Criteria

- [ ] Handler implements the event handling interface
- [ ] Sequence numbers increment correctly
- [ ] Event conversion preserves all fields
- [ ] Errors from daemon are propagated
- [ ] Thread-safe for concurrent event handling

## Files to Create/Modify

- `internal/history/handler.go` (create)
- `internal/history/handler_test.go` (create)

## Notes

The actual daemon client will be implemented in the DAEMON unit. This task creates the handler that will use it, with a minimal interface for testing.
