# Task 03: Daemon Client

```yaml
task: 03-client
unit: daemon
depends_on: [02-daemon]
backpressure: "go test ./internal/daemon/... -run TestClient -v"
```

## Objective

Implement the `Client` for CLI processes to communicate with the daemon via HTTP.

## Requirements

1. Create `internal/daemon/client.go` with:

   ```go
   type Client struct {
       baseURL    string
       httpClient *http.Client
   }

   // Constructor
   func NewClient(baseURL string) *Client
   func Connect() (*Client, error)  // Auto-discover daemon from PID file

   // Health
   func (c *Client) Ping() error
   func (c *Client) IsAvailable() bool

   // Run operations
   func (c *Client) StartRun(cfg history.RunConfig) (*history.Run, error)
   func (c *Client) SendEvent(runID string, event history.EventRecord) (*history.StoredEvent, error)
   func (c *Client) CompleteRun(runID string, result history.RunResult) (*history.Run, error)

   // Query operations (for CLI status commands)
   func (c *Client) GetRun(runID string) (*history.Run, error)
   func (c *Client) ListRuns(repoPath string, opts history.ListOptions) (*history.RunList, error)
   func (c *Client) GetEvents(runID string, opts history.EventListOptions) (*history.EventList, error)
   ```

2. Connection discovery:
   - Read port from `~/.choo/daemon.pid` or use default 8099
   - `Connect()` returns error if daemon not running
   - `IsAvailable()` does a quick health check

3. HTTP client configuration:
   - Reasonable timeouts (10s for operations, 30s for long polls)
   - Keep-alive connections
   - JSON content type for all requests

4. Error handling:
   - Parse API error responses into Go errors
   - Distinguish between network errors and API errors
   - Include response body in error messages for debugging

## Acceptance Criteria

- [ ] Client can connect to running daemon
- [ ] All run operations work correctly
- [ ] Errors are properly propagated
- [ ] Connection discovery works from PID file
- [ ] Client handles daemon restart gracefully

## Files to Create/Modify

- `internal/daemon/client.go` (create)
- `internal/daemon/client_test.go` (create)
