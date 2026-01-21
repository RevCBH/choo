package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile manages the daemon's PID file for single-instance enforcement.
type PIDFile struct {
	path string
}

// NewPIDFile creates a PIDFile manager for the given path.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Acquire writes the current process PID to the file.
// Returns an error if another daemon is already running.
func (p *PIDFile) Acquire() error {
	// 1. Check if PID file exists
	if _, err := os.Stat(p.path); err == nil {
		// 2. If exists, read PID and check if process is running
		existingPID, err := ReadPID(p.path)
		if err != nil {
			return fmt.Errorf("failed to read existing PID file: %w", err)
		}

		// 3. If process running, return error with PID
		if existingPID > 0 && IsProcessRunning(existingPID) {
			return fmt.Errorf("daemon already running with PID %d", existingPID)
		}

		// 4. If stale (process not running), remove file
		if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale PID file: %w", err)
		}
	}

	// 5. Write current PID to file
	currentPID := os.Getpid()
	pidStr := fmt.Sprintf("%d", currentPID)
	if err := os.WriteFile(p.path, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Release removes the PID file.
// Safe to call multiple times.
func (p *PIDFile) Release() error {
	err := os.Remove(p.path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsProcessRunning checks if a process with the given PID exists.
func IsProcessRunning(pid int) bool {
	// Use syscall.Kill with signal 0
	// Signal 0 doesn't send anything but checks if process exists
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}

// ReadPID reads the PID from a file, returning 0 if file doesn't exist or is invalid.
func ReadPID(path string) (int, error) {
	// Read file contents
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, err
		}
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse as integer
	pidStr := strings.TrimSpace(string(content))
	if pidStr == "" {
		return 0, fmt.Errorf("PID file is empty")
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}
