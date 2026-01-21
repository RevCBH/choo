package client

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
)

// Client wraps gRPC connection and service stub for daemon communication
type Client struct {
	conn   *grpc.ClientConn
	daemon apiv1.DaemonServiceClient
}

// New creates a client connected to the daemon Unix socket.
// The socketPath should be the full path to the daemon socket
// (typically ~/.choo/daemon.sock).
//
// The connection uses insecure credentials since Unix sockets
// are protected by filesystem permissions.
func New(socketPath string) (*Client, error) {
	conn, err := grpc.Dial(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		daemon: apiv1.NewDaemonServiceClient(conn),
	}, nil
}

// Close releases the underlying gRPC connection.
// It is safe to call Close multiple times.
func (c *Client) Close() error {
	return c.conn.Close()
}

// StartJob initiates a new orchestration job with the given configuration.
// Returns the job ID on success.
func (c *Client) StartJob(ctx context.Context, cfg JobConfig) (string, error) {
	req := jobConfigToProto(cfg)
	resp, err := c.daemon.StartJob(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetJobId(), nil
}

// StopJob cancels a running job.
// If force is true, the job terminates immediately without cleanup.
// If force is false, the job completes current tasks before stopping.
func (c *Client) StopJob(ctx context.Context, jobID string, force bool) error {
	req := &apiv1.StopJobRequest{
		JobId: jobID,
		Force: force,
	}
	_, err := c.daemon.StopJob(ctx, req)
	return err
}

// ListJobs returns job summaries, optionally filtered by status.
// Pass an empty slice for statusFilter to list all jobs.
func (c *Client) ListJobs(ctx context.Context, statusFilter []string) ([]*JobSummary, error) {
	req := &apiv1.ListJobsRequest{
		StatusFilter: statusFilter,
	}
	resp, err := c.daemon.ListJobs(ctx, req)
	if err != nil {
		return nil, err
	}
	return protoToJobSummaries(resp.GetJobs()), nil
}

// GetJobStatus returns detailed status for a specific job.
// Returns an error if the job ID does not exist.
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	req := &apiv1.GetJobStatusRequest{
		JobId: jobID,
	}
	resp, err := c.daemon.GetJobStatus(ctx, req)
	if err != nil {
		return nil, err
	}
	return protoToJobStatus(resp), nil
}

// Health checks daemon health and returns version info.
// This is a lightweight call suitable for polling.
func (c *Client) Health(ctx context.Context) (*HealthInfo, error) {
	req := &apiv1.HealthRequest{}
	resp, err := c.daemon.Health(ctx, req)
	if err != nil {
		return nil, err
	}
	return protoToHealthInfo(resp), nil
}

// Shutdown requests daemon termination.
// If waitForJobs is true, the daemon waits for active jobs to complete
// before shutting down, up to the specified timeout in seconds.
// If waitForJobs is false, active jobs are cancelled immediately.
func (c *Client) Shutdown(ctx context.Context, waitForJobs bool, timeout int) error {
	req := &apiv1.ShutdownRequest{
		WaitForJobs:    waitForJobs,
		TimeoutSeconds: int32(timeout),
	}
	_, err := c.daemon.Shutdown(ctx, req)
	return err
}
