package core

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

// Port configures a port to exposed from a container or service.
type Port struct {
	Port        int             `field:"true"`
	Protocol    NetworkProtocol `field:"true"`
	Description *string         `field:"true"`
}

func (Port) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Port",
		NonNull:   true,
	}
}

// NetworkProtocol is a GraphQL enum type.
type NetworkProtocol string

var NetworkProtocols = dagql.NewEnum[NetworkProtocol]()

var (
	NetworkProtocolTCP = NetworkProtocols.Register("TCP")
	NetworkProtocolUDP = NetworkProtocols.Register("UDP")
)

func (proto NetworkProtocol) Type() *ast.Type {
	return &ast.Type{
		NamedType: "NetworkProtocol",
		NonNull:   true,
	}
}

func (proto NetworkProtocol) Decoder() dagql.InputDecoder {
	return NetworkProtocols
}

func (proto NetworkProtocol) ToLiteral() *idproto.Literal {
	return NetworkProtocols.Literal(proto)
}

// Network returns the value appropriate for the "network" argument to Go
// net.Dial, and for appending to the port number to form the key for the
// ExposedPorts object in the OCI image config.
func (proto NetworkProtocol) Network() string {
	return strings.ToLower(string(proto))
}

type PortForward struct {
	Frontend int `default:"0"` // TODO
	Backend  int
	Protocol NetworkProtocol `default:"TCP"`
}

func (pf PortForward) TypeName() string {
	return "PortForward"
}

func (pf PortForward) FrontendOrBackendPort() int {
	if pf.Frontend != 0 {
		return pf.Frontend
	}
	return pf.Backend
}
