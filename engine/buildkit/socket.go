package buildkit

import (
	"context"

	"github.com/dagger/dagger/core/socket"
	"github.com/moby/buildkit/session/sshforward"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type socketProxy struct {
	c *Client
}

func (p *socketProxy) Register(srv *grpc.Server) {
	sshforward.RegisterSSHServer(srv, p)
}

func (p *socketProxy) CheckAgent(ctx context.Context, req *sshforward.CheckAgentRequest) (*sshforward.CheckAgentResponse, error) {
	// NOTE: we currently just fail only at the ForwardAgent call since that's the only time it's currently possible
	// to get the client ID. Not as ideal, but can be improved w/ work to support socket sharing across nested clients.
	return &sshforward.CheckAgentResponse{}, nil
}

func (p *socketProxy) ForwardAgent(stream sshforward.SSH_ForwardAgentServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	incomingMD, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "no metadata")
	}

	ctx = metadata.NewOutgoingContext(ctx, incomingMD)
	var id string
	if v, ok := incomingMD[sshforward.KeySSHID]; ok && len(v) > 0 && v[0] != "" {
		id = v[0]
	}
	if id == "" {
		return status.Errorf(codes.InvalidArgument, "id is not set")
	}

	socket, err := socket.ID(id).ToSocket()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	conn, err := p.c.SessionManager.Get(ctx, socket.ClientID, true)
	if err != nil {
		return err
	}

	forwardAgentClient, err := sshforward.NewSSHClient(conn.Conn()).ForwardAgent(ctx)
	if err != nil {
		return err
	}

	return proxyStream[sshforward.BytesMessage](ctx, forwardAgentClient, stream)
}
