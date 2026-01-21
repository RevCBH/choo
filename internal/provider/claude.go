package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ClaudeProvider implements Provider using the Claude CLI.
// Uses --dangerously-skip-permissions to run without interactive prompts.
type ClaudeProvider struct {
	// command is the path to the claude executable.
	// Defaults to "claude" (resolved via PATH).
	command string
}

// NewClaude creates a Claude provider with the specified command path.
// If command is empty, defaults to "claude".
func NewClaude(command string) *ClaudeProvider {
	if command == "" {
		command = "claude"
	}
	return &ClaudeProvider{command: command}
}

// Invoke executes Claude CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	args := []string{
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude invocation failed: %w", err)
	}
	return nil
}

// Name returns ProviderClaude
func (p *ClaudeProvider) Name() ProviderType {
	return ProviderClaude
}
