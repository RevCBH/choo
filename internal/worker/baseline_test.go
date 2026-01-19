package worker

import (
	"context"
	"strings"
	"testing"
	"time"
)

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
