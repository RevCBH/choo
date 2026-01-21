# Task 03: Register History Routes

```yaml
task: 03-routes
unit: history-api
depends_on: [02-handlers]
backpressure: "go test ./internal/daemon/... -run TestHistoryRoutes -v"
```

## Objective

Register the history API routes in the daemon's HTTP server.

## Requirements

1. Modify `internal/daemon/routes.go` to include history endpoints:

   ```go
   func (d *Daemon) registerRoutes(mux *http.ServeMux) {
       // ... existing routes ...

       // History API endpoints (Web UI reads)
       historyHandler := web.NewHandler(d.store)
       mux.HandleFunc("GET /api/history/runs", historyHandler.ListRuns)
       mux.HandleFunc("GET /api/history/runs/{id}", historyHandler.GetRun)
       mux.HandleFunc("GET /api/history/runs/{id}/events", historyHandler.GetRunEvents)
       mux.HandleFunc("GET /api/history/runs/{id}/graph", historyHandler.GetRunGraph)
   }
   ```

2. Ensure proper ordering:
   - History routes before static file catch-all
   - More specific routes before general ones

3. Add CORS middleware for history endpoints:
   - Allow `Origin: http://localhost:*`
   - Allow methods: GET, OPTIONS
   - Allow headers: Content-Type

4. Add request logging middleware that logs:
   - Method and path
   - Response status code
   - Request duration

## Acceptance Criteria

- [ ] History endpoints accessible from browser
- [ ] CORS allows localhost origins
- [ ] Request logging works
- [ ] Routes don't conflict with existing endpoints
- [ ] Integration test hits all endpoints

## Files to Create/Modify

- `internal/daemon/routes.go` (modify)
- `internal/daemon/middleware.go` (modify - add CORS, logging)
- `internal/daemon/routes_test.go` (modify - add history tests)
