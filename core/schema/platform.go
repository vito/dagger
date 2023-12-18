package schema

import (
	"context"

	"github.com/dagger/dagger/core"
	"github.com/vito/dagql"
)

type platformSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &platformSchema{}

func (s *platformSchema) Name() string {
	return "platform"
}

func (s *platformSchema) Schema() string {
	return Platform
}

func (s *platformSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("defaultPlatform", s.defaultPlatform),
	}.Install(s.srv)

	s.srv.InstallScalar(core.Platform{})
}

func (s *platformSchema) defaultPlatform(ctx context.Context, parent *core.Query, _ struct{}) (core.Platform, error) {
	return parent.Platform, nil
}
