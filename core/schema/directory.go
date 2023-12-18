package schema

import (
	"context"
	"io/fs"

	"github.com/vito/dagql"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/pipeline"
)

type directorySchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &directorySchema{}

func (s *directorySchema) Name() string {
	return "directory"
}

func (s *directorySchema) Schema() string {
	return Directory
}

func (s *directorySchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("directory", s.directory),
	}.Install(s.srv)

	dagql.Fields[*core.Directory]{
		Syncer[*core.Directory](),
		dagql.Func("pipeline", s.pipeline),
		dagql.Func("entries", s.entries),
		dagql.Func("glob", s.glob),
		dagql.Func("file", s.file),
		dagql.Func("withFile", s.withFile),
		dagql.Func("withNewFile", s.withNewFile),
		dagql.Func("withoutFile", s.withoutFile),
		dagql.Func("directory", s.subdirectory),
		dagql.Func("withDirectory", s.withDirectory),
		dagql.Func("withTimestamps", s.withTimestamps),
		dagql.Func("withNewDirectory", s.withNewDirectory),
		dagql.Func("withoutDirectory", s.withoutDirectory),
		dagql.Func("diff", s.diff),
		dagql.Func("export", s.export).Impure(),
		dagql.Func("dockerBuild", s.dockerBuild),
	}.Install(s.srv)
}

type directoryPipelineArgs struct {
	Name        string
	Description string                              `default:""`
	Labels      []dagql.InputObject[pipeline.Label] `default:"[]"`
}

func (s *directorySchema) pipeline(ctx context.Context, parent *core.Directory, args directoryPipelineArgs) (*core.Directory, error) {
	return parent.WithPipeline(ctx, args.Name, args.Description, collectInputsSlice(args.Labels))
}

type directoryArgs struct {
	ID dagql.Optional[core.DirectoryID]
}

func (s *directorySchema) directory(ctx context.Context, parent *core.Query, args directoryArgs) (*core.Directory, error) {
	if args.ID.Valid {
		inst, err := args.ID.Value.Load(ctx, s.srv)
		if err != nil {
			return nil, err
		}
		return inst.Self, nil
	}
	platform := parent.Platform
	return core.NewScratchDirectory(parent, parent.PipelinePath(), platform), nil
}

type subdirectoryArgs struct {
	Path string
}

func (s *directorySchema) subdirectory(ctx context.Context, parent *core.Directory, args subdirectoryArgs) (*core.Directory, error) {
	return parent.Directory(ctx, args.Path)
}

type withNewDirectoryArgs struct {
	Path        string
	Permissions int `default:"0644"` // FIXME(vito): verify this parses as expected, prob doesn't
}

func (s *directorySchema) withNewDirectory(ctx context.Context, parent *core.Directory, args withNewDirectoryArgs) (*core.Directory, error) {
	return parent.WithNewDirectory(ctx, args.Path, fs.FileMode(args.Permissions))
}

type WithDirectoryArgs struct {
	Path      string
	Directory core.DirectoryID

	core.CopyFilter
}

func (s *directorySchema) withDirectory(ctx context.Context, parent *core.Directory, args WithDirectoryArgs) (*core.Directory, error) {
	dir, err := args.Directory.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.WithDirectory(ctx, args.Path, dir.Self, args.CopyFilter, nil)
}

type dirWithTimestampsArgs struct {
	Timestamp int
}

func (s *directorySchema) withTimestamps(ctx context.Context, parent *core.Directory, args dirWithTimestampsArgs) (*core.Directory, error) {
	return parent.WithTimestamps(ctx, args.Timestamp)
}

type entriesArgs struct {
	Path dagql.Optional[dagql.String]
}

func (s *directorySchema) entries(ctx context.Context, parent *core.Directory, args entriesArgs) (dagql.Array[dagql.String], error) {
	ents, err := parent.Entries(ctx, args.Path.Value.String())
	if err != nil {
		return nil, err
	}
	return dagql.NewStringArray(ents...), nil
}

type globArgs struct {
	Pattern string
}

func (s *directorySchema) glob(ctx context.Context, parent *core.Directory, args globArgs) (dagql.Array[dagql.String], error) {
	ents, err := parent.Glob(ctx, ".", args.Pattern)
	if err != nil {
		return nil, err
	}
	return dagql.NewStringArray(ents...), nil
}

type dirFileArgs struct {
	Path string
}

func (s *directorySchema) file(ctx context.Context, parent *core.Directory, args dirFileArgs) (*core.File, error) {
	return parent.File(ctx, args.Path)
}

func (s *directorySchema) withNewFile(ctx context.Context, parent *core.Directory, args struct {
	Path        string
	Contents    string
	Permissions int `default:"0644"`
}) (*core.Directory, error) {
	return parent.WithNewFile(ctx, args.Path, []byte(args.Contents), fs.FileMode(args.Permissions), nil)
}

type WithFileArgs struct {
	Path        string
	Source      core.FileID
	Permissions *int
}

func (s *directorySchema) withFile(ctx context.Context, parent *core.Directory, args WithFileArgs) (*core.Directory, error) {
	file, err := args.Source.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}

	return parent.WithFile(ctx, args.Path, file.Self, args.Permissions, nil)
}

type withoutDirectoryArgs struct {
	Path string
}

func (s *directorySchema) withoutDirectory(ctx context.Context, parent *core.Directory, args withoutDirectoryArgs) (*core.Directory, error) {
	return parent.Without(ctx, args.Path)
}

type withoutFileArgs struct {
	Path string
}

func (s *directorySchema) withoutFile(ctx context.Context, parent *core.Directory, args withoutFileArgs) (*core.Directory, error) {
	return parent.Without(ctx, args.Path)
}

type diffArgs struct {
	Other core.DirectoryID
}

func (s *directorySchema) diff(ctx context.Context, parent *core.Directory, args diffArgs) (*core.Directory, error) {
	dir, err := args.Other.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}
	return parent.Diff(ctx, dir.Self)
}

type dirExportArgs struct {
	Path string
}

func (s *directorySchema) export(ctx context.Context, parent *core.Directory, args dirExportArgs) (dagql.Boolean, error) {
	err := parent.Export(ctx, args.Path)
	if err != nil {
		return false, err
	}

	return true, nil
}

type dirDockerBuildArgs struct {
	Platform   dagql.Optional[core.Platform]
	Dockerfile string                             `default:"Dockerfile"`
	Target     string                             `default:""`
	BuildArgs  []dagql.InputObject[core.BuildArg] `default:"[]"`
	Secrets    []core.SecretID                    `default:"[]"`
}

func (s *directorySchema) dockerBuild(ctx context.Context, parent *core.Directory, args dirDockerBuildArgs) (*core.Container, error) {
	platform := parent.Query.Platform
	if args.Platform.Valid {
		platform = args.Platform.Value
	}
	ctr, err := core.NewContainer(parent.Query, parent.Pipeline, platform)
	if err != nil {
		return nil, err
	}
	secrets, err := dagql.LoadIDs(ctx, s.srv, args.Secrets)
	if err != nil {
		return nil, err
	}
	return ctr.Build(
		ctx,
		parent,
		args.Dockerfile,
		collectInputsSlice(args.BuildArgs),
		args.Target,
		secrets,
	)
}
