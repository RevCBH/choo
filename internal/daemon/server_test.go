package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServer_SocketCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
		Version:    "v1.0.0",
	}, nil, jm)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Wait for socket to be created
	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	// Verify socket exists
	info, err := os.Stat(socketPath)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSocket, info.Mode()&os.ModeSocket)

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Stop(ctx))
}

func TestServer_SocketPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	// Check permissions are 0600
	info, err := os.Stat(socketPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Stop(ctx)
}

func TestServer_SocketRemovesStaleFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create a stale socket file
	f, err := os.Create(socketPath)
	require.NoError(t, err)
	f.Close()

	jm := newMockJobManager()
	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		info, err := os.Stat(socketPath)
		if err != nil {
			return false
		}
		// Should be a socket now, not a regular file
		return info.Mode()&os.ModeSocket != 0
	}, time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Stop(ctx)
}

func TestServer_SocketCleanuponStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Stop(ctx))

	// Socket should be removed
	_, statErr := os.Stat(socketPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestServer_SocketGRPCConnection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
		Version:    "v1.2.3",
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	// Connect via gRPC
	conn, err := grpc.Dial(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Call Health RPC
	client := apiv1.NewDaemonServiceClient(conn)
	resp, err := client.Health(context.Background(), &apiv1.HealthRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "v1.2.3", resp.Version)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Stop(ctx)
}

func TestServer_SocketDoubleStart(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		return server.IsRunning()
	}, time.Second, 10*time.Millisecond)

	// Second start should fail
	startErr := server.Start()
	assert.Error(t, startErr)
	assert.Contains(t, startErr.Error(), "already running")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Stop(ctx)
}

func TestServer_SocketGracefulStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		return server.IsRunning()
	}, time.Second, 10*time.Millisecond)

	// Graceful stop with long timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stopErr := server.Stop(ctx)
	require.NoError(t, stopErr)

	assert.False(t, server.IsRunning())
}

func TestServer_SocketForceStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sock-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, "test.sock")
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		SocketPath: socketPath,
	}, nil, jm)

	go server.Start()

	require.Eventually(t, func() bool {
		return server.IsRunning()
	}, time.Second, 10*time.Millisecond)

	// Force stop with immediate timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to force stop

	stopErr := server.Stop(ctx)
	require.NoError(t, stopErr)
	assert.False(t, server.IsRunning())
}

func TestServer_SocketDefaultPath(t *testing.T) {
	jm := newMockJobManager()

	server := NewServer(ServerConfig{
		// SocketPath not specified
	}, nil, jm)

	assert.Equal(t, DefaultSocketPath, server.SocketPath())
}
