# CI — GitHub Actions CI Workflow

## Overview

The CI component provides automated testing for pull requests and the main branch via GitHub Actions. It ensures code quality through build verification, unit testing with race detection, static analysis (vet), and linting before changes can be merged.

This component addresses the need for confidence when self-hosting: contributors must know that changes don't break the build before merging. The CI workflow runs automatically on every PR and push to main, providing fast feedback on code quality.

```
┌─────────────────────────────────────────────────────────────┐
│                    GitHub Actions CI                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────┐    ┌──────────────────────┐       │
│  │      test job        │    │      lint job        │       │
│  ├──────────────────────┤    ├──────────────────────┤       │
│  │ • checkout           │    │ • checkout           │       │
│  │ • setup-go           │    │ • setup-go           │       │
│  │ • go build           │    │ • golangci-lint      │       │
│  │ • go test -race      │    └──────────────────────┘       │
│  │ • go vet             │                                   │
│  └──────────────────────┘                                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Status Check API                          │
├─────────────────────────────────────────────────────────────┤
│  GetCheckStatus(ref) → pending | success | failure          │
└─────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. CI workflow triggers on all pull requests targeting main branch
2. CI workflow triggers on all pushes to main branch
3. Build job compiles all packages and reports errors
4. Test job runs all tests with race detection enabled
5. Test job generates coverage report
6. Vet job runs static analysis on all packages
7. Lint job runs golangci-lint with latest version
8. Status check API reports aggregated check status (pending/success/failure)
9. Configuration option to require CI pass before merge

### Performance Requirements

| Metric | Target |
|--------|--------|
| CI feedback time | < 5 minutes |
| Parallel job execution | 2 concurrent jobs |
| Time to merge after approval | < 5 min (no conflicts) |

### Constraints

- Requires GitHub Actions enabled on repository
- Go version must match project requirements (1.22+)
- golangci-lint version managed by action, not pinned
- Status checks depend on GitHub API availability

## Design

### Module Structure

```
.github/
└── workflows/
    └── ci.yml           # GitHub Actions workflow definition

internal/
└── github/
    └── checks.go        # Status check integration
```

### Workflow Configuration

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build
        run: go build -v ./...

      - name: Test
        run: go test -v -race -coverprofile=coverage.txt ./...

      - name: Vet
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

### Core Types

```go
// internal/github/checks.go

// CheckStatus represents the aggregated status of CI checks for a commit
type CheckStatus string

const (
    // CheckPending indicates one or more checks are still running
    CheckPending CheckStatus = "pending"
    // CheckSuccess indicates all checks completed successfully
    CheckSuccess CheckStatus = "success"
    // CheckFailure indicates one or more checks failed
    CheckFailure CheckStatus = "failure"
)

// CheckRun represents a single GitHub Actions check run
type CheckRun struct {
    ID         int64  `json:"id"`
    Name       string `json:"name"`
    Status     string `json:"status"`      // queued, in_progress, completed
    Conclusion string `json:"conclusion"`  // success, failure, cancelled, skipped
}

// CheckRunsResponse represents the GitHub API response for check runs
type CheckRunsResponse struct {
    TotalCount int        `json:"total_count"`
    CheckRuns  []CheckRun `json:"check_runs"`
}
```

### API Surface

```go
// GetCheckStatus returns the aggregated status of all CI checks for a git ref.
// Returns CheckPending if any check is still running, CheckFailure if any check
// failed, or CheckSuccess if all checks completed successfully.
func (c *Client) GetCheckStatus(ctx context.Context, ref string) (CheckStatus, error)

// getCheckRuns fetches all check runs for a git ref from the GitHub API.
func (c *Client) getCheckRuns(ctx context.Context, ref string) ([]CheckRun, error)

// WaitForChecks polls check status until all checks complete or timeout.
func (c *Client) WaitForChecks(ctx context.Context, ref string, timeout time.Duration) (CheckStatus, error)
```

### Status Check Implementation

```go
// internal/github/checks.go

// GetCheckStatus aggregates the status of all check runs for a ref.
// Logic:
// - If any check has conclusion "failure" → return CheckFailure
// - If any check has status != "completed" → return CheckPending
// - Otherwise → return CheckSuccess
func (c *Client) GetCheckStatus(ctx context.Context, ref string) (CheckStatus, error) {
    runs, err := c.getCheckRuns(ctx, ref)
    if err != nil {
        return "", fmt.Errorf("get check runs: %w", err)
    }

    allComplete := true
    anyFailed := false

    for _, run := range runs {
        if run.Status != "completed" {
            allComplete = false
        }
        if run.Conclusion == "failure" {
            anyFailed = true
        }
    }

    if anyFailed {
        return CheckFailure, nil
    }
    if !allComplete {
        return CheckPending, nil
    }
    return CheckSuccess, nil
}

// getCheckRuns fetches check runs from GitHub API
func (c *Client) getCheckRuns(ctx context.Context, ref string) ([]CheckRun, error) {
    url := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs", c.owner, c.repo, ref)

    var response CheckRunsResponse
    if err := c.get(ctx, url, &response); err != nil {
        return nil, err
    }

    return response.CheckRuns, nil
}

// WaitForChecks polls until checks complete or context times out
func (c *Client) WaitForChecks(ctx context.Context, ref string, pollInterval time.Duration) (CheckStatus, error) {
    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return "", ctx.Err()
        case <-ticker.C:
            status, err := c.GetCheckStatus(ctx, ref)
            if err != nil {
                return "", err
            }
            if status != CheckPending {
                return status, nil
            }
        }
    }
}
```

### Configuration Integration

```go
// internal/config/config.go

type ReviewConfig struct {
    PollInterval time.Duration `yaml:"poll_interval"` // Default: 30s
    RequireCI    bool          `yaml:"require_ci"`    // Default: true
}

// Example configuration in choo.yaml:
// review:
//   poll_interval: 30s
//   require_ci: true
```

## Implementation Notes

### Race Detection

The `-race` flag enables Go's race detector during tests. This catches data races at runtime but increases test execution time by 2-10x and memory usage by 5-10x. For CI, this tradeoff is acceptable because correctness matters more than speed.

### Coverage Reports

Coverage is written to `coverage.txt` in the runner's workspace. Future enhancement: upload to coverage service (Codecov, Coveralls). For now, the file is discarded after the workflow completes.

### Golangci-lint Configuration

The action uses `version: latest` which automatically picks up the most recent stable release. For reproducibility, consider pinning to a specific version (e.g., `version: v1.57.2`). The linter respects `.golangci.yml` in the repository root if present.

### Check Run Timing

GitHub Actions creates check runs asynchronously. When a PR is first opened, there may be a brief window where no check runs exist yet. The `GetCheckStatus` function returns `CheckSuccess` if no runs are found, which could be incorrect. Consider adding a minimum wait or checking for expected workflow names.

### Error Handling

The status check API should not block merges on transient GitHub API errors. Consider:
- Retry logic with exponential backoff
- Timeout handling
- Fallback to manual merge approval if API is unavailable

## Testing Strategy

### Unit Tests

```go
// internal/github/checks_test.go

func TestGetCheckStatus_AllSuccess(t *testing.T) {
    runs := []CheckRun{
        {Name: "test", Status: "completed", Conclusion: "success"},
        {Name: "lint", Status: "completed", Conclusion: "success"},
    }

    client := newTestClient(t, runs)

    status, err := client.GetCheckStatus(context.Background(), "abc123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if status != CheckSuccess {
        t.Errorf("expected CheckSuccess, got %s", status)
    }
}

func TestGetCheckStatus_OnePending(t *testing.T) {
    runs := []CheckRun{
        {Name: "test", Status: "completed", Conclusion: "success"},
        {Name: "lint", Status: "in_progress", Conclusion: ""},
    }

    client := newTestClient(t, runs)

    status, err := client.GetCheckStatus(context.Background(), "abc123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if status != CheckPending {
        t.Errorf("expected CheckPending, got %s", status)
    }
}

func TestGetCheckStatus_OneFailure(t *testing.T) {
    runs := []CheckRun{
        {Name: "test", Status: "completed", Conclusion: "failure"},
        {Name: "lint", Status: "completed", Conclusion: "success"},
    }

    client := newTestClient(t, runs)

    status, err := client.GetCheckStatus(context.Background(), "abc123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if status != CheckFailure {
        t.Errorf("expected CheckFailure, got %s", status)
    }
}

func TestGetCheckStatus_FailureTakesPrecedence(t *testing.T) {
    // Even if some checks are pending, a failure is immediately reported
    runs := []CheckRun{
        {Name: "test", Status: "completed", Conclusion: "failure"},
        {Name: "lint", Status: "in_progress", Conclusion: ""},
    }

    client := newTestClient(t, runs)

    status, err := client.GetCheckStatus(context.Background(), "abc123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if status != CheckFailure {
        t.Errorf("expected CheckFailure, got %s", status)
    }
}

func TestWaitForChecks_Timeout(t *testing.T) {
    runs := []CheckRun{
        {Name: "test", Status: "in_progress", Conclusion: ""},
    }

    client := newTestClient(t, runs)

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    _, err := client.WaitForChecks(ctx, "abc123", 10*time.Millisecond)
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("expected deadline exceeded, got %v", err)
    }
}
```

### Integration Tests

- Create PR and verify CI triggers automatically
- Verify both `test` and `lint` jobs run in parallel
- Introduce test failure and verify status reports failure
- Introduce lint violation and verify status reports failure
- Fix issues and verify status transitions to success

### Manual Testing

- [ ] Push to main triggers CI workflow
- [ ] Open PR triggers CI workflow
- [ ] Build step catches compilation errors
- [ ] Test step catches test failures
- [ ] Test step catches race conditions
- [ ] Vet step catches static analysis issues
- [ ] Lint step catches style violations
- [ ] All jobs run in parallel (not sequentially)
- [ ] Status checks appear on PR
- [ ] Merge blocked when `require_ci: true` and checks fail

## Design Decisions

### Why Separate Test and Lint Jobs?

Running test and lint as separate parallel jobs provides:
1. Faster feedback - lint failures visible without waiting for tests
2. Clearer failure attribution - immediately obvious which check failed
3. Independent retry - can re-run just the failed job

The tradeoff is redundant checkout and setup-go steps, adding ~30 seconds total. This is acceptable for the clarity benefit.

### Why Race Detection in CI Only?

Race detection has significant overhead (2-10x slower, 5-10x memory). Running it locally slows development iteration. CI is the ideal place because:
1. Runs on every PR regardless of developer discipline
2. Has dedicated compute resources
3. Speed matters less than correctness in CI

### Why Not Pin Golangci-lint Version?

Using `version: latest` ensures new checks are picked up automatically. The tradeoff is potential surprise failures when new rules are added. For a self-hosted project, staying current with best practices outweighs the minor inconvenience of occasional lint updates.

### Why Poll-Based Status Checking?

GitHub webhooks would be more efficient but require:
1. Public endpoint to receive webhooks
2. Webhook secret management
3. Additional infrastructure

Poll-based checking (every 30s by default) is simpler and sufficient for the target merge time of <5 minutes.

## Future Enhancements

1. **Coverage upload** - Integrate with Codecov or Coveralls for trend tracking
2. **Caching** - Add Go module cache to speed up builds
3. **Matrix testing** - Test against multiple Go versions
4. **Required checks** - Configure branch protection via API
5. **Webhook integration** - Replace polling with webhooks for faster feedback
6. **Custom check annotations** - Report test failures as inline PR annotations

## References

- PRD Section 8: CI Workflow
- PRD Section 10: Configuration (`review.require_ci`)
- PRD Section 13: Success Criteria (Time to merge target)
- [GitHub Actions documentation](https://docs.github.com/en/actions)
- [golangci-lint action](https://github.com/golangci/golangci-lint-action)
