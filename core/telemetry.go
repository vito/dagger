package core

import (
	"context"
	"log"

	"github.com/dagger/dagger/core/pipeline"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
	"github.com/vito/dagql/ioctx"
	"github.com/vito/progrock"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/types/known/anypb"
)

func TelemetryFunc(ctx context.Context, self dagql.Object, id *idproto.ID, next func(context.Context) (dagql.Typed, error)) func(context.Context) (dagql.Typed, error) {
	return func(ctx context.Context) (dagql.Typed, error) {
		if isIntrospection(id) {
			return next(ctx)
		}

		rec := progrock.FromContext(ctx)

		dig, resolverErr := id.Canonical().Digest()
		if resolverErr != nil {
			slog.Warn("failed to digest id", "id", id.Display(), "err", resolverErr)
			return next(ctx)
		}
		payload, resolverErr := anypb.New(id)
		if resolverErr != nil {
			slog.Warn("failed to anypb.New(id)", "id", id.Display(), "err", resolverErr)
			return next(ctx)
		}
		vtx := rec.Vertex(dig, id.Field, progrock.Internal())
		ctx = ioctx.WithStdout(ctx, vtx.Stdout())
		ctx = ioctx.WithStderr(ctx, vtx.Stderr())

		// send ID payload to the frontend
		vtx.Meta("id", payload)

		// respect user-configured pipelines
		if w, ok := self.(dagql.Wrapper); ok {
			if pl, ok := w.Unwrap().(pipeline.Pipelineable); ok {
				rec = pl.PipelinePath().RecorderGroup(rec)
			}
		}

		// group any future vertices (e.g. from Buildkit) under this one
		rec = rec.WithGroup(id.Field, progrock.WithGroupID(dig.String()))

		// call the resolver with progrock wired up
		ctx = progrock.ToContext(ctx, rec)
		res, resolverErr := next(ctx)

		if resolverErr != nil {
			log.Println("!!! ID ERRORED", id.Display(), resolverErr)
		} else {
			log.Println("!!! ID OK", id.Display())
		}

		if obj, ok := res.(dagql.Object); ok {
			objDigest, err := obj.ID().Canonical().Digest()
			if err != nil {
				slog.Error("failed to digest object", "id", id.Display(), "err", err)
			} else {
				vtx.Output(objDigest)
			}
		}

		vtx.Done(resolverErr)

		return res, resolverErr
	}
}

func isIntrospection(id *idproto.ID) bool {
	if id.Parent == nil {
		return id.Field == "__schema"
	} else {
		return isIntrospection(id.Parent)
	}
}
