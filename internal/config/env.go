package config

import "os"

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
}

// applyEnvOverrides modifies config in place with environment variable values.
func applyEnvOverrides(cfg *Config) {
	for _, override := range envOverrides {
		if val := os.Getenv(override.envVar); val != "" {
			override.apply(cfg, val)
		}
	}
}
