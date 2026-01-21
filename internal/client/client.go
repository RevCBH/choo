package client

import (
	"google.golang.org/grpc"

	apiv1 "github.com/RevCBH/choo/pkg/api/v1"
)

// Client wraps gRPC connection and service stub for daemon communication
type Client struct {
	conn   *grpc.ClientConn
	daemon apiv1.DaemonServiceClient
}
