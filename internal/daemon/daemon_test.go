package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(tmpDir string) *Config {
	return &Config{
		SocketPath: filepath.Join(tmpDir, "d.sock"),
		PIDFile:    filepath.Join(tmpDir, "d.pid"),
		DBPath:     filepath.Join(tmpDir, "db"),
		MaxJobs:    5,
	}
}

func TestDaemon_New(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		SocketPath: filepath.Join(tmpDir, "d.sock"),
		PIDFile:    filepath.Join(tmpDir, "d.pid"),
		DBPath:     filepath.Join(tmpDir, "db"),
		MaxJobs:    5,
	}

	d, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, d)

	assert.NotNil(t, d.jobManager)
	assert.NotNil(t, d.db)
}

func TestDaemon_New_InvalidConfig(t *testing.T) {
	// Test with invalid config (MaxJobs <= 0)
	cfg := &Config{
		SocketPath: "/tmp/daemon.sock",
		PIDFile:    "/tmp/daemon.pid",
		DBPath:     "/tmp/test.db",
		MaxJobs:    0, // Invalid
	}

	d, err := New(cfg)
	assert.Error(t, err)
	assert.Nil(t, d)
	assert.Contains(t, err.Error(), "invalid config")
}

func TestDaemon_Start_CreatesPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	d, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine since it blocks
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Verify PID file exists
	_, err = os.Stat(cfg.PIDFile)
	assert.NoError(t, err)

	// Trigger shutdown
	cancel()
	d.Shutdown()

	// Wait for clean exit
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}
}

func TestDaemon_Start_AlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	// Start first daemon
	d1, err := New(cfg)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- d1.Start(ctx1)
	}()

	// Wait for first daemon to start
	time.Sleep(100 * time.Millisecond)

	// Try to start second daemon with same config
	d2, err := New(cfg)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	err = d2.Start(ctx2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Cleanup first daemon
	cancel1()
	d1.Shutdown()
	<-errCh1
}

func TestDaemon_Start_CreatesSocket(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	d, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Verify socket exists
	_, err = os.Stat(cfg.SocketPath)
	assert.NoError(t, err)

	// Verify it's a socket
	info, err := os.Stat(cfg.SocketPath)
	require.NoError(t, err)
	assert.NotEqual(t, 0, info.Mode()&os.ModeSocket)

	// Trigger shutdown
	cancel()
	d.Shutdown()

	// Wait for clean exit
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}
}

func TestDaemon_Start_ResumesJobs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	// Create a daemon and start it to initialize the database
	d1, err := New(cfg)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d1.Start(ctx1)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Shutdown the daemon
	cancel1()
	d1.Shutdown()

	// Wait for shutdown
	select {
	case <-errCh:
		// Daemon stopped
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first daemon to stop")
	}

	// Create a new daemon with the same config
	// This should trigger resume logic on startup
	d2, err := New(cfg)
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- d2.Start(ctx2)
	}()

	// Wait for startup - if it doesn't hang, resume logic worked
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel2()
	d2.Shutdown()

	select {
	case <-errCh2:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for second daemon to stop")
	}
}

func TestDaemon_Shutdown_GracefulStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	d, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	d.Shutdown()

	// Wait for shutdown
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for graceful shutdown")
	}
}

func TestDaemon_Shutdown_CleanupFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	d, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Files should exist
	assert.FileExists(t, cfg.PIDFile)
	assert.FileExists(t, cfg.SocketPath)

	cancel()
	d.Shutdown()

	// Wait for shutdown to complete
	select {
	case <-errCh:
		// Shutdown complete
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}

	// Give a moment for cleanup
	time.Sleep(100 * time.Millisecond)

	// Files should be cleaned up
	assert.NoFileExists(t, cfg.PIDFile)
	assert.NoFileExists(t, cfg.SocketPath)
}

func TestDaemon_Shutdown_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir)

	d, err := New(cfg)
	require.NoError(t, err)

	// Start a long-running job by directly using the job manager
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	jobCfg := JobConfig{
		RepoPath:      tmpDir, // Use tmpDir as a placeholder
		TasksDir:      tmpDir,
		TargetBranch:  "main",
		FeatureBranch: "test",
		Concurrency:   1,
	}

	// Note: This test verifies timeout behavior exists, but we can't easily
	// create a job that takes longer than 30s to stop in a unit test.
	// The timeout logic is tested by observing it doesn't hang forever.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Start a job (it will fail to actually run due to invalid paths, but will be tracked)
	_, _ = d.jobManager.Start(jobCtx, jobCancel, jobCfg)

	// Trigger shutdown
	cancel()
	d.Shutdown()

	// The shutdown should complete within timeout (30s + buffer)
	// If it hangs, the test will timeout
	select {
	case <-errCh:
		// Success - shutdown completed
	case <-time.After(35 * time.Second):
		t.Fatal("shutdown did not complete within expected timeout")
	}
}
