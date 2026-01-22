package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig holds global choo configuration from ~/.choo/config.yaml.
// This config is user-wide and used for cross-repo operations like `choo feature spec`.
type GlobalConfig struct {
	// Provider settings for LLM providers
	Provider ProviderConfig `yaml:"provider"`

	// DefaultSpecsDir is the default output directory for specs (relative to repo root)
	// Default: "specs"
	DefaultSpecsDir string `yaml:"default_specs_dir"`

	// Skills configuration for user skill overrides
	Skills SkillsConfig `yaml:"skills"`
}

// SkillsConfig holds configuration for user skill overrides.
type SkillsConfig struct {
	// Dir is the directory for user skill overrides
	// Default: ~/.choo/skills
	Dir string `yaml:"dir"`
}

// DefaultGlobalConfig returns a GlobalConfig with default values.
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Provider: ProviderConfig{
			Type: ProviderClaude,
			Providers: map[ProviderType]ProviderSettings{
				ProviderClaude: {Command: "claude"},
				ProviderCodex:  {Command: "codex"},
			},
		},
		DefaultSpecsDir: "specs",
		Skills: SkillsConfig{
			Dir: "~/.choo/skills",
		},
	}
}

// LoadGlobalConfig loads global configuration from ~/.choo/config.yaml.
// If the file doesn't exist, returns default configuration.
func LoadGlobalConfig() (*GlobalConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Can't find home directory, return defaults
		return DefaultGlobalConfig(), nil
	}

	configPath := filepath.Join(homeDir, ".choo", "config.yaml")
	return LoadGlobalConfigFromPath(configPath)
}

// LoadGlobalConfigFromPath loads global configuration from a specific path.
func LoadGlobalConfigFromPath(path string) (*GlobalConfig, error) {
	cfg := DefaultGlobalConfig()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist, use defaults
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ExpandSkillsDir expands ~ in the skills directory path.
func (cfg *GlobalConfig) ExpandSkillsDir() string {
	dir := cfg.Skills.Dir
	if len(dir) > 0 && dir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			dir = filepath.Join(homeDir, dir[1:])
		}
	}
	return dir
}

// GetProviderCommandForType returns the command for a specific provider type from global config.
func (cfg *GlobalConfig) GetProviderCommandForType(providerType ProviderType) string {
	if settings, ok := cfg.Provider.Providers[providerType]; ok && settings.Command != "" {
		return settings.Command
	}

	// Fall back to default command
	if cfg.Provider.Command != "" {
		return cfg.Provider.Command
	}

	// Use provider type as command name
	return string(providerType)
}

// EnsureGlobalConfigDir creates the ~/.choo directory if it doesn't exist.
func EnsureGlobalConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	chooDir := filepath.Join(homeDir, ".choo")
	if err := os.MkdirAll(chooDir, 0755); err != nil {
		return "", err
	}

	return chooDir, nil
}

// EnsureGlobalSkillsDir creates the ~/.choo/skills directory if it doesn't exist.
func EnsureGlobalSkillsDir() (string, error) {
	chooDir, err := EnsureGlobalConfigDir()
	if err != nil {
		return "", err
	}

	skillsDir := filepath.Join(chooDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return "", err
	}

	return skillsDir, nil
}

// EnsureGlobalSpecStateDir creates the ~/.choo/spec-state directory if it doesn't exist.
func EnsureGlobalSpecStateDir() (string, error) {
	chooDir, err := EnsureGlobalConfigDir()
	if err != nil {
		return "", err
	}

	stateDir := filepath.Join(chooDir, "spec-state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", err
	}

	return stateDir, nil
}
