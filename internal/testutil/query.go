package testutil

import (
	"context"

	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/internal/sdk/dagger"
)

type QueryOptions struct {
	Variables map[string]any
	Operation string
}

func Query(query string, res any, opts *QueryOptions, optionalCfg ...*engine.Config) error {
	ctx := context.Background()

	if opts == nil {
		opts = &QueryOptions{}
	}
	if opts.Variables == nil {
		opts.Variables = make(map[string]any)
	}

	c, err := dagger.Connect(ctx, optionalCfg...)
	if err != nil {
		return err
	}
	defer c.Close()

	return c.Do(ctx,
		&dagger.Request{
			Query:     query,
			Variables: opts.Variables,
			OpName:    opts.Operation,
		},
		&dagger.Response{Data: &res},
	)
}
