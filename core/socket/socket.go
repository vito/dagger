package socket

import (
	"context"
	"io"
	"net"

	"github.com/dagger/dagger/core/resourceid"
	"github.com/moby/buildkit/session/sshforward"
)

type Socket struct {
	// Client which is providing the socket.
	ClientID string `json:"client_id,omitempty"`

	// Unix
	HostPath string `json:"host_path,omitempty"`

	// IP
	HostProtocol string `json:"host_protocol,omitempty"`
	HostAddr     string `json:"host_addr,omitempty"`
}

type ID string

func (id ID) String() string { return string(id) }

func (id ID) ToSocket() (*Socket, error) {
	var socket Socket
	if err := resourceid.Decode(&socket, id); err != nil {
		return nil, err
	}

	return &socket, nil
}

func NewHostUnixSocket(clientID, absPath string) *Socket {
	return &Socket{
		ClientID: clientID,

		HostPath: absPath,
	}
}

func (socket *Socket) ID() (ID, error) {
	return resourceid.Encode[ID](socket)
}

func NewHostIPSocket(clientID, proto string, addr string) *Socket {
	return &Socket{
		ClientID: clientID,

		HostAddr:     addr,
		HostProtocol: proto,
	}
}

func (socket *Socket) IsHost() bool {
	return socket.HostPath != "" || socket.HostAddr != ""
}

func (socket *Socket) Server() (sshforward.SSHServer, error) {
	// TODO udp
	return &socketProxy{
		dial: func() (io.ReadWriteCloser, error) {
			return net.Dial(socket.Network(), socket.Addr())
		},
	}, nil
}

func (socket *Socket) Network() string {
	switch {
	case socket.HostPath != "":
		return "unix"
	default:
		return socket.HostProtocol
	}
}

func (socket *Socket) Addr() string {
	switch {
	case socket.HostPath != "":
		return socket.HostPath
	default:
		return socket.HostAddr
	}
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
