---
task: 6
status: pending
backpressure: "go test ./internal/config/... -v"
depends_on: [2, 3, 4, 5]
---

# LoadConfig Function

**Parent spec**: `/specs/CONFIG.md`
**Task**: #6 of 6 in implementation plan

## Objective

Implement the main LoadConfig() function that orchestrates YAML loading, defaults, env overrides, path resolution, GitHub detection, and validation.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #2 must be complete (provides: DefaultConfig())
- Task #3 must be complete (provides: applyEnvOverrides())
- Task #4 must be complete (provides: validateConfig())
- Task #5 must be complete (provides: detectGitHubRepo())

### Package Dependencies
- `gopkg.in/yaml.v3`
- `os` (standard library)
- `path/filepath` (standard library)
- `fmt` (standard library)

## Deliverables

### Files to Modify

```
internal/
└── config/
    └── config.go    # MODIFY: Add LoadConfig function
```

### Functions to Implement

```go
// LoadConfig loads configuration from the repository root.
// It applies defaults, then file values, then environment overrides,
// then validates and auto-detects values.
//
// Parameters:
//   - repoRoot: absolute path to the repository root directory
//
// Returns the validated Config or an error if validation fails.
func LoadConfig(repoRoot string) (*Config, error) {
    cfg := DefaultConfig()

    // Try to load config file (optional)
    configPath := filepath.Join(repoRoot, ".choo.yaml")
    if data, err := os.ReadFile(configPath); err == nil {
        if err := yaml.Unmarshal(data, cfg); err != nil {
            return nil, fmt.Errorf("parse config: %w", err)
        }
    }
    // Note: missing config file is not an error (use defaults)

    // Apply environment variable overrides
    applyEnvOverrides(cfg)

    // Resolve relative paths
    if !filepath.IsAbs(cfg.Worktree.BasePath) {
        cfg.Worktree.BasePath = filepath.Join(repoRoot, cfg.Worktree.BasePath)
    }

    // Auto-detect GitHub owner/repo if set to "auto"
    if cfg.GitHub.Owner == "auto" || cfg.GitHub.Repo == "auto" {
        owner, repo, err := detectGitHubRepo(repoRoot)
        if err != nil {
            return nil, fmt.Errorf("auto-detect github: %w", err)
        }
        if cfg.GitHub.Owner == "auto" {
            cfg.GitHub.Owner = owner
        }
        if cfg.GitHub.Repo == "auto" {
            cfg.GitHub.Repo = repo
        }
    }

    // Validate
    if err := validateConfig(cfg); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }

    return cfg, nil
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/config/... -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestLoadConfig_Defaults` | No config file, uses all defaults |
| `TestLoadConfig_FileOverrides` | Config file values override defaults |
| `TestLoadConfig_EnvOverrides` | Env vars override file values |
| `TestLoadConfig_PathResolution` | Relative worktree path becomes absolute |
| `TestLoadConfig_GitHubAutoDetect` | GitHub owner/repo detected from git remote |
| `TestLoadConfig_GitHubExplicit` | Explicit GitHub values not overwritten |
| `TestLoadConfig_InvalidYAML` | Returns parse error for malformed YAML |
| `TestLoadConfig_ValidationError` | Returns validation error for invalid values |
| `TestLoadConfig_NoGitRemote` | Returns error when auto-detect fails and GitHub is "auto" |
| `TestLoadConfig_PartialConfig` | Partial config merges with defaults correctly |
| `TestLoadConfig_BaselineChecks` | Baseline checks parsed from YAML |
| `TestLoadConfig_ConditionalCommands` | Setup/teardown commands with "if" conditions parsed |

### Test File to Create/Modify

```
internal/
└── config/
    └── config_test.go    # CREATE: Tests for LoadConfig
```

### Test Helpers

```go
// writeFile creates a file with the given content for testing
func writeFile(t *testing.T, path, content string) {
    t.Helper()
    err := os.WriteFile(path, []byte(content), 0644)
    require.NoError(t, err)
}

// initGitRepo creates a git repo with the given remote URL for testing
func initGitRepo(t *testing.T, dir, remoteURL string) {
    t.Helper()
    cmd := exec.Command("git", "init")
    cmd.Dir = dir
    require.NoError(t, cmd.Run())

    cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
    cmd.Dir = dir
    require.NoError(t, cmd.Run())
}
```

### Test Fixtures

| Fixture | Location | Purpose |
|---------|----------|---------|
| Temp git repos | `t.TempDir()` | Each test creates fresh temp directory |

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Config file `.choo.yaml` is optional - missing file uses all defaults
- YAML parse errors should be returned (not silently ignored)
- Path resolution happens AFTER env overrides (env can set relative path)
- GitHub detection only runs if owner OR repo is "auto"
- Validation runs LAST, after all transformations
- The returned Config is considered immutable (no setter methods)

### Load Order

1. Start with DefaultConfig()
2. Parse `.choo.yaml` if it exists (YAML fields merge into defaults)
3. Apply environment variable overrides
4. Resolve relative paths to absolute
5. Auto-detect GitHub if needed
6. Validate final config
7. Return immutable config

## NOT In Scope

- Hot reloading of config
- Multiple config file locations
- Config file generation (`choo config init`)
- Config validation CLI command
