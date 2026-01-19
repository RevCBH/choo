package claude

import (
	"io"
	"time"
)

// ExecuteOptions configures Claude CLI execution
type ExecuteOptions struct {
	// Prompt is the instruction to send to Claude
	Prompt string

	// WorkDir is the working directory for the claude command
	WorkDir string

	// MaxTurns limits the number of conversation turns
	MaxTurns int

	// DangerouslySkipPermissions skips permission prompts
	DangerouslySkipPermissions bool

	// Timeout specifies the maximum execution time
	Timeout time.Duration

	// Stdout captures standard output (nil inherits parent)
	Stdout io.Writer

	// Stderr captures standard error (nil inherits parent)
	Stderr io.Writer
}

// ExecuteResult contains the result of Claude execution
type ExecuteResult struct {
	// ExitCode is the process exit code
	ExitCode int

	// Success indicates if the execution completed successfully
	Success bool

	// Error is any error that occurred during execution
	Error error
}

// DefaultExecuteOptions returns ExecuteOptions with sensible defaults
func DefaultExecuteOptions() ExecuteOptions {
	return ExecuteOptions{
		DangerouslySkipPermissions: true,
		MaxTurns:                   10,
		Timeout:                    10 * time.Minute,
	}
}
