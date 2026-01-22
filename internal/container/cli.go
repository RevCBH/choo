package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CLIManager implements Manager using docker/podman CLI.
type CLIManager struct {
	runtime string // "docker" or "podman"
}

// NewCLIManager creates a Manager using the specified runtime.
// Use DetectRuntime() to find an available runtime first.
func NewCLIManager(runtime string) *CLIManager {
	return &CLIManager{runtime: runtime}
}

// Create creates a new container but does not start it.
func (m *CLIManager) Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error) {
	args := []string{"create", "--name", cfg.Name}

	// Add environment variables
	for k, v := range cfg.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Set working directory if specified
	if cfg.WorkDir != "" {
		args = append(args, "-w", cfg.WorkDir)
	}

	// Image and command come last
	args = append(args, cfg.Image)
	args = append(args, cfg.Cmd...)

	cmd := exec.CommandContext(ctx, m.runtime, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to create container: %s", exitErr.Stderr)
		}
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return ContainerID(strings.TrimSpace(string(output))), nil
}

// Start starts a previously created container.
func (m *CLIManager) Start(ctx context.Context, id ContainerID) error {
	cmd := exec.CommandContext(ctx, m.runtime, "start", string(id))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %s", output)
	}

	return nil
}

// Wait blocks until the container exits and returns the exit code.
func (m *CLIManager) Wait(ctx context.Context, id ContainerID) (int, error) {
	cmd := exec.CommandContext(ctx, m.runtime, "wait", string(id))
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return -1, fmt.Errorf("failed to wait for container: %s", exitErr.Stderr)
		}
		return -1, fmt.Errorf("failed to wait for container: %w", err)
	}

	exitCode, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return -1, fmt.Errorf("failed to parse exit code: %w", err)
	}

	return exitCode, nil
}

// Logs returns a stream of container logs (stdout and stderr combined).
func (m *CLIManager) Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error) {
	// -f follows the log output until container exits
	cmd := exec.CommandContext(ctx, m.runtime, "logs", "-f", string(id))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start log streaming: %w", err)
	}

	// Return the pipe; caller is responsible for closing
	// When ctx is canceled, the command will be killed and pipe will close
	return stdout, nil
}

// Stop stops a running container with the specified timeout.
func (m *CLIManager) Stop(ctx context.Context, id ContainerID, timeout time.Duration) error {
	timeoutSecs := int(timeout.Seconds())
	cmd := exec.CommandContext(ctx, m.runtime, "stop", "-t", strconv.Itoa(timeoutSecs), string(id))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop container: %s", output)
	}

	return nil
}

// Remove removes a stopped container.
func (m *CLIManager) Remove(ctx context.Context, id ContainerID) error {
	cmd := exec.CommandContext(ctx, m.runtime, "rm", string(id))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove container: %s", output)
	}

	return nil
}

// Verify CLIManager implements Manager interface
var _ Manager = (*CLIManager)(nil)
