package provider

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// CodexProvider implements Provider using the OpenAI Codex CLI.
// Uses exec subcommand with --yolo flag for autonomous execution.
type CodexProvider struct {
	// command is the path to the codex executable.
	// Defaults to "codex" (resolved via PATH).
	command string

	// model is an optional model override.
	model string
}

// NewCodex creates a Codex provider with the specified command path.
// If command is empty, defaults to "codex".
func NewCodex(command string) *CodexProvider {
	if command == "" {
		command = "codex"
	}
	return &CodexProvider{command: command}
}

// SetModel sets an optional model override.
func (p *CodexProvider) SetModel(model string) {
	p.model = model
}

// Command returns the configured CLI command.
func (p *CodexProvider) Command() string {
	return p.command
}

// Model returns the configured model override.
func (p *CodexProvider) Model() string {
	return p.model
}

// Invoke executes Codex CLI with the given prompt.
// The command runs in workdir with stdout/stderr connected to the provided writers.
// Returns when the subprocess exits or context is cancelled.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	args := []string{
		"exec",
		"--yolo",
	}
	if p.model != "" {
		args = append(args, "--model", p.model)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, p.command, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex invocation failed: %w", err)
	}
	return nil
}

// Name returns ProviderCodex
func (p *CodexProvider) Name() ProviderType {
	return ProviderCodex
}
