---
task: 2
status: pending
backpressure: "go test ./internal/github/... -run TestGetToken"
depends_on: [1]
---

# PR Client

**Parent spec**: `/specs/GITHUB.md`
**Task**: #2 of 5 in implementation plan

## Objective

Implement the PRClient constructor, GitHub authentication (token retrieval), owner/repo auto-detection from git remote, and HTTP client with rate limiting support.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `PRClient`, `PRClientConfig`)

### Package Dependencies
- `net/http` (standard library)
- `os` (standard library)
- `os/exec` (standard library)
- `strings` (standard library)
- `fmt` (standard library)
- `time` (standard library)
- `encoding/json` (standard library)

## Deliverables

### Files to Create/Modify

```
internal/
└── github/
    ├── client.go       # MODIFY: Add constructor, auth, HTTP methods
    └── client_test.go  # CREATE: Tests for client functionality
```

### Functions to Implement

```go
// internal/github/client.go

// NewPRClient creates a new GitHub PR client
func NewPRClient(cfg PRClientConfig) (*PRClient, error) {
    // 1. Get token via getToken()
    // 2. Auto-detect owner/repo if not provided via detectOwnerRepo()
    // 3. Set defaults for pollInterval (30s) and reviewTimeout (2h)
    // 4. Create http.Client with auth header
}

// getToken retrieves the GitHub token from env or gh CLI
func getToken() (string, error) {
    // 1. Check GITHUB_TOKEN env var
    // 2. Fall back to `gh auth token`
    // 3. Return error if neither available
}

// detectOwnerRepo parses owner/repo from git remote
func detectOwnerRepo() (owner, repo string, err error) {
    // 1. Run `git remote get-url origin`
    // 2. Parse github.com/owner/repo from URL
    // 3. Handle both HTTPS and SSH formats
}

// doRequest executes an HTTP request with rate limit handling
func (c *PRClient) doRequest(ctx context.Context, method, url string, body any) (*http.Response, error) {
    // 1. Marshal body to JSON if present
    // 2. Create request with auth header
    // 3. Execute with exponential backoff retry
    // 4. Handle 403/429 rate limits with Retry-After header
    // 5. Retry 5xx errors, fail on 4xx (except rate limit)
}
```

### Default Values

| Config | Default |
|--------|---------|
| PollInterval | 30 seconds |
| ReviewTimeout | 2 hours |
| Max retries | 5 |
| Initial backoff | 1 second |
| Backoff multiplier | 2x |

## Backpressure

### Validation Command

```bash
go test ./internal/github/... -run TestGetToken -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestGetToken_EnvVariable` | Returns token from GITHUB_TOKEN env |
| `TestGetToken_NoToken` | Returns error when no token available |
| `TestDetectOwnerRepo_HTTPS` | Parses `github.com/owner/repo` from HTTPS URL |
| `TestDetectOwnerRepo_SSH` | Parses `owner/repo` from SSH URL |
| `TestNewPRClient_Defaults` | Sets 30s poll interval and 2h timeout when not provided |
| `TestDoRequest_RateLimitRetry` | Retries on 429, succeeds on subsequent attempt |

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| None | N/A | Uses env vars and mocked exec |

### CI Compatibility

- [x] No external API keys required (tests use env mocking)
- [x] No network access required (tests use httptest)
- [x] Runs in <60 seconds

## Implementation Notes

### Token Retrieval Order

```go
func getToken() (string, error) {
    // 1. Check environment first
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        return token, nil
    }

    // 2. Try gh CLI
    cmd := exec.Command("gh", "auth", "token")
    out, err := cmd.Output()
    if err == nil {
        return strings.TrimSpace(string(out)), nil
    }

    return "", fmt.Errorf("no GitHub token found: set GITHUB_TOKEN or run 'gh auth login'")
}
```

### Remote URL Parsing

Handle both formats:
- HTTPS: `https://github.com/owner/repo.git`
- SSH: `git@github.com:owner/repo.git`

### Rate Limit Handling

Check `Retry-After` header first, fall back to exponential backoff:

```go
if resp.StatusCode == 403 || resp.StatusCode == 429 {
    if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
        // Parse and sleep
    }
}
```

## NOT In Scope

- Review polling (Task #3)
- PR CRUD operations (Task #4)
- Comment fetching (Task #5)
- Event emission (future integration)
