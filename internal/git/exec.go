package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Runner executes git commands.
type Runner interface {
	Exec(ctx context.Context, dir string, args ...string) (string, error)
	ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error)
}

// osRunner executes real git commands via exec.CommandContext.
type osRunner struct{}

func (osRunner) Exec(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s",
			strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

func (osRunner) ExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(stdin)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s",
			strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

var (
	defaultRunner Runner = osRunner{}
	runnerMu      sync.RWMutex
)

// DefaultRunner returns the current default runner.
func DefaultRunner() Runner {
	runnerMu.RLock()
	defer runnerMu.RUnlock()
	return defaultRunner
}

// SetDefaultRunner replaces the default runner. Intended for tests.
func SetDefaultRunner(runner Runner) {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	if runner == nil {
		defaultRunner = osRunner{}
		return
	}
	defaultRunner = runner
}

// gitExec executes a git command in the specified directory and returns stdout.
// Returns an error with stderr content if the command fails.
func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
	runnerMu.RLock()
	runner := defaultRunner
	runnerMu.RUnlock()
	return runner.Exec(ctx, dir, args...)
}

// gitExecWithStdin executes a git command with stdin input.
// Used for commands that require piped input.
//
//nolint:unused // WIP: will be used for commands requiring stdin input
func gitExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	runnerMu.RLock()
	runner := defaultRunner
	runnerMu.RUnlock()
	return runner.ExecWithStdin(ctx, dir, stdin, args...)
}
