package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd/labels"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/progrock"
)

type Host struct {
	Query *Query
}

func (*Host) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Host",
		NonNull:   true,
	}
}

type CopyFilter struct {
	Exclude []string `default:"[]"`
	Include []string `default:"[]"`
}

func LoadBlob(ctx context.Context, srv *dagql.Server, desc specs.Descriptor) (i dagql.Instance[*Directory], err error) {
	// Instead of directly returning a Directory, which would get "stamped" with
	// an impure ID that cannot be passed between modules, we fetch the Directory
	// we just uploaded by its blob, which yields a pure ID.
	err = srv.Select(ctx, srv.Root(), &i, dagql.Selector{
		Field: "blob",
		Args: []dagql.NamedInput{
			{
				Name:  "digest",
				Value: dagql.NewString(desc.Digest.String()),
			},
			{
				Name:  "size",
				Value: dagql.NewInt(desc.Size),
			},
			{
				Name:  "mediaType",
				Value: dagql.NewString(desc.MediaType),
			},
			{
				Name:  "uncompressed",
				Value: dagql.NewString(desc.Annotations[labels.LabelUncompressed]),
			},
		},
	})
	return
}

func (host *Host) Directory(
	ctx context.Context,
	srv *dagql.Server,
	dirPath string,
	pipelineNamePrefix string,
	filter CopyFilter,
) (dagql.Instance[*Directory], error) {
	var i dagql.Instance[*Directory]
	// TODO: enforcement that requester session is granted access to source session at this path

	// Create a sub-pipeline to group llb.Local instructions
	pipelineName := fmt.Sprintf("%s %s", pipelineNamePrefix, dirPath)
	ctx, subRecorder := progrock.WithGroup(ctx, pipelineName, progrock.Weak())

	_, desc, err := host.Query.Buildkit.LocalImport(
		ctx,
		subRecorder,
		host.Query.Platform.Spec(),
		dirPath,
		filter.Exclude,
		filter.Include,
	)
	if err != nil {
		return i, fmt.Errorf("host directory %s: %w", dirPath, err)
	}
	return LoadBlob(ctx, srv, desc)
}

func (host *Host) File(ctx context.Context, srv *dagql.Server, filePath string) (dagql.Instance[*File], error) {
	fileDir, fileName := filepath.Split(filePath)
	var i dagql.Instance[*File]
	if err := srv.Select(ctx, srv.Root(), &i, dagql.Selector{
		Field: "host",
	}, dagql.Selector{
		Field: "directory",
		Args: []dagql.NamedInput{
			{
				Name:  "path",
				Value: dagql.NewString(fileDir),
			},
			{
				Name:  "include",
				Value: dagql.ArrayInput[dagql.String]{dagql.NewString(fileName)},
			},
		},
	}, dagql.Selector{
		Field: "file",
		Args: []dagql.NamedInput{
			{
				Name:  "path",
				Value: dagql.NewString(fileName),
			},
		},
	}); err != nil {
		return i, err
	}
	return i, nil
}

func (host *Host) SetSecretFile(ctx context.Context, secretName string, path string) (*Secret, error) {
	secret := host.Query.NewSecret(secretName)

	secretFileContent, err := host.Query.Buildkit.ReadCallerHostFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}

	if err := host.Query.Secrets.AddSecret(ctx, secretName, secretFileContent); err != nil {
		return nil, err
	}

	return secret, nil
}

func (host *Host) Socket(sockPath string) *Socket {
	return NewHostUnixSocket(sockPath)
}
