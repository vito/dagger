package socket

import (
	"context"
	"io"
	"net"

	"github.com/dagger/dagger/core/idproto"
	"github.com/dagger/dagger/core/resourceid"
	"github.com/moby/buildkit/session/sshforward"
)

type ID = resourceid.ID[Socket]

type Socket struct {
	ID *idproto.ID `json:"id,omitempty"`

	HostPath string `json:"host_path,omitempty"`
}

func NewHostSocket(absPath string) *Socket {
	return &Socket{
		HostPath: absPath,
	}
}

func (socket *Socket) IsHost() bool {
	return socket.HostPath != ""
}

func (socket *Socket) Server() (sshforward.SSHServer, error) {
	return &socketProxy{
		dial: func() (io.ReadWriteCloser, error) {
			return net.Dial("unix", socket.HostPath)
		},
	}, nil
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
