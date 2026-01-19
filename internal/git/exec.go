package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitExec executes a git command in the specified directory and returns stdout.
// Returns an error with stderr content if the command fails.
func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
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

// gitExecWithStdin executes a git command with stdin input.
// Used for commands that require piped input.
func gitExecWithStdin(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
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
