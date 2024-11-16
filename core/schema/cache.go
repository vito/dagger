package schema

import (
	"context"
	"errors"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/dagql"
)

type cacheSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &cacheSchema{}

func (s *cacheSchema) Name() string {
	return "cache"
}

func (s *cacheSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.NodeFunc("cacheVolume", s.cacheVolume).
			Doc("Constructs a cache volume for a given cache key.").
			ArgDoc("key", `A string identifier to target this cache volume (e.g., "modules-cache").`).
			ArgDoc("namespace", `An additional key component to namespace the cache volume (e.g., "my-module").`),
	}.Install(s.srv)

	dagql.Fields[*core.CacheVolume]{}.Install(s.srv)
}

func (s *cacheSchema) Dependencies() []SchemaResolvers {
	return nil
}

type cacheArgs struct {
	Key       string
	Namespace string `default:""`
}

func (s *cacheSchema) cacheVolume(ctx context.Context, parent dagql.Instance[*core.Query], args cacheArgs) (dagql.Instance[*core.CacheVolume], error) {
	var inst dagql.Instance[*core.CacheVolume]

	if args.Namespace == "" {
		m, err := parent.Self.Server.CurrentModule(ctx)
		if err != nil && !errors.Is(err, core.ErrNoCurrentModule) {
			return inst, err
		}

		if m != nil {
			// we're redirecting to a pure value based on the calling module, so
			// don't cache this call
			dagql.Taint(ctx)

			err = s.srv.Select(ctx, parent, &inst, dagql.Selector{
				Field: "cacheVolume",
				Args: []dagql.NamedInput{
					{
						Name:  "key",
						Value: dagql.NewString(args.Key),
					},
					{
						Name:  "namespace",
						Value: dagql.NewString(m.Source.ID().Digest().String()),
					},
				},
			})
			return inst, err
		}
	}

	key := args.Key
	if args.Namespace != "" {
		key = args.Namespace + ":" + key
	}
	cache := core.NewCache(key)
	return dagql.NewInstanceForCurrentID(ctx, s.srv, parent, cache)
}
