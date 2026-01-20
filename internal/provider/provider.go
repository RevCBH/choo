package provider

import (
	"context"
	"io"
)

// ProviderType identifies which LLM provider to use
type ProviderType string

const (
	// ProviderClaude uses the Claude CLI (default)
	ProviderClaude ProviderType = "claude"

	// ProviderCodex uses the OpenAI Codex CLI
	ProviderCodex ProviderType = "codex"
)

// Provider defines the interface for CLI-based LLM providers
type Provider interface {
	// Invoke executes the provider with the given prompt in the specified workdir.
	// Output is streamed to stdout and stderr writers.
	// Returns an error if the provider fails or context is cancelled.
	Invoke(ctx context.Context, prompt string, workdir string, stdout, stderr io.Writer) error

	// Name returns the provider type identifier
	Name() ProviderType
}

// Config holds provider configuration
type Config struct {
	// Type specifies which provider to use (defaults to "claude" if empty)
	Type ProviderType

	// Command is the path to the provider CLI executable.
	// If empty, uses the default command name ("claude" or "codex").
	Command string
}
