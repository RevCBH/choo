---
task: 3
status: pending
backpressure: "go test ./internal/worker/... -run Backpressure"
depends_on: []
---

# Backpressure Command Runner

**Parent spec**: `/specs/WORKER.md`
**Task**: #3 of 8 in implementation plan

## Objective

Implement the backpressure validation command runner that executes per-task validation commands.

## Dependencies

### External Specs (must be implemented)
- DISCOVERY - provides `Task` type

### Task Dependencies (within this unit)
- None (can be implemented in parallel with tasks #1 and #2)

### Package Dependencies
- `os/exec` - for command execution
- `context` - for timeout handling
- `time` - for duration tracking

## Deliverables

### Files to Create

```
internal/worker/
└── backpressure.go    # Backpressure command runner
```

### Types to Implement

```go
// internal/worker/backpressure.go

// BackpressureResult holds the result of a backpressure command
type BackpressureResult struct {
    Success  bool
    Output   string
    Duration time.Duration
    ExitCode int
}
```

### Functions to Implement

```go
// RunBackpressure executes a task's backpressure command
func RunBackpressure(ctx context.Context, command string, workdir string, timeout time.Duration) BackpressureResult {
    // 1. Create timeout context
    // 2. Execute command via sh -c
    // 3. Capture combined stdout/stderr
    // 4. Track duration
    // 5. Extract exit code on failure
    // 6. Return structured result
}

// ValidateTaskComplete checks if task status was updated to complete
func ValidateTaskComplete(task *discovery.Task) bool {
    // Check if task.Status == discovery.TaskStatusComplete
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Backpressure
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunBackpressure_Success` | `result.Success == true`, `result.ExitCode == 0` for `exit 0` |
| `TestRunBackpressure_Failure` | `result.Success == false`, `result.ExitCode == 1` for `exit 1` |
| `TestRunBackpressure_Timeout` | `result.Success == false` for command exceeding timeout |
| `TestRunBackpressure_CapturesOutput` | `result.Output` contains command stdout/stderr |
| `TestRunBackpressure_TracksDuration` | `result.Duration > 0` |
| `TestValidateTaskComplete` | Returns true only when status is complete |

### Test Implementation

```go
func TestRunBackpressure_Success(t *testing.T) {
    result := RunBackpressure(context.Background(), "exit 0", t.TempDir(), time.Minute)

    if !result.Success {
        t.Error("expected success for exit 0")
    }
    if result.ExitCode != 0 {
        t.Errorf("expected exit code 0, got %d", result.ExitCode)
    }
}

func TestRunBackpressure_Failure(t *testing.T) {
    result := RunBackpressure(context.Background(), "exit 1", t.TempDir(), time.Minute)

    if result.Success {
        t.Error("expected failure for exit 1")
    }
    if result.ExitCode != 1 {
        t.Errorf("expected exit code 1, got %d", result.ExitCode)
    }
}

func TestRunBackpressure_Timeout(t *testing.T) {
    result := RunBackpressure(context.Background(), "sleep 10", t.TempDir(), 100*time.Millisecond)

    if result.Success {
        t.Error("expected failure for timeout")
    }
}

func TestRunBackpressure_CapturesOutput(t *testing.T) {
    result := RunBackpressure(context.Background(), "echo hello && echo world >&2", t.TempDir(), time.Minute)

    if !strings.Contains(result.Output, "hello") {
        t.Error("expected stdout to be captured")
    }
    if !strings.Contains(result.Output, "world") {
        t.Error("expected stderr to be captured")
    }
}

func TestRunBackpressure_TracksDuration(t *testing.T) {
    result := RunBackpressure(context.Background(), "sleep 0.1", t.TempDir(), time.Minute)

    if result.Duration < 100*time.Millisecond {
        t.Errorf("expected duration >= 100ms, got %v", result.Duration)
    }
}

func TestValidateTaskComplete_Complete(t *testing.T) {
    task := &discovery.Task{Status: discovery.TaskStatusComplete}
    if !ValidateTaskComplete(task) {
        t.Error("expected true for complete task")
    }
}

func TestValidateTaskComplete_Pending(t *testing.T) {
    task := &discovery.Task{Status: discovery.TaskStatusPending}
    if ValidateTaskComplete(task) {
        t.Error("expected false for pending task")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `exec.CommandContext` for timeout support
- Extract exit code from `*exec.ExitError` when command fails
- Use `CombinedOutput()` to capture both stdout and stderr
- Set `cmd.Dir` to the workdir parameter
- Track duration with `time.Since(start)`

## NOT In Scope

- Retry logic (handled in loop.go, task #5)
- Event emission (handled in worker.go, task #6)
- Baseline checks (separate task #4)
