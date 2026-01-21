package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestPIDFile_Acquire_AlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// First acquire
	pf1 := NewPIDFile(pidPath)
	err := pf1.Acquire()
	require.NoError(t, err)

	// Second acquire should fail (current process is still running)
	pf2 := NewPIDFile(pidPath)
	err = pf2.Acquire()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon already running")
	assert.Contains(t, err.Error(), strconv.Itoa(os.Getpid()))

	// Cleanup
	require.NoError(t, pf1.Release())
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

func TestPIDFile_Release(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	pf := NewPIDFile(pidPath)
	err := pf.Acquire()
	require.NoError(t, err)

	// Release should remove the file
	err = pf.Release()
	require.NoError(t, err)

	// File should no longer exist
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err))
}

func TestPIDFile_Release_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	pf := NewPIDFile(pidPath)
	err := pf.Release()
	require.NoError(t, err) // Should not error when file doesn't exist
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should be running
	running := IsProcessRunning(os.Getpid())
	assert.True(t, running)
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	// PID 999999 should not exist
	running := IsProcessRunning(999999)
	assert.False(t, running)
}

func TestReadPID_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	expectedPID := 12345
	err := os.WriteFile(pidPath, []byte(strconv.Itoa(expectedPID)), 0644)
	require.NoError(t, err)

	pid, err := ReadPID(pidPath)
	require.NoError(t, err)
	assert.Equal(t, expectedPID, pid)
}

func TestReadPID_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write invalid content
	err := os.WriteFile(pidPath, []byte("not-a-number"), 0644)
	require.NoError(t, err)

	pid, err := ReadPID(pidPath)
	require.Error(t, err)
	assert.Equal(t, 0, pid)
	assert.Contains(t, err.Error(), "invalid PID")
}

func TestReadPID_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	pid, err := ReadPID(pidPath)
	require.Error(t, err)
	assert.Equal(t, 0, pid)
	assert.True(t, os.IsNotExist(err))
}
