package schema

import (
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/resourceid"
)

type socketSchema struct {
	*MergedSchemas

	host *core.Host
}

var _ ExecutableSchema = &socketSchema{}

func (s *socketSchema) Name() string {
	return "socket"
}

func (s *socketSchema) Schema() string {
	return Socket
}

var socketIDResolver = idResolver[core.SocketID, core.Socket]()

func (s *socketSchema) Resolvers() Resolvers {
	return Resolvers{
		"SocketID": socketIDResolver,
		"Query": ObjectResolver{
			"socket": ToResolver(s.socket),
		},
		"Socket": ToIDableObjectResolver(loader[core.Socket](s.queryCache), ObjectResolver{
			"id": ToResolver(s.id),
		}),
	}
}

func (s *socketSchema) Dependencies() []ExecutableSchema {
	return nil
}

func (s *socketSchema) id(ctx *core.Context, parent *core.Socket, args any) (core.SocketID, error) {
	return resourceid.FromProto[core.Socket](parent.ID), nil
}

type socketArgs struct {
	ID core.SocketID
}

// nolint: unparam
func (s *socketSchema) socket(_ *core.Context, _ any, args socketArgs) (*core.Socket, error) {
	return args.ID.Decode()
}
