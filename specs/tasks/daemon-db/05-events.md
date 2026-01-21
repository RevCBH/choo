---
task: 5
status: complete
backpressure: "go test ./internal/daemon/db/... -run TestEvent"
depends_on: [3]
---

# Event Logging Operations

**Parent spec**: `specs/DAEMON-DB.md`
**Task**: #5 of 6 in implementation plan

## Objective

Implement event logging with per-run sequence numbers and query operations for replay and debugging.

## Dependencies

### Task Dependencies (within this unit)
- Task #3 must be complete (runs must exist before events can reference them)

### Package Dependencies
- `database/sql` - Standard database interface
- `time` - For timestamp handling
- `encoding/json` - For payload serialization
- `fmt` - For error wrapping

## Deliverables

### Files to Create/Modify

```
internal/daemon/db/
├── events.go   # CREATE: Event logging and query operations
└── db_test.go  # MODIFY: Add event tests
```

### Functions to Implement

```go
// AppendEvent records a new event with an auto-assigned sequence number.
// The sequence number is calculated within a transaction to avoid races.
// Payload is JSON-serialized if non-nil.
func (db *DB) AppendEvent(runID string, eventType string, unitID *string, payload interface{}) error {
    // 1. Begin transaction
    // 2. Get next sequence number for this run
    // 3. JSON serialize payload if non-nil
    // 4. INSERT INTO events (run_id, sequence, event_type, unit_id, payload_json)
    // 5. Commit transaction
}

// GetNextSequence returns the next sequence number for a run.
// Used internally by AppendEvent but exposed for transaction support.
func (db *DB) GetNextSequence(runID string) (int, error) {
    // SELECT COALESCE(MAX(sequence), 0) + 1 FROM events WHERE run_id = ?
}

// getNextSequenceInTx returns the next sequence number within a transaction.
func (db *DB) getNextSequenceInTx(tx *sql.Tx, runID string) (int, error) {
    // Same query as GetNextSequence but uses tx
}

// ListEvents returns all events for a run in sequence order.
func (db *DB) ListEvents(runID string) ([]*EventRecord, error) {
    // SELECT * FROM events WHERE run_id = ? ORDER BY sequence
}

// ListEventsSince returns all events with sequence > the given value.
// Used for incremental event fetching (e.g., for live monitoring).
func (db *DB) ListEventsSince(runID string, sequence int) ([]*EventRecord, error) {
    // SELECT * FROM events WHERE run_id = ? AND sequence > ? ORDER BY sequence
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/db/... -run TestEvent
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestEventAppend` | AppendEvent inserts event with auto-assigned sequence |
| `TestEventSequencing` | Multiple AppendEvent calls produce consecutive sequences starting at 1 |
| `TestEventWithPayload` | AppendEvent serializes payload to JSON |
| `TestEventWithUnitID` | AppendEvent stores optional unit ID |
| `TestEventWithoutRun` | AppendEvent returns error for non-existent run |
| `TestEventList` | ListEvents returns all events in sequence order |
| `TestEventListSince` | ListEventsSince returns only events after given sequence |
| `TestEventGetNextSequence` | GetNextSequence returns 1 for new run, increments after each event |

## NOT In Scope

- Transaction helpers for combined run/event updates (Task #6)
- Event compaction or archival (future enhancement)
- Full-text search on payloads (future enhancement)
