package core

import (
	"context"
	"io"
	"net"

	"github.com/moby/buildkit/session/sshforward"
)

type Socket struct {
	// Unix
	HostPath string `json:"host_path,omitempty"`

	// IP
	HostProtocol NetworkProtocol `json:"host_protocol,omitempty"`
	HostAddr     string          `json:"host_addr,omitempty"`
}

type SocketID string

func (id SocketID) String() string { return string(id) }

func (id SocketID) ToSocket() (*Socket, error) {
	var socket Socket
	if err := decodeID(&socket, id); err != nil {
		return nil, err
	}

	return &socket, nil
}

func NewHostUnixSocket(absPath, clientHostname string) *Socket {
	return &Socket{
		HostPath: absPath,
	}
}

func NewHostIPSocket(proto NetworkProtocol, addr string) *Socket {
	return &Socket{
		HostAddr:     addr,
		HostProtocol: proto,
	}
}

func (socket *Socket) ID() (SocketID, error) {
	return encodeID[SocketID](socket)
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
	if socket.HostPath != "" {
		return "unix"
	} else {
		return socket.HostProtocol.Network()
	}
}

func (socket *Socket) Addr() string {
	if socket.HostPath != "" {
		return socket.HostPath
	} else {
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
