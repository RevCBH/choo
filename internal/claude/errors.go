package claude

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyPrompt indicates Execute was called with an empty prompt
	ErrEmptyPrompt = errors.New("prompt cannot be empty")

	// ErrEmptyWorkDir indicates Execute was called with an empty working directory
	ErrEmptyWorkDir = errors.New("working directory cannot be empty")

	// ErrTimeout indicates the Claude execution exceeded the timeout
	ErrTimeout = errors.New("claude execution timed out")

	// ErrNonZeroExit indicates Claude exited with a non-zero status
	ErrNonZeroExit = errors.New("claude exited with non-zero status")
)

// ExecutionError wraps errors that occur during Claude execution
type ExecutionError struct {
	ExitCode int
	Err      error
}

func (e *ExecutionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("claude execution failed (exit %d): %v", e.ExitCode, e.Err)
	}
	return fmt.Sprintf("claude execution failed (exit %d)", e.ExitCode)
}

func (e *ExecutionError) Unwrap() error {
	return e.Err
}

// NewExecutionError creates an ExecutionError
func NewExecutionError(exitCode int, err error) *ExecutionError {
	return &ExecutionError{
		ExitCode: exitCode,
		Err:      err,
	}
}
