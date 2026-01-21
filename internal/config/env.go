package config

import "os"

const (
	EnvProvider = "RALPH_PROVIDER"
	EnvCodexCmd = "RALPH_CODEX_CMD"
)

// envOverrides maps environment variables to config field setters.
var envOverrides = []struct {
	envVar string
	apply  func(*Config, string)
}{
	{
		envVar: "RALPH_CLAUDE_CMD",
		apply: func(c *Config, v string) {
			c.Claude.Command = v
		},
	},
	{
		envVar: "RALPH_WORKTREE_BASE",
		apply: func(c *Config, v string) {
			c.Worktree.BasePath = v
		},
	},
	{
		envVar: "RALPH_LOG_LEVEL",
		apply: func(c *Config, v string) {
			c.LogLevel = v
		},
	},
	{
		envVar: EnvProvider,
		apply: func(c *Config, v string) {
			c.Provider.Type = ProviderType(v)
		},
	},
	{
		envVar: EnvCodexCmd,
		apply: func(c *Config, v string) {
			if c.Provider.Providers == nil {
				c.Provider.Providers = make(map[ProviderType]ProviderSettings)
			}
			c.Provider.Providers[ProviderCodex] = ProviderSettings{Command: v}
		},
	},
}

// applyEnvOverrides modifies config in place with environment variable values.
func applyEnvOverrides(cfg *Config) {
	for _, override := range envOverrides {
		if val := os.Getenv(override.envVar); val != "" {
			override.apply(cfg, val)
		}
	}
}
