package container

import (
	"context"
	"io"
	"time"
)

// Manager provides container lifecycle management.
// Implementations must be safe for concurrent use.
type Manager interface {
	// Create creates a new container but does not start it.
	// Returns the container ID on success.
	Create(ctx context.Context, cfg ContainerConfig) (ContainerID, error)

	// Start starts a previously created container.
	Start(ctx context.Context, id ContainerID) error

	// Wait blocks until the container exits and returns the exit code.
	// Returns an error if the container doesn't exist or wait fails.
	Wait(ctx context.Context, id ContainerID) (exitCode int, err error)

	// Logs returns a stream of container logs (stdout and stderr combined).
	// The caller must close the returned ReadCloser.
	Logs(ctx context.Context, id ContainerID) (io.ReadCloser, error)

	// Stop stops a running container. Sends SIGTERM, waits for timeout,
	// then sends SIGKILL if still running.
	Stop(ctx context.Context, id ContainerID, timeout time.Duration) error

	// Remove removes a container. The container must be stopped first.
	Remove(ctx context.Context, id ContainerID) error
}
