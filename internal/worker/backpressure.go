package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/RevCBH/choo/internal/discovery"
)

// BackpressureResult holds the result of a backpressure command
type BackpressureResult struct {
	Success  bool
	Output   string
	Duration time.Duration
	ExitCode int
}

// RunBackpressure executes a task's backpressure command
func RunBackpressure(ctx context.Context, command string, workdir string, timeout time.Duration) BackpressureResult {
	fmt.Fprintf(os.Stderr, "DEBUG RunBackpressure: command=%q workdir=%q timeout=%v\n", command, workdir, timeout)

	// 1. Create timeout context
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// 2. Execute command via sh -c
	cmd := exec.CommandContext(ctxWithTimeout, "sh", "-c", command)
	cmd.Dir = workdir

	// 3. Capture combined stdout/stderr
	output, err := cmd.CombinedOutput()

	// 4. Track duration
	duration := time.Since(start)

	fmt.Fprintf(os.Stderr, "DEBUG RunBackpressure: err=%v duration=%v output=%q\n", err, duration, string(output))

	// 5. Extract exit code on failure
	exitCode := 0
	success := true

	if err != nil {
		success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// For other errors (e.g., timeout), set a non-zero exit code
			fmt.Fprintf(os.Stderr, "DEBUG RunBackpressure: non-exit error type=%T\n", err)
			exitCode = -1
		}
	}

	// 6. Return structured result
	return BackpressureResult{
		Success:  success,
		Output:   string(output),
		Duration: duration,
		ExitCode: exitCode,
	}
}

// ValidateTaskComplete checks if task status was updated to complete
func ValidateTaskComplete(task *discovery.Task) bool {
	return task.Status == discovery.TaskStatusComplete
}
