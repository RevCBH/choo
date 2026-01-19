package claude

import (
	"errors"
	"testing"
)

func TestExecutionError_Error(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		err      error
		expected string
	}{
		{
			name:     "with wrapped error",
			exitCode: 1,
			err:      errors.New("command failed"),
			expected: "claude execution failed (exit 1): command failed",
		},
		{
			name:     "without wrapped error",
			exitCode: 2,
			err:      nil,
			expected: "claude execution failed (exit 2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execErr := &ExecutionError{
				ExitCode: tt.exitCode,
				Err:      tt.err,
			}
			if got := execErr.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExecutionError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	execErr := &ExecutionError{
		ExitCode: 1,
		Err:      innerErr,
	}

	unwrapped := execErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}
}

func TestNewExecutionError(t *testing.T) {
	innerErr := errors.New("test error")
	execErr := NewExecutionError(42, innerErr)

	if execErr.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", execErr.ExitCode)
	}
	if execErr.Err != innerErr {
		t.Errorf("Err = %v, want %v", execErr.Err, innerErr)
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrEmptyPrompt",
			err:  ErrEmptyPrompt,
			msg:  "prompt cannot be empty",
		},
		{
			name: "ErrEmptyWorkDir",
			err:  ErrEmptyWorkDir,
			msg:  "working directory cannot be empty",
		},
		{
			name: "ErrTimeout",
			err:  ErrTimeout,
			msg:  "claude execution timed out",
		},
		{
			name: "ErrNonZeroExit",
			err:  ErrNonZeroExit,
			msg:  "claude exited with non-zero status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error constant is nil")
			}
			if tt.err.Error() != tt.msg {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.msg)
			}
		})
	}
}
