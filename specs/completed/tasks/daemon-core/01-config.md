---
task: 1
status: complete
backpressure: "go build ./internal/daemon/..."
depends_on: []
---

# Daemon Configuration

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #1 of 7 in implementation plan

## Objective

Define the Config struct with sensible defaults and validation for daemon configuration.

## Dependencies

### Task Dependencies (within this unit)
- None (foundational types)

### Package Dependencies
- `os` - for path defaults
- `path/filepath` - for path construction

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── config.go    # CREATE: Configuration types and defaults
```

### Types to Implement

```go
// Config holds daemon configuration with sensible defaults.
type Config struct {
    SocketPath string // Default: ~/.choo/daemon.sock
    PIDFile    string // Default: ~/.choo/daemon.pid
    DBPath     string // Default: ~/.choo/choo.db
    MaxJobs    int    // Default: 10
}
```

### Functions to Implement

```go
// DefaultConfig returns a Config with sensible defaults.
// Paths are resolved relative to the user's home directory.
func DefaultConfig() (*Config, error) {
    // Get home directory
    // Construct default paths under ~/.choo/
    // Return config with defaults
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
    // Ensure MaxJobs > 0
    // Ensure paths are absolute
    // Ensure parent directories can be created
}

// EnsureDirectories creates the directories needed for daemon files.
func (c *Config) EnsureDirectories() error {
    // Create ~/.choo/ directory if needed
    // Set appropriate permissions (0700)
}
```

## Backpressure

### Validation Command

```bash
go build ./internal/daemon/...
```

### Must Pass

| Test | Assertion |
|------|-----------|
| Build succeeds | No compilation errors |
| `DefaultConfig()` | Returns config with non-empty paths |
| `DefaultConfig()` | SocketPath ends with `daemon.sock` |
| `DefaultConfig()` | PIDFile ends with `daemon.pid` |
| `DefaultConfig()` | DBPath ends with `choo.db` |
| `DefaultConfig()` | MaxJobs is 10 |
| `Config{MaxJobs: 0}.Validate()` | Returns error |
| `Config{MaxJobs: 5, ...}.Validate()` | Returns nil for valid config |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `os.UserHomeDir()` for cross-platform home directory detection
- Default MaxJobs of 10 is configurable but reasonable for most workloads
- Directory permissions should be 0700 for security (user-only access)
- SocketPath must be absolute for Unix socket binding

## NOT In Scope

- PID file operations (Task #2)
- Job manager types (Task #3)
- Daemon lifecycle (Task #7)
