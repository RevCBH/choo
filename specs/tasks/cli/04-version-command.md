---
task: 4
status: pending
backpressure: "go test ./internal/cli/... -run Version"
depends_on: [1]
---

# Version Command

**Parent spec**: `/specs/CLI.md`
**Task**: #4 of 9 in implementation plan

## Objective

Implement the version command that displays version, commit, and build date information.

## Dependencies

### External Specs (must be implemented)
- None

### Task Dependencies (within this unit)
- Task #1 must be complete (provides: `App` struct, `rootCmd`)

### Package Dependencies
- None beyond standard library

## Deliverables

### Files to Create/Modify

```
internal/
└── cli/
    └── version.go    # CREATE: Version command implementation
```

### Types to Implement

```go
// VersionInfo holds version metadata
type VersionInfo struct {
    Version string
    Commit  string
    Date    string
}
```

### Functions to Implement

```go
// NewVersionCmd creates the version command
func NewVersionCmd(app *App) *cobra.Command {
    // Create cobra.Command with Use: "version"
    // Print version info in RunE
    // Return command
}
```

### Modifications to cli.go

```go
// Add to App struct
type App struct {
    // ... existing fields ...
    versionInfo VersionInfo
}

// Update SetVersion implementation
func (a *App) SetVersion(version, commit, date string) {
    a.versionInfo = VersionInfo{
        Version: version,
        Commit:  commit,
        Date:    date,
    }
}

// Add version command in setupRootCmd or New
a.rootCmd.AddCommand(NewVersionCmd(a))
```

## Backpressure

### Validation Command

```bash
go test ./internal/cli/... -run Version -v
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestVersionCmd_Output` | Output contains version, commit, and date |
| `TestVersionCmd_Format` | Output format matches expected pattern |
| `TestSetVersion` | SetVersion correctly stores version info |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Version output format:
  ```
  choo version X.Y.Z
  commit: abc1234
  built: 2024-01-15T10:30:00Z
  ```
- If version info is not set, show "dev" for version and "unknown" for commit/date
- This command should be instant (no lazy initialization needed)
- The version command is often run as `choo version` or `choo --version`

### Default Values

When `SetVersion` has not been called:
- Version: "dev"
- Commit: "unknown"
- Date: "unknown"

## NOT In Scope

- Automatic version detection from git
- Update checking
- JSON output format (future enhancement)
