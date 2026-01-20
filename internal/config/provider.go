package config

import (
	"fmt"
	"os"
)

// ProviderResolutionContext holds all inputs needed to resolve the provider.
type ProviderResolutionContext struct {
	// ForceTaskProvider from CLI --force-task-provider flag
	ForceTaskProvider string

	// UnitProvider from unit frontmatter
	UnitProvider string

	// CLIProvider from CLI --provider flag
	CLIProvider string

	// EnvProvider from RALPH_PROVIDER environment variable
	EnvProvider string

	// ConfigProvider from .choo.yaml provider.type
	ConfigProvider ProviderType
}

// ResolvedProvider contains the final provider selection and its command.
type ResolvedProvider struct {
	// Type is the resolved provider type
	Type ProviderType

	// Command is the binary path to execute
	Command string

	// Source indicates where the provider was determined from
	Source string
}

// ResolveProvider determines the provider to use based on precedence rules.
// Precedence (highest to lowest):
// 1. ForceTaskProvider (--force-task-provider CLI flag)
// 2. UnitProvider (per-unit frontmatter)
// 3. CLIProvider (--provider CLI flag)
// 4. EnvProvider (RALPH_PROVIDER environment variable)
// 5. ConfigProvider (.choo.yaml provider.type)
// 6. Default (claude)
func ResolveProvider(ctx ProviderResolutionContext, cfg *Config) ResolvedProvider {
	var providerType ProviderType
	var source string

	// Level 1: --force-task-provider (highest precedence)
	if ctx.ForceTaskProvider != "" {
		providerType = ProviderType(ctx.ForceTaskProvider)
		source = "cli:--force-task-provider"
	} else if ctx.UnitProvider != "" {
		// Level 2: Per-unit frontmatter
		providerType = ProviderType(ctx.UnitProvider)
		source = "unit:frontmatter"
	} else if ctx.CLIProvider != "" {
		// Level 3: --provider CLI arg
		providerType = ProviderType(ctx.CLIProvider)
		source = "cli:--provider"
	} else if ctx.EnvProvider != "" {
		// Level 4: RALPH_PROVIDER env var
		providerType = ProviderType(ctx.EnvProvider)
		source = "env:RALPH_PROVIDER"
	} else if cfg.Provider.Type != "" {
		// Level 5: .choo.yaml provider.type
		providerType = cfg.Provider.Type
		source = "config:.choo.yaml"
	} else {
		// Default fallback
		providerType = DefaultProviderType
		source = "default"
	}

	return ResolvedProvider{
		Type:    providerType,
		Command: GetProviderCommand(cfg, providerType),
		Source:  source,
	}
}

// GetProviderCommand returns the command for a given provider type.
// Resolution order:
// 1. provider.providers[type].command in config
// 2. Environment variable (RALPH_CODEX_CMD for codex)
// 3. Legacy config field (claude.command for Claude)
// 4. Default constant
func GetProviderCommand(cfg *Config, providerType ProviderType) string {
	// Check per-provider settings first
	if settings, ok := cfg.Provider.Providers[providerType]; ok && settings.Command != "" {
		return settings.Command
	}

	// Check environment variable for codex
	if providerType == ProviderCodex {
		if cmd := os.Getenv(EnvCodexCmd); cmd != "" {
			return cmd
		}
		return DefaultCodexCommand
	}

	// For Claude, check legacy ClaudeConfig first
	if providerType == ProviderClaude {
		if cfg.Claude.Command != "" {
			return cfg.Claude.Command
		}
		return DefaultClaudeCommand
	}

	// Unknown provider, return type as command (will fail at execution)
	return string(providerType)
}

// ValidateProviderType checks if a provider type string is valid.
// Returns an error if the provider is not supported.
// Empty string is valid (uses default).
func ValidateProviderType(provider string) error {
	switch ProviderType(provider) {
	case ProviderClaude, ProviderCodex:
		return nil
	case "":
		return nil // Empty is valid (uses default)
	default:
		return fmt.Errorf("invalid provider type %q: must be one of: claude, codex", provider)
	}
}
