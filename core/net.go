package core

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

// Port configures a port to exposed from a container or service.
type Port struct {
	Port     int             `field:"true" doc:"The port number."`
	Protocol NetworkProtocol `field:"true" doc:"The transport layer protocol."`

	// TODO: use a more specific method name like GraphQLDescription?
	Description_ *string `field:"true" name:"description" doc:"The port description."`
}

func (Port) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Port",
		NonNull:   true,
	}
}

func (Port) Description() string {
	return "A port exposed by a container."
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

func (proto NetworkProtocol) Description() string {
	return "Transport layer network protocol associated to a port."
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
	Frontend *int            `doc:"Port to expose to clients. If unspecified, a default will be chosen."`
	Backend  int             `doc:"Destination port for traffic."`
	Protocol NetworkProtocol `doc:"Transport layer protocol to use for traffic." default:"TCP"`
}

func (pf PortForward) TypeName() string {
	return "PortForward"
}

func (pf PortForward) Description() string {
	return "Port forwarding rules for tunneling network traffic."
}

func (pf PortForward) FrontendOrBackendPort() int {
	if pf.Frontend != nil {
		return *pf.Frontend
	}
	return pf.Backend
}
