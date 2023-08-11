package schema

import (
	"github.com/dagger/dagger/core"
)

type fileSchema struct {
	*MergedSchemas

	host *core.Host
}

var _ ExecutableSchema = &fileSchema{}

func (s *fileSchema) Name() string {
	return "file"
}

func (s *fileSchema) Schema() string {
	return File
}

var fileIDResolver = stringResolver(core.FileID(""))

func (s *fileSchema) Resolvers() Resolvers {
	return Resolvers{
		"FileID": fileIDResolver,
		"Query": ObjectResolver{
			"file": ToResolver(s.file),
		},
		"File": ToIDableObjectResolver(core.FileID.ToFile, ObjectResolver{
			"id":             ToResolver(s.id),
			"sync":           ToResolver(s.sync),
			"contents":       ToResolver(s.contents),
			"size":           ToResolver(s.size),
			"export":         ToResolver(s.export),
			"withTimestamps": ToResolver(s.withTimestamps),
			"withVersion":    ToResolver(s.withVersion),
			"labels":         ToResolver(s.labels),
			"withLabel":      ToResolver(s.withLabel),
			"sbom":           ToResolver(s.sbom),
		}),
	}
}

func (s *fileSchema) Dependencies() []ExecutableSchema {
	return nil
}

type fileArgs struct {
	ID core.FileID
}

func (s *fileSchema) file(ctx *core.Context, parent any, args fileArgs) (*core.File, error) {
	return args.ID.ToFile()
}

func (s *fileSchema) id(ctx *core.Context, parent *core.File, args any) (core.FileID, error) {
	return parent.ID()
}

func (s *fileSchema) sync(ctx *core.Context, parent *core.File, _ any) (core.FileID, error) {
	err := parent.Evaluate(ctx.Context, s.bk)
	if err != nil {
		return "", err
	}
	return parent.ID()
}

func (s *fileSchema) contents(ctx *core.Context, file *core.File, args any) (string, error) {
	content, err := file.Contents(ctx, s.bk)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (s *fileSchema) size(ctx *core.Context, file *core.File, args any) (int64, error) {
	info, err := file.Stat(ctx, s.bk)
	if err != nil {
		return 0, err
	}

	return info.Size_, nil
}

type fileExportArgs struct {
	Path               string
	AllowParentDirPath bool
}

func (s *fileSchema) export(ctx *core.Context, parent *core.File, args fileExportArgs) (bool, error) {
	err := parent.Export(ctx, s.bk, s.host, args.Path, args.AllowParentDirPath)
	if err != nil {
		return false, err
	}

	return true, nil
}

type fileWithTimestampsArgs struct {
	Timestamp int
}

func (s *fileSchema) withTimestamps(ctx *core.Context, parent *core.File, args fileWithTimestampsArgs) (*core.File, error) {
	return parent.WithTimestamps(ctx, args.Timestamp)
}

type fileWithVersionArgs struct {
	Version string
}

func (s *fileSchema) withVersion(ctx *core.Context, parent *core.File, args fileWithVersionArgs) (*core.File, error) {
	parent = parent.Clone()
	parent.Version = args.Version
	return parent, nil
}

func (s *fileSchema) labels(ctx *core.Context, parent *core.File, args any) ([]Label, error) {
	labels := make([]Label, 0, len(parent.Labels))
	for name, value := range parent.Labels {
		label := Label{
			Name:  name,
			Value: value,
		}

		labels = append(labels, label)
	}

	return labels, nil
}

type fileWithLabelArgs struct {
	Name  string
	Value string
}

func (s *fileSchema) withLabel(ctx *core.Context, parent *core.File, args fileWithLabelArgs) (*core.File, error) {
	parent = parent.Clone()
	if parent.Labels == nil {
		parent.Labels = make(map[string]string)
	}
	parent.Labels[args.Name] = args.Value
	return parent, nil
}

func (s *fileSchema) sbom(ctx *core.Context, parent *core.File, args any) (string, error) {
	// TODO:dummy implementation
	return "", nil
}
