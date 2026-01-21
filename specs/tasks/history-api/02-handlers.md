# Task 02: History API Handlers

```yaml
task: 02-handlers
unit: history-api
depends_on: [01-types]
backpressure: "go test ./internal/web/... -run TestHandler -v"
```

## Objective

Implement HTTP handlers for the history query endpoints used by the web UI.

## Requirements

1. Create `internal/web/handlers.go` with:

   ```go
   type Handler struct {
       store *history.Store
   }

   func NewHandler(store *history.Store) *Handler

   // List runs for a repository
   // GET /api/history/runs?repo=...&limit=...&offset=...&status=...
   func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request)

   // Get single run details
   // GET /api/history/runs/{id}
   func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request)

   // Get events for a run
   // GET /api/history/runs/{id}/events?type=...&unit=...&limit=...&offset=...
   func (h *Handler) GetRunEvents(w http.ResponseWriter, r *http.Request)

   // Get dependency graph for a run
   // GET /api/history/runs/{id}/graph
   func (h *Handler) GetRunGraph(w http.ResponseWriter, r *http.Request)
   ```

2. Handler implementations:

   **ListRuns:**
   - Parse and validate query params
   - Query store with options
   - Return JSON RunList response
   - Error: 400 if repo missing, 500 on store error

   **GetRun:**
   - Extract run ID from path
   - Query store for run
   - Return JSON Run response
   - Error: 404 if not found

   **GetRunEvents:**
   - Extract run ID from path
   - Parse and validate query params
   - Query store for events
   - Return JSON EventList response
   - Error: 404 if run not found

   **GetRunGraph:**
   - Extract run ID from path
   - Query store for graph
   - Return JSON GraphData response
   - Error: 404 if not found

3. Helper functions:

   ```go
   // writeJSON writes a JSON response
   func writeJSON(w http.ResponseWriter, status int, data interface{})

   // writeError writes an APIError response
   func writeError(w http.ResponseWriter, status int, message, code string)
   ```

## Acceptance Criteria

- [ ] All four handlers implemented correctly
- [ ] Query parameters parsed and validated
- [ ] Error responses use consistent format
- [ ] Tests cover success and error paths
- [ ] Pagination works correctly

## Files to Create/Modify

- `internal/web/handlers.go` (create)
- `internal/web/handlers_test.go` (create)
