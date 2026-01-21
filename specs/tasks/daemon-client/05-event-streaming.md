---
task: 5
status: pending
backpressure: "go test ./internal/client/... -run TestWatch"
depends_on: [2, 3]
---

# Event Streaming

**Parent spec**: `specs/DAEMON-CLIENT.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement the WatchJob method with callback-based event streaming over gRPC.

## Dependencies

### Task Dependencies (within this unit)
- Task #2 must be complete (provides protobuf conversion)
- Task #3 must be complete (provides connection)

### Package Dependencies
- `context` - context-based cancellation
- `io` - EOF handling
- `internal/api/v1` - generated streaming client
- `internal/events` - event types (assumed to exist)

## Deliverables

### Files to Create/Modify

```
internal/client/
├── client.go        # MODIFY: Add WatchJob method
├── convert.go       # MODIFY: Add protoToEvent conversion
└── client_test.go   # MODIFY: Add streaming tests
```

### Types to Implement

None (event types defined in internal/events package).

### Functions to Implement

```go
// client.go

// WatchJob streams job events, calling handler for each event received.
// The method blocks until the job completes (returns nil), the context
// is cancelled (returns context error), or an error occurs.
//
// Events are delivered in sequence order. The fromSeq parameter specifies
// the sequence number to start from (0 = beginning). This enables
// reconnection scenarios where the client resumes from a specific point.
func (c *Client) WatchJob(ctx context.Context, jobID string, fromSeq int, handler func(events.Event)) error {
    // Build WatchJobRequest with jobID and fromSequence=fromSeq
    // Call daemon.WatchJob to get stream
    // Loop: Recv() events from stream
    //   - io.EOF: job complete, return nil
    //   - Other error: return error
    //   - Success: convert via protoToEvent, call handler
}
```

```go
// convert.go

// protoToEvent converts a protobuf JobEvent to the internal Event type.
// Handles all event type variants (job started, unit progress, etc.)
func protoToEvent(p *apiv1.JobEvent) events.Event {
    // Map event type field to events.Event
    // Convert timestamp via AsTime()
    // Handle event-specific payload fields
}
```

## Implementation Pattern

The streaming loop handles three cases:

```go
stream, err := c.daemon.WatchJob(ctx, &apiv1.WatchJobRequest{
    JobId:        jobID,
    FromSequence: int32(fromSeq),
})
if err != nil {
    return err
}

for {
    event, err := stream.Recv()
    if err == io.EOF {
        return nil  // Job completed normally
    }
    if err != nil {
        return err  // Connection lost or job failed
    }
    handler(protoToEvent(event))
}
```

## Design Decision: Callbacks vs Channels

The spec chose callbacks over channels because:
1. Callbacks allow natural backpressure control
2. Channel-based designs require buffering decisions
3. Risk of deadlocks if consumers are slow

## Backpressure

### Validation Command

```bash
go test ./internal/client/... -run TestWatch
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestWatchJob_ReceivesEvents` | Handler called for each event |
| `TestWatchJob_EOF` | Returns nil on clean completion |
| `TestWatchJob_Error` | Propagates stream errors |
| `TestWatchJob_ContextCancel` | Stops on context cancellation |
| `TestWatchJob_FromSequence` | Resumes from specified sequence number |
| `TestProtoToEvent` | All event fields converted correctly |

## NOT In Scope

- Automatic reconnection on stream failure
- Event buffering or batching
