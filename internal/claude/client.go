package claude

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
)

// Client defines the interface for interacting with the Claude CLI
type Client interface {
	// Execute runs the Claude CLI with the given options
	Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error)
}

// CLIClient implements Client by invoking the claude CLI as a subprocess
type CLIClient struct {
	// claudeBinary is the path to the claude binary (default: "claude")
	claudeBinary string
}

// NewCLIClient creates a new CLIClient with default settings
func NewCLIClient() *CLIClient {
	return &CLIClient{
		claudeBinary: "claude",
	}
}

// NewCLIClientWithBinary creates a CLIClient with a custom binary path
func NewCLIClientWithBinary(binary string) *CLIClient {
	return &CLIClient{
		claudeBinary: binary,
	}
}

// Execute runs the Claude CLI with the provided options
func (c *CLIClient) Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
	// Validate inputs
	if opts.Prompt == "" {
		return nil, ErrEmptyPrompt
	}
	if opts.WorkDir == "" {
		return nil, ErrEmptyWorkDir
	}

	// Build command arguments
	args := c.buildArgs(opts)

	// Create command context with timeout if specified
	cmdCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Create exec.CommandContext
	cmd := exec.CommandContext(cmdCtx, c.claudeBinary, args...)
	cmd.Dir = opts.WorkDir

	// Set stdout/stderr
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	// Execute command
	err := cmd.Run()

	// Build result
	result := &ExecuteResult{
		ExitCode: 0,
		Success:  err == nil,
		Error:    err,
	}

	// Extract exit code if available
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}

		// Check if error was due to timeout
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Error = ErrTimeout
			return result, ErrTimeout
		}

		// Wrap non-zero exit as ExecutionError
		if result.ExitCode != 0 {
			execErr := NewExecutionError(result.ExitCode, err)
			result.Error = execErr
			return result, execErr
		}
	}

	return result, nil
}

// buildArgs constructs the command-line arguments for the claude binary
func (c *CLIClient) buildArgs(opts ExecuteOptions) []string {
	args := make([]string, 0, 8)

	// Add dangerously-skip-permissions flag
	if opts.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	// Add prompt
	args = append(args, "-p", opts.Prompt)

	// Add max-turns if specified
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}

	return args
}

// MockClient is a test implementation of Client for testing purposes
type MockClient struct {
	// ExecuteFunc is called when Execute is invoked
	ExecuteFunc func(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error)
}

// Execute delegates to the ExecuteFunc
func (m *MockClient) Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, opts)
	}
	return &ExecuteResult{Success: true}, nil
}
