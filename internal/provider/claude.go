package provider

import (
	"context"
	"io"
)

// ClaudeProvider implements Provider using the Claude CLI
type ClaudeProvider struct {
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

// Invoke executes the Claude CLI with the given prompt.
// NOTE: Full implementation deferred to provider-implementations unit.
func (p *ClaudeProvider) Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error {
	// Placeholder - full implementation in provider-implementations unit
	return nil
}

// Name returns ProviderClaude
func (p *ClaudeProvider) Name() ProviderType {
	return ProviderClaude
}
