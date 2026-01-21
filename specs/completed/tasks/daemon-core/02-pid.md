---
task: 2
status: complete
backpressure: "go test ./internal/daemon/... -run TestPID"
depends_on: []
---

# PID File Utilities

**Parent spec**: `specs/DAEMON-CORE.md`
**Task**: #2 of 7 in implementation plan

## Objective

Implement PID file management for single-instance enforcement, including stale PID detection and cleanup.

## Dependencies

### Task Dependencies (within this unit)
- None (can be implemented in parallel with Task #1)

### Package Dependencies
- `os` - for file and process operations
- `strconv` - for PID parsing
- `syscall` - for signal checking
- `strings` - for whitespace trimming

## Deliverables

### Files to Create/Modify

```
internal/daemon/
└── pid.go    # CREATE: PID file utilities
```

### Types to Implement

```go
// PIDFile manages the daemon's PID file for single-instance enforcement.
type PIDFile struct {
    path string
}
```

### Functions to Implement

```go
// NewPIDFile creates a PIDFile manager for the given path.
func NewPIDFile(path string) *PIDFile

// Acquire writes the current process PID to the file.
// Returns an error if another daemon is already running.
func (p *PIDFile) Acquire() error {
    // 1. Check if PID file exists
    // 2. If exists, read PID and check if process is running
    // 3. If process running, return error with PID
    // 4. If stale (process not running), remove file
    // 5. Write current PID to file
}

// Release removes the PID file.
// Safe to call multiple times.
func (p *PIDFile) Release() error {
    // Remove the PID file
    // Ignore "not exist" errors
}

// IsProcessRunning checks if a process with the given PID exists.
func IsProcessRunning(pid int) bool {
    // Use os.FindProcess and signal 0 check
    // Signal 0 doesn't send anything but checks if process exists
}

// ReadPID reads the PID from a file, returning 0 if file doesn't exist or is invalid.
func ReadPID(path string) (int, error) {
    // Read file contents
    // Parse as integer
    // Return 0 for empty/invalid
}
```

## Backpressure

### Validation Command

```bash
go test ./internal/daemon/... -run TestPID
```

### Must Pass

| Test | Assertion |
|------|-----------|
| `TestPIDFile_Acquire` | Creates PID file with current process PID |
| `TestPIDFile_Acquire_AlreadyRunning` | Returns error when daemon already running |
| `TestPIDFile_Acquire_StalePID` | Cleans up stale PID file and acquires |
| `TestPIDFile_Release` | Removes PID file |
| `TestPIDFile_Release_NotExists` | Does not error when file doesn't exist |
| `TestIsProcessRunning_CurrentProcess` | Returns true for `os.Getpid()` |
| `TestIsProcessRunning_InvalidPID` | Returns false for PID 999999 |
| `TestReadPID_ValidFile` | Returns correct PID |
| `TestReadPID_InvalidContent` | Returns 0 and error |
| `TestReadPID_NotExists` | Returns 0 and appropriate error |

### Test Fixtures

```go
func TestPIDFile_Acquire(t *testing.T) {
    tmpDir := t.TempDir()
    pidPath := filepath.Join(tmpDir, "test.pid")

    pf := NewPIDFile(pidPath)
    err := pf.Acquire()
    require.NoError(t, err)

    // Verify file contains current PID
    content, err := os.ReadFile(pidPath)
    require.NoError(t, err)
    pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
    require.NoError(t, err)
    assert.Equal(t, os.Getpid(), pid)

    // Cleanup
    require.NoError(t, pf.Release())
}

func TestPIDFile_Acquire_StalePID(t *testing.T) {
    tmpDir := t.TempDir()
    pidPath := filepath.Join(tmpDir, "test.pid")

    // Write a fake PID that doesn't exist
    err := os.WriteFile(pidPath, []byte("999999"), 0644)
    require.NoError(t, err)

    pf := NewPIDFile(pidPath)
    err = pf.Acquire()
    require.NoError(t, err) // Should succeed, stale PID cleaned up

    require.NoError(t, pf.Release())
}
```

### CI Compatibility

- [x] No external API keys required
- [x] No network access required
- [x] Runs in <60 seconds

## Implementation Notes

- Use `syscall.Signal(0)` to check if process exists without sending a signal
- File permissions for PID file should be 0644 (readable by others for debugging)
- Handle race conditions: another process could acquire between check and write
- Error message should include the conflicting PID for debugging

## NOT In Scope

- Configuration loading (Task #1)
- Daemon lifecycle integration (Task #7)
- Signal handling for graceful shutdown (Task #7)
