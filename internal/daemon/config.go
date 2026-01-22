package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds daemon configuration with sensible defaults.
type Config struct {
	SocketPath    string // Default: ~/.choo/daemon.sock
	PIDFile       string // Default: ~/.choo/daemon.pid
	DBPath        string // Default: ~/.choo/choo.db
	MaxJobs       int    // Default: 10
	WebAddr       string // Default: :8080
	WebSocketPath string // Default: ~/.choo/web.sock

	ContainerMode    bool   // Enable container isolation for job execution
	ContainerImage   string // Container image to use, e.g., "choo:latest"
	ContainerRuntime string // "auto", "docker", or "podman"
}

// DefaultConfig returns a Config with sensible defaults.
// Paths are resolved relative to the user's home directory.
func DefaultConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	chooDir := filepath.Join(home, ".choo")

	return &Config{
		SocketPath:    filepath.Join(chooDir, "daemon.sock"),
		PIDFile:       filepath.Join(chooDir, "daemon.pid"),
		DBPath:        filepath.Join(chooDir, "choo.db"),
		MaxJobs:       10,
		WebAddr:       ":8080",
		WebSocketPath: filepath.Join(chooDir, "web.sock"),
	}, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.MaxJobs <= 0 {
		return fmt.Errorf("MaxJobs must be greater than 0, got %d", c.MaxJobs)
	}

	if !filepath.IsAbs(c.SocketPath) {
		return fmt.Errorf("SocketPath must be absolute, got %s", c.SocketPath)
	}

	if !filepath.IsAbs(c.PIDFile) {
		return fmt.Errorf("PIDFile must be absolute, got %s", c.PIDFile)
	}

	if !filepath.IsAbs(c.DBPath) {
		return fmt.Errorf("DBPath must be absolute, got %s", c.DBPath)
	}

	if c.ContainerMode {
		if c.ContainerImage == "" {
			return fmt.Errorf("ContainerImage is required when ContainerMode is enabled")
		}
		if c.ContainerRuntime != "" && c.ContainerRuntime != "auto" &&
			c.ContainerRuntime != "docker" && c.ContainerRuntime != "podman" {
			return fmt.Errorf("ContainerRuntime must be 'auto', 'docker', or 'podman', got %s", c.ContainerRuntime)
		}
	}

	return nil
}

// EnsureDirectories creates the directories needed for daemon files.
func (c *Config) EnsureDirectories() error {
	dirs := make(map[string]bool)

	// Collect unique parent directories
	dirs[filepath.Dir(c.SocketPath)] = true
	dirs[filepath.Dir(c.PIDFile)] = true
	dirs[filepath.Dir(c.DBPath)] = true

	// Create each directory with 0700 permissions
	for dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
