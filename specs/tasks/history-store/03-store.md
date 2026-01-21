# Task 03: Implement Store CRUD Operations

```yaml
task: 03-store
unit: history-store
depends_on: [02-schema]
backpressure: "go test ./internal/history/... -run TestStore -v"
```

## Objective

Implement the `Store` struct with all CRUD operations for runs, events, and graphs.

## Requirements

1. Create `internal/history/store.go` with:

   ```go
   type Store struct {
       db     *sql.DB
       dbPath string
   }

   // Constructor
   func NewStore(dbPath string) (*Store, error)
   func (s *Store) Close() error

   // Run operations
   func (s *Store) CreateRun(cfg RunConfig) (*Run, error)
   func (s *Store) GetRun(id string) (*Run, error)
   func (s *Store) ListRuns(repoPath string, opts ListOptions) (*RunList, error)
   func (s *Store) CompleteRun(id string, result RunResult) (*Run, error)
   func (s *Store) UpdateRunCounts(id string, completed, failed, blocked int) error

   // Event operations
   func (s *Store) InsertEvent(runID string, event EventRecord) (*StoredEvent, error)
   func (s *Store) GetEvents(runID string, opts EventListOptions) (*EventList, error)
   func (s *Store) GetLatestSeq(runID string) (int64, error)

   // Graph operations
   func (s *Store) SaveGraph(runID string, graph GraphData) error
   func (s *Store) GetGraph(runID string) (*GraphData, error)
   ```

2. Error handling:
   - Define `ErrNotFound` for missing resources
   - Define `ErrAlreadyExists` for duplicate creates
   - Wrap SQL errors with context

3. Pagination:
   - `ListRuns` respects `Limit`, `Offset`, `Status` from `ListOptions`
   - `GetEvents` respects `Limit`, `Offset`, `AfterSeq`, `Type`, `Unit` from `EventListOptions`
   - Both return `Total` count and `HasMore` flag

4. Event insertion:
   - Auto-increment `seq` if not provided (use `GetLatestSeq + 1`)
   - Set `time` to `time.Now()` if zero value
   - Redact sensitive fields from payload before storage

5. Payload redaction (implement in store or separate function):
   - Use allow-list approach from HISTORY-TYPES.md
   - Safe fields: `file`, `path`, `worktree`, `branch`, `commit`, `pr_number`, `status`, `duration`, `duration_ms`, `exit_code`, `reason`, `resumed_from_seq`

## Acceptance Criteria

- [ ] All CRUD operations work correctly
- [ ] Pagination returns correct totals and HasMore
- [ ] Events are inserted with correct sequence numbers
- [ ] Payload redaction removes sensitive fields
- [ ] Concurrent access is safe (WAL mode)
- [ ] Tests cover happy path and error cases

## Files to Create/Modify

- `internal/history/store.go` (create)
- `internal/history/store_test.go` (create)
- `internal/history/redact.go` (create - payload redaction)
