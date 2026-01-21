# Task 01: API Request/Response Types

```yaml
task: 01-types
unit: history-api
depends_on: []
backpressure: "go build ./internal/web/..."
```

## Objective

Implement the HTTP-specific request and response types for the history API.

## Requirements

1. Create `internal/web/types.go` with:

   ```go
   // Request types (HTTP-specific, maps to shared types)

   type ListRunsParams struct {
       Repo   string `json:"repo"`   // Required
       Limit  int    `json:"limit"`  // Default 50, max 100
       Offset int    `json:"offset"` // Default 0
       Status string `json:"status"` // Optional filter
   }

   type ListEventsParams struct {
       Type   string `json:"type"`   // Filter by prefix
       Unit   string `json:"unit"`   // Filter by unit
       Limit  int    `json:"limit"`  // Default 100, max 1000
       Offset int    `json:"offset"` // Default 0
   }

   // Response is just the shared types from history package
   // Re-export for convenience:
   type (
       Run        = history.Run
       RunList    = history.RunList
       Event      = history.StoredEvent
       EventList  = history.EventList
       GraphData  = history.GraphData
       APIError   = history.APIError
   )
   ```

2. Parsing functions:

   ```go
   // ParseListRunsParams extracts params from URL query
   func ParseListRunsParams(r *http.Request) (*ListRunsParams, error)

   // ParseListEventsParams extracts params from URL query
   func ParseListEventsParams(r *http.Request) (*ListEventsParams, error)

   // ToListOptions converts HTTP params to store options
   func (p *ListRunsParams) ToListOptions() history.ListOptions

   // ToEventListOptions converts HTTP params to store options
   func (p *ListEventsParams) ToEventListOptions() history.EventListOptions
   ```

3. Parameter validation:
   - `repo` is required for ListRuns
   - `limit` capped to max values
   - `status` validated against known values
   - Return 400 with APIError on invalid params

## Acceptance Criteria

- [ ] All request types implemented with JSON tags
- [ ] Parsing functions handle query parameters
- [ ] Validation returns appropriate errors
- [ ] Conversion to store options works correctly

## Files to Create/Modify

- `internal/web/types.go` (create)
- `internal/web/types_test.go` (create)
