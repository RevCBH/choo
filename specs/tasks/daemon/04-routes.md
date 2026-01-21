# Task 04: HTTP Routes

```yaml
task: 04-routes
unit: daemon
depends_on: [02-daemon]
backpressure: "go test ./internal/daemon/... -run TestHTTP -v"
```

## Objective

Implement HTTP endpoints for the daemon's internal API (CLI-to-daemon communication).

## Requirements

1. Create `internal/daemon/routes.go` with route registration:

   ```go
   func (d *Daemon) registerRoutes(mux *http.ServeMux) {
       // Health
       mux.HandleFunc("GET /health", d.handleHealth)

       // Run management (CLI writes)
       mux.HandleFunc("POST /api/runs", d.handleCreateRun)
       mux.HandleFunc("POST /api/runs/{id}/events", d.handleCreateEvent)
       mux.HandleFunc("POST /api/runs/{id}/complete", d.handleCompleteRun)

       // Run queries (CLI reads)
       mux.HandleFunc("GET /api/runs/{id}", d.handleGetRun)
       mux.HandleFunc("GET /api/runs/{id}/events", d.handleGetEvents)

       // History queries (Web UI reads) - defer to HISTORY-API unit
       // These will be added in the history-api unit

       // Static files (Web UI)
       mux.Handle("GET /", http.FileServer(http.Dir(d.cfg.StaticDir)))
   }
   ```

2. Handler implementations in `internal/daemon/handlers.go`:

   ```go
   // Health check
   func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request)

   // Create a new run
   func (d *Daemon) handleCreateRun(w http.ResponseWriter, r *http.Request)

   // Record an event for a run
   func (d *Daemon) handleCreateEvent(w http.ResponseWriter, r *http.Request)

   // Mark a run as complete
   func (d *Daemon) handleCompleteRun(w http.ResponseWriter, r *http.Request)

   // Get run details
   func (d *Daemon) handleGetRun(w http.ResponseWriter, r *http.Request)

   // Get events for a run
   func (d *Daemon) handleGetEvents(w http.ResponseWriter, r *http.Request)
   ```

3. Request/response handling:
   - Parse JSON request bodies
   - Validate required fields
   - Return appropriate HTTP status codes
   - Use `history.APIError` format for errors

4. Middleware:
   - Request logging (method, path, duration)
   - Recovery from panics
   - CORS headers for web UI (localhost only)

## Acceptance Criteria

- [ ] All endpoints handle requests correctly
- [ ] Error responses use consistent format
- [ ] Request logging works
- [ ] Static file serving works
- [ ] Tests cover success and error cases

## Files to Create/Modify

- `internal/daemon/routes.go` (create)
- `internal/daemon/handlers.go` (create)
- `internal/daemon/middleware.go` (create)
- `internal/daemon/handlers_test.go` (create)
