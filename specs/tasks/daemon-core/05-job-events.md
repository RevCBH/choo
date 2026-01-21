---
task: 5
status: pending
backpressure: "go test ./internal/daemon/... -run TestJobEvents"
depends_on: [4]
---

# Job Event Subscription

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #5 of 7 in implementation plan

## Objective

Implement event subscription and streaming for individual jobs, enabling clients to receive real-time updates.

## Dependencies

### Task Dependencies (within this unit)
- Task #4 (JobManager core implementation)

### Package Dependencies
- `sync` - for mutex
- `github.com/charlotte/internal/events` - for Event type

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── job_events.go    # CREATE: Event subscription logic
```

### Types to Implement

```go
// Subscription represents an active event subscription for a job.
type Subscription struct {
    JobID    string
    Channel  <-chan events.Event
    cancel   func()
}
```

### Functions to Implement

```go
// Subscribe returns an event channel for a specific job.
// The returned channel receives events until the job completes
// or Unsubscribe is called. The cleanup function must be called
// when done to release resources.
func (jm *JobManager) Subscribe(jobID string) (<-chan events.Event, func(), error) {
    // 1. Get job from map (return error if not found)
    // 2. Create buffered channel for events
    // 3. Register channel with job's event bus
    // 4. Return channel and unsubscribe function
}

// SubscribeFrom returns events starting from a specific sequence number.
// Historical events are replayed from the database before live events.
func (jm *JobManager) SubscribeFrom(jobID string, fromSequence int) (<-chan events.Event, func(), error) {
    // 1. Get job from map (return error if not found)
    // 2. Create buffered channel for events
    // 3. Query historical events from database
    // 4. Send historical events first
    // 5. Register for live events
    // 6. Return channel and cleanup function
}

// broadcast sends an event to all subscribers of a job.
// Called internally when events occur.
func (jm *JobManager) broadcast(jobID string, event events.Event) {
    // Get job's subscriber list
    // Send to each channel (non-blocking)
    // Log if channel full (subscriber too slow)
}

// closeJobSubscriptions closes all subscription channels for a job.
// Called when job completes.
func (jm *JobManager) closeJobSubscriptions(jobID string) {
    // Get all subscribers for job
    // Close each channel
    // Clear subscriber list
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestJobEvents
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestSubscribe_ValidJob` | Returns channel and cleanup function |
| `TestSubscribe_InvalidJob` | Returns error for unknown job ID |
| `TestSubscribe_ReceivesEvents` | Channel receives broadcasted events |
| `TestSubscribe_Unsubscribe` | Cleanup function removes subscription |
| `TestSubscribeFrom_ReplaysHistory` | Receives historical events first |
| `TestSubscribeFrom_ThenLive` | Receives live events after history |
| `TestBroadcast_MultipleSubscribers` | All subscribers receive event |
| `TestBroadcast_SlowSubscriber` | Fast subscribers not blocked by slow |
| `TestCloseSubscriptions` | All channels closed when job completes |

### Test Fixtures

```go
func TestSubscribe_ReceivesEvents(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 10)

    // Start a job
    jobID, err := jm.Start(context.Background(), validJobConfig())
    require.NoError(t, err)

    // Subscribe
    events, cleanup, err := jm.Subscribe(jobID)
    require.NoError(t, err)
    defer cleanup()

    // Broadcast an event
    testEvent := events.Event{Type: "test", Data: "hello"}
    jm.broadcast(jobID, testEvent)

    // Receive with timeout
    select {
    case e := <-events:
        assert.Equal(t, "test", e.Type)
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for event")
    }
}

func TestSubscribeFrom_ReplaysHistory(t *testing.T) {
    db := setupTestDB(t)
    jm := NewJobManager(db, 10)

    // Start job and emit some events
    jobID, _ := jm.Start(context.Background(), validJobConfig())
    jm.broadcast(jobID, events.Event{Sequence: 1, Type: "event1"})
    jm.broadcast(jobID, events.Event{Sequence: 2, Type: "event2"})
    jm.broadcast(jobID, events.Event{Sequence: 3, Type: "event3"})

    // Subscribe from sequence 2
    ch, cleanup, err := jm.SubscribeFrom(jobID, 2)
    require.NoError(t, err)
    defer cleanup()

    // Should receive events 2 and 3
    e1 := <-ch
    assert.Equal(t, 2, e1.Sequence)
    e2 := <-ch
    assert.Equal(t, 3, e2.Sequence)
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use buffered channels (suggest 100 events) to prevent blocking
- Non-blocking sends with logging for slow subscribers
- Historical replay queries database, so may be slow for long-running jobs
- Channel buffer size should be configurable
- Cleanup function must be safe to call multiple times

## NOT In Scope

- gRPC streaming integration (handled by DAEMON-GRPC)
- Event persistence (handled by DAEMON-DB)
- Daemon-level events (only job-specific events)
