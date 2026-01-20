package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProvider_ForceTakesPrecedence(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Type: ProviderClaude,
			Providers: map[ProviderType]ProviderSettings{
				ProviderClaude: {Command: "claude"},
				ProviderCodex:  {Command: "codex"},
			},
		},
	}

	ctx := ProviderResolutionContext{
		ForceTaskProvider: "codex",
		UnitProvider:      "claude",
		CLIProvider:       "claude",
		EnvProvider:       "claude",
		ConfigProvider:    ProviderClaude,
	}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderCodex, result.Type)
	assert.Equal(t, "codex", result.Command)
	assert.Equal(t, "cli:--force-task-provider", result.Source)
}

func TestResolveProvider_UnitOverridesDefault(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Type: ProviderClaude,
			Providers: map[ProviderType]ProviderSettings{
				ProviderClaude: {Command: "claude"},
				ProviderCodex:  {Command: "codex"},
			},
		},
	}

	ctx := ProviderResolutionContext{
		UnitProvider:   "codex",
		CLIProvider:    "claude",
		EnvProvider:    "claude",
		ConfigProvider: ProviderClaude,
	}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderCodex, result.Type)
	assert.Equal(t, "codex", result.Command)
	assert.Equal(t, "unit:frontmatter", result.Source)
}

func TestResolveProvider_CLIPrecedenceOverEnv(t *testing.T) {
	cfg := &Config{
		Provider: DefaultProviderConfig(),
	}

	ctx := ProviderResolutionContext{
		CLIProvider: "codex",
		EnvProvider: "claude",
	}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderCodex, result.Type)
	assert.Equal(t, "cli:--provider", result.Source)
}

func TestResolveProvider_EnvPrecedenceOverConfig(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Type:      ProviderClaude,
			Providers: DefaultProviderConfig().Providers,
		},
	}

	ctx := ProviderResolutionContext{
		EnvProvider:    "codex",
		ConfigProvider: ProviderClaude,
	}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderCodex, result.Type)
	assert.Equal(t, "env:RALPH_PROVIDER", result.Source)
}

func TestResolveProvider_ConfigFallback(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Type:      ProviderCodex,
			Providers: DefaultProviderConfig().Providers,
		},
	}

	ctx := ProviderResolutionContext{}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderCodex, result.Type)
	assert.Equal(t, "config:.choo.yaml", result.Source)
}

func TestResolveProvider_DefaultFallback(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Providers: DefaultProviderConfig().Providers,
		},
	}

	ctx := ProviderResolutionContext{}

	result := ResolveProvider(ctx, cfg)

	assert.Equal(t, ProviderClaude, result.Type)
	assert.Equal(t, "claude", result.Command)
	assert.Equal(t, "default", result.Source)
}

func TestGetProviderCommand_CustomPath(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{
			Providers: map[ProviderType]ProviderSettings{
				ProviderCodex: {Command: "/custom/path/codex"},
			},
		},
	}

	cmd := GetProviderCommand(cfg, ProviderCodex)
	assert.Equal(t, "/custom/path/codex", cmd)
}

func TestGetProviderCommand_EnvOverride(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{},
	}

	t.Setenv("RALPH_CODEX_CMD", "/env/codex")

	cmd := GetProviderCommand(cfg, ProviderCodex)
	assert.Equal(t, "/env/codex", cmd)
}

func TestGetProviderCommand_LegacyClaudeConfig(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{},
		Claude: ClaudeConfig{
			Command: "/legacy/claude",
		},
	}

	cmd := GetProviderCommand(cfg, ProviderClaude)
	assert.Equal(t, "/legacy/claude", cmd)
}

func TestValidateProviderType(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{"claude", false},
		{"codex", false},
		{"", false}, // Empty is valid
		{"gemini", true},
		{"CLAUDE", true}, // Case-sensitive
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			err := ValidateProviderType(tt.provider)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid provider type")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
