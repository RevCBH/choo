package provider

import (
	"context"
	"io"
)

// CodexProvider implements Provider using the OpenAI Codex CLI
type CodexProvider struct {
	command string
}

// NewCodex creates a Codex provider with the specified command path.
// If command is empty, defaults to "codex".
func NewCodex(command string) *CodexProvider {
	if command == "" {
		command = "codex"
	}
	return &CodexProvider{command: command}
}

// Invoke executes the Codex CLI with the given prompt.
// NOTE: Full implementation deferred to provider-implementations unit.
func (p *CodexProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	// Placeholder - full implementation in provider-implementations unit
	return nil
}

// Name returns ProviderCodex
func (p *CodexProvider) Name() ProviderType {
	return ProviderCodex
}
