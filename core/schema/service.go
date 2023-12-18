package schema

import (
	"context"
	"runtime/debug"

	"github.com/dagger/dagger/core"
	"github.com/vito/dagql"
)

type serviceSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &serviceSchema{}

func (s *serviceSchema) Name() string {
	return "service"
}

func (s *serviceSchema) Schema() string {
	return Service
}

func (s *serviceSchema) Install() {
	dagql.Fields[*core.Container]{
		dagql.Func("asService", s.containerAsService),
	}.Install(s.srv)

	dagql.Fields[*core.Service]{
		dagql.NodeFunc("hostname", s.hostname),
		dagql.NodeFunc("ports", s.ports),
		dagql.NodeFunc("endpoint", s.endpoint),
		dagql.NodeFunc("start", s.start).Impure(),
		dagql.NodeFunc("stop", s.stop).Impure(),
	}.Install(s.srv)
}

func (s *serviceSchema) containerAsService(ctx context.Context, parent *core.Container, args struct{}) (*core.Service, error) {
	return parent.Service(ctx)
}

func (s *serviceSchema) hostname(ctx context.Context, parent dagql.Instance[*core.Service], args struct{}) (dagql.String, error) {
	hn, err := parent.Self.Hostname(ctx, parent.ID())
	if err != nil {
		return "", err
	}
	return dagql.NewString(hn), nil
}

func (s *serviceSchema) ports(ctx context.Context, parent dagql.Instance[*core.Service], args struct{}) (dagql.Array[core.Port], error) {
	return parent.Self.Ports(ctx, parent.ID())
}

type serviceEndpointArgs struct {
	Port   dagql.Optional[dagql.Int]
	Scheme string `default:""`
}

func (s *serviceSchema) endpoint(ctx context.Context, parent dagql.Instance[*core.Service], args serviceEndpointArgs) (dagql.String, error) {
	str, err := parent.Self.Endpoint(ctx, parent.ID(), args.Port.Value.Int(), args.Scheme)
	if err != nil {
		return "", err
	}
	return dagql.NewString(str), nil
}

func (s *serviceSchema) start(ctx context.Context, parent dagql.Instance[*core.Service], args struct{}) (core.ServiceID, error) {
	defer func() {
		if err := recover(); err != nil {
			debug.PrintStack()
			panic(err)
		}
	}()

	if err := parent.Self.StartAndTrack(ctx, parent.ID()); err != nil {
		return core.ServiceID{}, err
	}

	return dagql.NewID[*core.Service](parent.ID()), nil
}

func (s *serviceSchema) stop(ctx context.Context, parent dagql.Instance[*core.Service], args struct{}) (core.ServiceID, error) {
	if err := parent.Self.Stop(ctx, parent.ID()); err != nil {
		return core.ServiceID{}, err
	}

	err := parent.Self.Stop(ctx, parent.ID())
	if err != nil {
		return core.ServiceID{}, err
	}

	return dagql.NewID[*core.Service](parent.ID()), nil
}
