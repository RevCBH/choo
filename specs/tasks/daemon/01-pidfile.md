# Task 01: PID File Management

```yaml
task: 01-pidfile
unit: daemon
depends_on: []
backpressure: "go test ./internal/daemon/... -run TestPIDFile -v"
```

## Objective

Implement PID file management for single-instance daemon enforcement.

## Requirements

1. Create `internal/daemon/pidfile.go` with:

   ```go
   type PIDFile struct {
       path string
       file *os.File
   }

   // Acquire tries to create/lock the PID file
   // Returns error if another daemon is running
   func Acquire(path string) (*PIDFile, error)

   // Release removes the PID file and releases the lock
   func (p *PIDFile) Release() error

   // ReadPID reads the PID from an existing file (for client use)
   func ReadPID(path string) (int, error)

   // IsRunning checks if a process with the given PID is alive
   func IsRunning(pid int) bool
   ```

2. PID file location: `~/.choo/daemon.pid`

3. Locking behavior:
   - Use `syscall.Flock` for advisory locking on Unix
   - Write PID to file after acquiring lock
   - `Acquire` fails if lock cannot be obtained (daemon already running)

4. Cleanup:
   - `Release` removes the file and releases lock
   - Stale PID files (process not running) should be cleaned up on `Acquire`

## Acceptance Criteria

- [ ] Only one daemon can hold the PID file at a time
- [ ] Stale PID files are cleaned up automatically
- [ ] `ReadPID` works for client connection
- [ ] `IsRunning` correctly detects live processes
- [ ] Works on macOS and Linux

## Files to Create/Modify

- `internal/daemon/pidfile.go` (create)
- `internal/daemon/pidfile_test.go` (create)
