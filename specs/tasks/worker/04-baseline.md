---
task: 4
status: pending
backpressure: "go test ./internal/worker/... -run Baseline"
depends_on: []
---

# Baseline Checks Runner

**Parent spec**: `/specs/WORKER.md`
**Task**: #4 of 8 in implementation plan

## Objective

Implement the baseline checks runner that executes project-wide quality gates (fmt, vet, typecheck) after all tasks in a unit complete.

## Dependencies

### External Specs (must be implemented)
- None (uses types from task #1, but can stub for testing)

### Task Dependencies (within this unit)
- None (can be implemented in parallel with other early tasks)

### Package Dependencies
- `os/exec` - for command execution
- `context` - for timeout handling
- `strings` - for output joining

## Deliverables

### Files to Create

```
internal/worker/
└── baseline.go    # Baseline checks runner
```

### Types to Implement

```go
// internal/worker/baseline.go

// BaselineCheckResult holds results for a single check
type BaselineCheckResult struct {
    Check  BaselineCheck
    Passed bool
    Output string
}
```

### Functions to Implement

```go
// RunBaselineChecks executes all baseline checks for the unit
// Returns (allPassed, combinedFailureOutput)
func RunBaselineChecks(ctx context.Context, checks []BaselineCheck, workdir string, timeout time.Duration) (bool, string) {
    // 1. Create timeout context for entire baseline check phase
    // 2. Iterate through checks
    // 3. For each check, run via sh -c
    // 4. Collect failures with formatted output: "=== checkName ===\noutput"
    // 5. Return whether all passed and combined failure output
}

// RunSingleBaselineCheck executes one baseline check and returns the result
func RunSingleBaselineCheck(ctx context.Context, check BaselineCheck, workdir string) BaselineCheckResult {
    // 1. Execute check.Command via sh -c
    // 2. Capture output
    // 3. Return structured result
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/worker/... -run Baseline
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestRunBaselineChecks_AllPass` | Returns `(true, "")` when all checks exit 0 |
| `TestRunBaselineChecks_SomeFail` | Returns `(false, output)` with failure details |
| `TestRunBaselineChecks_Timeout` | Returns failure when checks exceed timeout |
| `TestRunBaselineChecks_Empty` | Returns `(true, "")` for empty check list |
| `TestRunSingleBaselineCheck_Pass` | `result.Passed == true`, `result.Output` captured |
| `TestRunSingleBaselineCheck_Fail` | `result.Passed == false`, `result.Output` has error |

### Test Implementation

```go
func TestRunBaselineChecks_AllPass(t *testing.T) {
    checks := []BaselineCheck{
        {Name: "check1", Command: "exit 0"},
        {Name: "check2", Command: "exit 0"},
    }

    passed, output := RunBaselineChecks(context.Background(), checks, t.TempDir(), time.Minute)

    if !passed {
        t.Error("expected all checks to pass")
    }
    if output != "" {
        t.Errorf("expected empty output, got %q", output)
    }
}

func TestRunBaselineChecks_SomeFail(t *testing.T) {
    checks := []BaselineCheck{
        {Name: "passing", Command: "exit 0"},
        {Name: "failing", Command: "echo 'error message' && exit 1"},
    }

    passed, output := RunBaselineChecks(context.Background(), checks, t.TempDir(), time.Minute)

    if passed {
        t.Error("expected failure when a check fails")
    }
    if !strings.Contains(output, "=== failing ===") {
        t.Error("output should contain check name header")
    }
    if !strings.Contains(output, "error message") {
        t.Error("output should contain error message")
    }
}

func TestRunBaselineChecks_Timeout(t *testing.T) {
    checks := []BaselineCheck{
        {Name: "slow", Command: "sleep 10"},
    }

    passed, _ := RunBaselineChecks(context.Background(), checks, t.TempDir(), 100*time.Millisecond)

    if passed {
        t.Error("expected failure on timeout")
    }
}

func TestRunBaselineChecks_Empty(t *testing.T) {
    passed, output := RunBaselineChecks(context.Background(), []BaselineCheck{}, t.TempDir(), time.Minute)

    if !passed {
        t.Error("empty checks should pass")
    }
    if output != "" {
        t.Error("empty checks should have no output")
    }
}

func TestRunSingleBaselineCheck_Pass(t *testing.T) {
    check := BaselineCheck{Name: "test", Command: "echo 'success'"}

    result := RunSingleBaselineCheck(context.Background(), check, t.TempDir())

    if !result.Passed {
        t.Error("expected pass")
    }
    if !strings.Contains(result.Output, "success") {
        t.Error("expected output to be captured")
    }
}

func TestRunSingleBaselineCheck_Fail(t *testing.T) {
    check := BaselineCheck{Name: "test", Command: "echo 'failure' >&2 && exit 1"}

    result := RunSingleBaselineCheck(context.Background(), check, t.TempDir())

    if result.Passed {
        t.Error("expected failure")
    }
    if !strings.Contains(result.Output, "failure") {
        t.Error("expected stderr to be captured")
    }
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `exec.CommandContext` for timeout support
- Format failures with check name headers for easy identification
- Join multiple failures with double newlines for readability
- Set `cmd.Dir` to the workdir parameter
- The Pattern field in BaselineCheck is for future file filtering (not used in MVP)

## NOT In Scope

- Baseline fix prompt construction (task #2)
- Baseline fix retry loop (task #6)
- Event emission (task #6)
- Pattern-based file filtering
