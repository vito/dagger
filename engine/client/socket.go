package client

import (
	"context"
	"io"
	"net"

	"github.com/moby/buildkit/session/sshforward"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type SocketProvider struct {
	EnableHostNetworkAccess bool
}

func (p SocketProvider) Register(server *grpc.Server) {
	sshforward.RegisterSSHServer(server, p)
}

func (p SocketProvider) CheckAgent(ctx context.Context, req *sshforward.CheckAgentRequest) (*sshforward.CheckAgentResponse, error) {
	if !p.EnableHostNetworkAccess {
		return nil, status.Errorf(codes.PermissionDenied, "host access is disabled")
	}
	if req.ID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "id is not set")
	}
	return &sshforward.CheckAgentResponse{}, nil
}

func (p SocketProvider) ForwardAgent(stream sshforward.SSH_ForwardAgentServer) error {
	if !p.EnableHostNetworkAccess {
		return status.Errorf(codes.PermissionDenied, "host access is disabled")
	}
	opts, ok := metadata.FromIncomingContext(stream.Context()) // if no metadata continue with empty object
	if !ok {
		return status.Errorf(codes.InvalidArgument, "no metadata")
	}
	var unixPath string
	if v, ok := opts[sshforward.KeySSHID]; ok && len(v) > 0 && v[0] != "" {
		unixPath = v[0]
	}
	if unixPath == "" {
		return status.Errorf(codes.InvalidArgument, "id is not set")
	}
	return (&socketProxy{
		dial: func() (io.ReadWriteCloser, error) {
			return net.Dial("unix", unixPath)
		},
	}).ForwardAgent(stream)
}

type socketProxy struct {
	dial func() (io.ReadWriteCloser, error)
}

var _ sshforward.SSHServer = &socketProxy{}

func (p *socketProxy) CheckAgent(ctx context.Context, req *sshforward.CheckAgentRequest) (*sshforward.CheckAgentResponse, error) {
	return &sshforward.CheckAgentResponse{}, nil
}

func (p *socketProxy) ForwardAgent(stream sshforward.SSH_ForwardAgentServer) error {
	conn, err := p.dial()
	if err != nil {
		return err
	}

	return sshforward.Copy(context.TODO(), conn, stream, nil)
}
