package client

import (
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
